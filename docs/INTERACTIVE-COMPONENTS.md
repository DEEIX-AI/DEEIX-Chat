# 交互式组件设计

> 状态：P0、P1 与 P2 的沙箱渲染已实现；`message`/`tool` action 与 MCP 来源为设计稿。本文定义 DEEIX Chat 交互式组件（UI Components）的协议、架构边界和分期计划。

## 目标

让模型在对话中输出可交互的结构化界面（可筛选的卡片网格、可切换序列的图表、可提交的表单、可添加曲线的函数图等），而不只是静态 Markdown；同时允许管理员、用户和 MCP 服务提供自定义组件。

设计遵循四条原则：

1. **结构化优先，自由 HTML 兜底。** 内置组件由 JSON Schema 驱动，模型输出的是数据而不是 DOM。现有 Artifact（`html` 代码块 → 沙箱 iframe）继续承担"没有合适组件"时的自由渲染。
2. **模型只见标记名与入参。** 渲染实现永远不进入上下文；组件实现更新不需要修改提示词、协议或历史消息。
3. **一个协议，三层来源。** 内置组件、自定义组件、MCP 提供的组件对模型、对前端渲染注册表、对组件管理页面是同一套接口。
4. **交互是事件，不是猜测。** 用户在组件上的操作以结构化事件回到对话或调用工具，模型收到的是 `{component, action, payload}` 而不是自然语言转述。

## 现状

仓库已有三层相关能力，本设计在其上补"结构化"与"双向"两层，不重建：

| 现有能力 | 位置 | 保留用途 |
| --- | --- | --- |
| 内联 HTML（`html-visual` 提示词层 + Streamdown 标签白名单） | `backend/internal/application/conversation/system_prompt.go`、`frontend/shared/components/markdown/` | 正文里的静态排版（对比矩阵、信息卡） |
| Artifact（`html`/`svg` 代码块 → 沙箱 iframe） | `frontend/features/chat/model/chat-artifacts.ts`、`chat-artifact.tsx` | 自由 HTML 应用；其沙箱与 CSP 直接复用为自定义组件宿主 |
| Skill（用户/平台级 Markdown 提示词） | `backend/internal/application/skill/` | 教模型何时使用哪个组件；不承载渲染 |
| MCP 工具 | `backend/internal/application/mcp/` | `tool` action 的执行与授权；MCP 组件的来源 |

组件**不是** Skill 的子字段。Skill 是行为，组件是渲染能力；一个组件可被多个 Skill 引用，也可在没有 Skill 时由模型直接从目录中选用。

## 架构

```text
                        ┌────────────────────────────────────────┐
  模型侧（只见目录）      │   Component Catalog（后端事实源）         │
  ┌──────────────┐      │  ┌──────────┐ ┌──────────┐ ┌─────────┐ │
  │ <ui-components>│◀────│  │ builtin  │ │ custom   │ │  mcp    │ │
  │  name: desc    │     │  │ 仓库内置  │ │ 用户/管理员│ │ MCP 服务 │ │
  │  props={...}   │     │  │ React    │ │ 沙箱 HTML │ │ 沙箱 HTML│ │
  └──────────────┘      │  └──────────┘ └──────────┘ └─────────┘ │
         │ 输出          └────────────────────────────────────────┘
         ▼                                  │ 目录接口 + 渲染源
  ```deeix-ui                               ▼
  {component, id,          ┌──────────────────────────────────┐
   props, state}           │  前端 Renderer Registry            │
                           │  name → builtin React | SandboxHost│
                           └──────────────────────────────────┘
                                            │ action 事件
                                            ▼
                           local(改 state) / message(回对话) / tool(调 MCP)
```

后端目录是组件能力的事实源，与模型能力 JSON 的思路一致。前端不硬编码组件业务规则，未知组件降级渲染。

## 协议

### 模型输出

模型使用 Markdown 围栏输出组件，语言标识固定为 `deeix-ui`，内容为 JSON：

````markdown
今天的日报如下：

```deeix-ui
{
  "component": "card-grid",
  "id": "news-0919",
  "props": {
    "title": "9月19日 · 今日日报",
    "items": [
      { "tag": "AI / 科技", "source": "Reuters", "title": "…", "summary": "…", "url": "…" }
    ]
  }
}
```
````

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `component` | 是 | 组件名，对应目录中的 `name`。可带命名空间前缀 `publisher/name`（如 `acme/stock-glance`）；内置组件省略前缀 |
| `version` | 否 | 组件 schema 版本，缺省为 1。由渲染器按 `name@version` 分发；模型不输出，只有目录为某组件声明了破坏性新版本（如"输出 version: 2"）时才写 |
| `id` | 是 | 组件实例 ID，在一次对话内稳定。后续回合出现同 `id` 的块视为对该实例的 JSON Merge Patch，卡片原地更新 |
| `props` | 是 | 入参，必须符合组件 schema |
| `state` | 否 | 初始本地状态；缺省由组件定义决定 |

`actions` **不在消息里**。它属于组件定义，模型不写、不需要知道。这把模型的出错面压缩到"`props` 是否符合 schema"。

选择围栏而非工具调用的原因：任何模型都能输出围栏，不依赖 function calling，本地小模型同样可用；流式输出天然内联；消息正文仍是 Markdown，落库、历史回放、导出、分享零改动；`chat-artifacts.ts` 的围栏解析可直接复用。

### 提示词目录段

后端从已启用组件的 schema 自动生成目录段，每个组件一行，作为现有 `<format>` 分层机制中的一层注入：

```text
<ui-components>
  card-grid: 卡片网格，条目带 tag 时自动可筛选。props={title, items[{tag,source,title,summary,url}]}
  data-table: 数据表格，可排序/搜索/导出。props={columns[{key,label,type}], rows[]}
  chart: 图表。props={type:line|bar|pie, x[], series[{name,data[]}]}
</ui-components>
```

目录按对话的启用集裁剪。schema 的 `description` 与属性名进入目录，渲染源永不进入。

### 组件定义

```jsonc
{
  "manifest": 1,                           // manifest 格式版本，独立于组件 version
  "publisher": "acme",                      // 命名空间；内置组件为 deeix
  "name": "stock-glance",
  "version": 1,
  "source": "custom",                      // builtin | custom | mcp
  "scope": "user",                         // platform | user
  "description": "股票速览",                // → 提示词目录
  "schema": {                              // → 目录压缩 + 前端校验 + 渲染器契约
    "type": "object",
    "required": ["symbol"],
    "properties": {
      "symbol": { "type": "string", "description": "股票代码" },
      "name":   { "type": "string" }
    }
  },
  "actions": {                             // → 宿主白名单；目录只列名字
    "watch": { "kind": "message", "template": "把 {{props.symbol}} 加入关注" },
    "quote": { "kind": "tool", "tool": "market_quote", "into": "props.quote" }
  },
  "renderer": {                            // 永不进入提示词
    "kind": "sandbox",                     // builtin | sandbox
    "uri": "…"                             // sandbox 时的渲染源地址；导入包内可用 source 内联
  },
  "examples": [ { "props": { "symbol": "AAPL" } } ],   // 组件中心预览与 few-shot
  "requires": { "tools": ["market_quote"] }             // 依赖的 MCP 工具，导入时校验
}
```

`schema` 是唯一耦合点：提示词目录、前端校验、渲染器输入契约都从它派生，不存在三方漂移。

### Action

| `kind` | 语义 | 去向 |
| --- | --- | --- |
| `local` | 修改组件实例的 `state`（筛选、翻页、勾选、切换） | 当前：仅组件内存状态，刷新即回到 props 初始值。设计：随消息持久化；下一轮以一行摘要附给模型（如"用户当前筛选：AI / 科技"） |
| `message` | 按 `template` 渲染为一条用户消息发送 | 携带 `interaction { componentID, action, payload, nonce }` 元数据；模型收到结构化上下文 |
| `tool` | 调用声明的 MCP 工具 | 结果写回 `into` 指定的 props 路径，组件自更新，不占用对话回合 |

`template` 使用 `{{props.*}}` / `{{state.*}}` / `{{payload.*}}` 占位。

`tool` 不另建权限模型：`mcp` 来源的组件只能调用其所属 MCP 服务的工具；`custom` 来源的组件只能调用用户当前对话已勾选的 MCP 工具。授权、审计与计费复用现有 MCP 链路。

## 三层来源

| 来源 | 提供方 | 渲染域 | 进入目录的方式 |
| --- | --- | --- | --- |
| `builtin` | 仓库 | 主文档 React（直接使用 shadcn、主题变量、Recharts、KaTeX） | 代码内置，schema 与组件同目录 |
| `custom` | 管理员（`platform`）/ 用户（`user`） | 沙箱 iframe | 组件管理页新建：名称、schema、渲染代码、实时预览 |
| `mcp` | MCP 服务器 | 沙箱 iframe | MCP 服务声明 `ui://` 资源，连接时自动发现，随工具一起启用 |

`mcp` 来源使工具提供方可以随工具交付界面，用户勾选 MCP 服务即获得组件。该方向与 MCP Apps 生态正在收敛的"工具 + `ui://` HTML 资源"模式一致，DEEIX 已具备 MCP 基础设施，接入成本最低。

三层对前端渲染注册表是同一个接口 `name → renderer`，对模型是同一行目录，对用户是同一个组件管理页（`builtin` 只能开关，`custom` 可编辑，`mcp` 显示来源）。

### 内置组件

准入标准只有一条：组件必须做 Markdown 做不到的事（筛选、排序、计算、分支、对比、判分）。能用列表、表格或标题表达的内容不做组件。

内置目录是唯一事实来源：`backend/internal/domain/uicomponent/types.go` 用 `builtinSpec`（定位 / 适用 / 交互 / 约定 / 入参）四段式描述每个组件，`Description` 由「定位。适用：…。交互：…」生成（≤ 256 字，也是选择器展示的文案），入参约定跟随 `PropsSummary` 只给模型看。启动时 `SeedUIComponents` 把描述、入参、版本、排序同步进库，只保留管理员的启停状态；管理后台对内置组件也只开放启停。前端选择器、提示词目录、本文档看到的都是同一份文本。

| 组件 | 场景 | Markdown 做不到的点 |
| --- | --- | --- |
| `card-grid` | 新闻、搜索结果、候选方案 | 按 tag 筛选 |
| `stat-grid` | 并列 KPI、涨跌幅 | `history` 迷你走势线，悬停逐点读数（`periods` 为标签） |
| `chart` | line / bar / area / horizontal-bar / scatter / pie / radar / funnel / radial；series 级 `type` 做柱+线组合、`axis: right` 双 Y 轴；`stacked` + `percent` 100% 堆叠 | 序列显隐；所有形态共用 `x[] + series[{name, data[]}]` 一种数据结构 |
| `function-plot` | 函数图像与方程曲线 | 可编辑增删表达式；显式函数逐像素采样并在渐近线处断开，隐式方程用 marching squares 描零集；滚轮缩放与拖动平移 |
| `data-table` | 十行以上结构化数据 | 排序、搜索、分页、导出 CSV |
| `gantt` | 项目排期、里程碑、任务依赖 | 日/周/月刻度切换并适配宽度，点击任务高亮整条依赖链，分组折叠，今天线、周末带、进度填充、依赖箭头绕行 |
| `diff` | 修改前后、版本差异 | 行级 + 词级高亮，合并 / 并排切换，逐处跳转，折叠未变化行，复制修改后内容 |
| `calculator` | 贷款、换算、预算等 what-if | 拖动输入，公式实时重算（安全表达式求值，无 eval） |
| `decision-tree` | 排障、选型、资格判断 | 逐步点选，面包屑回退 |
| `quiz` | 学习检验、分支问答 / 向导 | 即时判对错、解析、计分；选项或题目带 `next` 时按 id 跳题、逐题呈现、可回退 |

视觉由 `ui-blocks/ui-block-frame.tsx` 统一：`rounded-xl border bg-card` 卡片容器，内部条目为 `border bg-background` 的白底卡，标题 `text-base font-semibold`；筛选选中与高亮输出用 `--primary` 低透明度，涨跌、diff 增删、答题对错用 green / red。

`calculator` 的公式与 `function-plot` 的函数都由 `shared/lib/safe-expression.ts` 解析为 AST 后求值：只允许 `+ - * / % ^`、括号、输入变量与固定函数白名单，不经过 `eval` 或 `Function`。`diff` 使用 `shared/lib/line-diff.ts` 的 LCS 行对比（上限 2000 行），改动行对之间再做一次词级 LCS 标出具体改动。

每个内置组件是 `ui-blocks/<name>.tsx` 中的一个 `UIBlockDefinition`（schema、React 组件、骨架屏）；`internal/domain/uicomponent.Builtin()` 持有同名条目的提示词描述与入参摘要，`types_test.go` 固定两侧的 `name@version` 配对。

## 沙箱宿主

自定义与 MCP 组件复用 Artifact 的 iframe 宿主：

- `sandbox="allow-scripts"`，CSP 沿用 `chat-artifacts.ts` 的 `ARTIFACT_CSP`，包括 `connect-src 'none'`。**组件不能自行发起网络请求**；需要数据时声明 `tool` action，由后端代取。这是安全边界，也是审计边界。
- 主题变量注入沿用 `htmlPreviewDocument`，组件自动跟随亮暗与配色。
- 宿主通过 `postMessage` 通信，校验 `event.source === iframe.contentWindow`，并按组件定义的 `actions` 白名单过滤事件。
- 渲染源大小上限、加载超时白屏兜底。

沙箱内注入 `window.deeix`：

```js
deeix.onProps(props => render(props));   // 初始 props 与后续 patch
deeix.onTheme(vars => applyTheme(vars)); // 主题切换
deeix.emit("watch", { symbol });         // 触发已声明的 action；未声明的被宿主拒绝
deeix.setState({ tab: "news" });         // 本地状态，随消息持久化
```

## 稳定性

| 情形 | 处理 |
| --- | --- |
| `props` 不符合 schema | 渲染"结构化内容不可用"折叠卡并展示原始 JSON；列表型组件逐条降级，只丢弃不合法的项 |
| 围栏未闭合（流式中） | 按组件类型显示骨架屏，闭合后一次渲染，避免 JSON 半截时抖动 |
| 未知组件或不支持的 `version` | 通用 JSON 卡，旧客户端不崩 |
| 同 `id` patch 目标不存在 | 视为新实例 |
| `message` action 连点 | `interaction.nonce` 后端去重 |
| 渲染源加载失败 | 白屏兜底，显示组件名与来源 |

`version` 从第一天存在，schema 演进时前端按版本选择渲染器，旧消息永远能按旧渲染器回放。

## 后端落点

按仓库分层：

| 层 | 内容 |
| --- | --- |
| `internal/domain/uicomponent` | `Component`、`Action`、`Renderer` 类型与常量 |
| `internal/ports/uicomponent` | 渲染源读取、MCP `ui://` 资源发现的契约 |
| `internal/application/uicomponent` | 目录服务（合并三层来源、按启用集裁剪）、提示词目录段生成、schema 校验 |
| `internal/repository` | `UIComponentRepository`（消费方声明，只含用到的方法） |
| `internal/infra/persistence` | `ui_components` 表；按域分组，不复用 Skill 或 MCP 表。含 `publisher`（命名空间）与 `origin`（导入 URL 或 MCP 服务 ID，供溯源与检查更新） |
| `internal/infra/mcp` | 连接时发现 `ui://` 资源 |
| `internal/transport/http` | `GET /api/v1/ui-components`（当前用户可用目录）、管理端 CRUD、用户私有组件 CRUD |

消息 `interaction` 元数据进入现有消息 JSON 列，不新增表。所有路由与 DTO 变更走 `pnpm api:generate`。

## 前端落点

| 位置 | 内容 |
| --- | --- |
| `shared/components/markdown/ui-blocks/` | 渲染注册表（`block.ts`）、结构校验器（`schema.ts`）、宿主与降级（`ui-block-host.tsx`）、内置组件（`card-grid.tsx`、`data-table.tsx`、`chart.tsx` 等）。放在 markdown 层而非 `features/chat`，因为分享页与知识库预览同样渲染消息正文 |
| `shared/components/markdown/ui-blocks/sandbox-host.tsx` | 沙箱宿主与 `postMessage` 桥，复用 Artifact 的文档构建（P2） |
| `shared/api/ui-components.ts` | 目录与渲染源请求，类型从 `@deeix/api-contract` 派生 |
| `features/ui-components/` | 组件管理页：列表、编辑器、实时预览 |

Streamdown 接入点在 `MarkdownCodePre`：语言为 `deeix-ui` 的代码块交给 `UIBlockHost`，其余代码块行为不变。流式判断复用 `MarkdownTableStreamingContext`；Streamdown 在流式期间会自动闭合未完成的围栏，因此半截 JSON 显示骨架而不是报错。

P0 的校验器是仓库内的最小结构校验（对象/数组/基础类型/枚举），不引入 JSON Schema 依赖；自定义组件（P2）需要通用 JSON Schema 时再评估。

## 扩展与生态

组件定义是自描述的 JSON（manifest），可以独立于数据库存在。这使三种接入形态都不需要额外协议：

| 形态 | 内容 | 依赖的设计点 | 分期 |
| --- | --- | --- | --- |
| 组件中心 | 站内页面：浏览、搜索、启用、编辑，用 `examples` 预览 | 目录接口 + 组件管理页 | P1 / P2 |
| 导入与市场 | 导入一个 manifest 文件或 URL；可选的远程索引提供官方/社区组件列表 | `manifest` 字段、`publisher/name` 全局标识、`requires` 校验、`origin` 溯源 | P4 |
| 第三方托管扩展 | 第三方运行自己的服务，DEEIX 连接后自动获得工具与组件 | `mcp` 来源 | P3 |

第三形态不自造协议。MCP 已是工具的扩展协议，MCP Apps 正在把 `ui://` 资源加进去；为其它客户端编写的 MCP App 应能以最小改动在 DEEIX 中运行。

### 导入校验

导入时后端按顺序校验：`manifest` 版本受支持；`publisher/name` 在目标 scope 内无冲突；`schema` 是合法 JSON Schema；`actions` 中 `tool` 引用的工具在 `requires.tools` 声明且在当前部署可用；渲染源大小在上限内。任一项失败整体拒绝，不做部分导入。

### 组件随内容走

对话导出与分享可选携带所用 `custom` 组件的 manifest。接收方导入后历史消息按原样渲染，而不是降级为 JSON 卡。`builtin` 与 `mcp` 组件不随内容携带，前者接收方已有，后者需接收方自行连接对应服务。

## 分期

每期可独立合并。

| 期 | 交付 | 涉及 |
| --- | --- | --- |
| P0 | `deeix-ui` 解析、渲染注册表、`card-grid` / `data-table` / `chart` 等 10 个内置组件（交互状态为组件内存态）、schema 降级；后端 `domain/uicomponent` 内置目录生成 `<ui-components>` 提示词层，P1 起客户端以 `uiComponentIDs` 上行勾选集（设备级偏好，跨会话记住） | 全栈（已实现） |
| P1 | 后端 `ui_components` 实体（内置行启动播种、受保护）、`/ui-components` 与 `/admin/ui-components` CRUD、会话按 `uiComponentIDs` 勾选（默认勾选已启用内置组件）、管理端与用户端组件库（与技能/提示词同页 tab） | 全栈（已实现） |
| P2 | 沙箱宿主、`window.deeix` SDK（`onProps`/`onTheme`）、`custom` 组件编辑器与实时预览 | 全栈（已实现；`emit`/`setState` 随 action 到来） |
| P2b | `message` action 与 `interaction` 元数据、`local` 状态持久化 | 全栈 |
| P3 | MCP `ui://` 资源发现、`tool` action、同 `id` patch | 全栈 |
| P4 | manifest 导入/导出、远程索引、分享携带组件 | 全栈 |

## 不做的事

- 不让模型生成完整 HTML 应用作为主路径。自由 HTML 通过现有 Artifact 承担，是兜底而非默认。
- 不把组件挂在 Skill 上。渲染代码进入提示词既浪费上下文，也让模型学到输出 DOM 的坏习惯。
- 不为沙箱组件开放网络访问。数据一律经 `tool` action 由后端代取。
- 不为 `tool` action 新建权限模型。复用 MCP 的授权、审计与计费。

## 当前边界

- `local` 状态只在内存中，刷新后回到初始态；随消息持久化与回流模型属于 P2b。
- 内置组件的提示词文案由后端 `internal/domain/uicomponent.Builtin()` 播种到 `ui_components`，渲染与校验由前端 `ui-blocks/registry.tsx` 按 `name@version` 分发；两侧必须成对更新。
- 管理端「对话配置 → 交互式组件」（`chat.ui_components_enabled`）是全局总闸：关闭时不注入目录，用户侧 `GET /ui-components` 返回空、会话内的 `uiComponentIDs` 解析为空，前端据此隐藏选择器并把已有组件块降级为原始内容；公开分享页通过响应里的 `uiComponentsEnabled` 得到同一开关（分享页只渲染内置组件，自定义组件在公开页降级）；管理端组件库不受影响。勾选集为空时同样不注入。
- 沙箱组件的 `propsSchema` 只支持 JSON Schema 的 object/array/string/number/integer/boolean/enum 子集，其余结构降级为不校验。
- 自定义组件的可见目录接口返回 `rendererSource`，一个用户最多加载 100 个组件；更大规模需要分页与按需拉取渲染源。

## 待决策

- 围栏语言名已采用 `deeix-ui`。
- P3 中 `tool` action 对 `custom` 组件的授权粒度：按对话已勾选的 MCP 工具，或在组件定义中额外声明白名单。
- 同 `id` patch 是否允许跨消息删除实例（`"props": null`）。
