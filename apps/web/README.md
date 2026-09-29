# DEEIX Chat Frontend

`@deeix/web` 是 DEEIX Chat 的浏览器端工作区，基于 Next.js App Router 实现对话、文件、知识库、提示词、用户设置和管理员后台。前端负责界面、客户端状态和展示层流程；认证授权、模型路由、文件处理、计费、持久化和审计等业务规则由 Go 后端负责。

前端采用 Next.js 静态导出模式（`output: "export"`）。开发时使用 Next.js 开发服务器，构建后生成 `apps/web/out`，生产环境可由 Go 服务托管，也可以交给 Nginx、CDN 或其他静态文件服务。

相关文档：[项目主 README](../../README.md) · [后端 README](../../backend/README.md) · [API 文档索引](../../backend/docs/README.md) · [多端架构](../../docs/ARCHITECTURE.md) · [Biome 规则与例外](./BIOME.md)

## 技术栈

- Next.js 16.3.4、React 19.2.8、TypeScript 7
- Tailwind CSS 4、Shadcn/UI、Radix UI、Base UI
- Biome 2（lint）
- Streamdown、KaTeX、Mermaid、Recharts、Motion
- `@deeix/api-contract`：从 Go Swagger 契约生成的 TypeScript 类型

## 目录结构

```text
apps/web/
├── app/                       # App Router 路由入口、页面和布局（只做挂载）
│   ├── globals.css            # 全局样式与设计 token
│   ├── (app)/                 # 主应用根布局（Web 与桌面内容标签页）
│   │   ├── (auth)/            # 登录和 OAuth 回调
│   │   ├── (project)/         # 需登录的工作区、设置和管理员后台
│   │   └── share/             # 公开分享页（无鉴权）
│   └── (shell)/               # 桌面端壳 UI 根布局
│       └── desktop/tabs/      # 桌面端标签栏
├── components/                # 无业务语义的基础和视觉组件
│   ├── ui/                    # 通用 UI primitives（shadcn）
│   ├── animate-ui/            # 第三方动画组件与图标
│   └── reactbits/             # 第三方视觉效果组件
├── entities/                  # 被多个 feature 复用的业务实体，每个实体只经 index.ts 对外
│   ├── announcement/          # 公告事件（打开公告、未读状态）
│   ├── billing/               # 计费展示与分时定价
│   ├── conversation/          # 会话：标签、分享/导出菜单、项目子菜单、搜索、默认模型、侧栏列表
│   ├── file/                  # 文件：预览、展示、处理状态与轮询、删除选项、文件库事件
│   ├── identity-provider/     # 身份源图标
│   ├── mcp/                   # MCP 工具选择规则
│   ├── model/                 # 模型：身份与图标、选择器、选项展示与参数策略、原生工具
│   ├── prompt-preset/         # 提示词预设约束
│   ├── skill/                 # 技能表单模型
│   └── user-settings/         # 用户设置存储与对话内容宽度
├── features/                  # 按业务域组织页面组件、hooks 和模型
│   ├── admin/                 # 管理后台
│   ├── announcements/         # 公告
│   ├── auth/                  # 登录和会话流程
│   ├── chat/                  # 对话工作区
│   ├── desktop/               # 桌面端启动、服务器设置、更新提示、标签栏
│   ├── files/                 # 文件管理与处理状态
│   ├── knowledge-bases/       # 知识库
│   ├── library/               # 指令库：技能与提示词
│   ├── recent/                # 最近会话
│   ├── settings/              # 用户设置
│   ├── share/                 # 分享页
│   └── shell/                 # 应用外壳：导航、Provider、工作区布局
├── shared/                    # 跨业务复用、无业务实体语义的基础能力
│   ├── api/                   # HTTP client、鉴权 client、按资源拆分的请求函数与传输类型
│   ├── auth/                  # 会话与访问令牌管理
│   ├── capabilities/          # 服务器能力声明（useFeature、FeatureGate、useSectionGuard）
│   ├── components/            # 跨业务组件
│   ├── config/                # 品牌与运行时配置
│   ├── generated/             # 资源同步生成文件（禁止手改）
│   ├── hooks/                 # 通用 hooks
│   ├── lib/                   # 通用工具
│   ├── model/                 # 全局纯模型
│   ├── platform/              # 桌面端（Tauri）原生能力绑定
│   └── pwa/                   # PWA 资源与迁移
├── i18n/                      # 国际化配置、en-US / zh-CN 文案、错误 key 解析
├── lib/                       # 根级通用工具（cn() 等）
├── styles/                    # 分主题样式
├── types/                     # 第三方模块的 .d.ts 声明
├── public/                    # 静态资源
├── scripts/                   # 资源同步脚本与架构检查脚本（check-architecture.mjs）
├── biome.jsonc                # Biome lint 配置（说明见 BIOME.md）
├── next.config.ts             # 静态导出与 Next 配置
└── package.json               # @deeix/web workspace 脚本
```

分层要点如下（可机械检查的部分由 `scripts/check-architecture.mjs` 强制）：

- `app/` 只做路由挂载、布局和边界处理；`page.tsx` 是 Server Component 薄壳，不写 `"use client"`。
- 业务代码放在 `features/<domain>`；被多个 feature 复用且带业务实体语义的内容放在 `entities/`；无业务语义的复用能力放在 `shared/`。
- 每个 feature 都有 `index.ts` 公共入口；一个 feature 只能通过另一个 feature 的 `index.ts`（`@/features/<x>`）引用它，不能深入其内部路径；副作用导入（`import "..."`）和相对路径（`../<other-feature>/...`）同样受检。
- `app/` 路由一律经 `@/features/<x>` 挂载入口组件，不深入 feature 内部路径（`pnpm check:arch` 强制）。`package.json` 声明了 `sideEffects`，未使用的 re-export 会被 tree-shake，每个路由只打包自己挂载的入口。
- entity 内部结构与 feature 相同（`components/`、`context/`、`events/`、`hooks/`、`lib/`、`model/`、`types/`，按需创建），hook 命名为 `use-<entity>-<purpose>.ts`。每个 entity 都有 `index.ts`，且只有这一个公共入口：entity 之外（`app/`、`features/`、其他 entity）一律经 `@/entities/<x>` 引用，不设 `components` / `lib` 等子路径入口；entity 内部文件按路径互相引用，不经自身 `index.ts`。entity 之间可以相互依赖，但不得成环。
- 懒加载的 entity 组件由 entity 自己导出已包装好的版本（如 `LazyFilePreviewDialog`）；不要写 `dynamic(() => import("@/entities/<x>"))`，那会把整个 entity 打进懒加载 chunk。
- `shared/` 不引用 `entities/` 或 `features/`；通用组件需要业务能力时由调用方通过 props 注入（如 `AboutSettingsContent` 的 `brandIcon`）。
- `package.json` 声明 `"sideEffects": ["*.css", "./instrumentation-client.ts"]`：只有样式表和列出的入口文件有导入副作用，打包器因此能裁剪 `index.ts` 中未被使用的再导出，单一入口不会让路由打包整个 entity / feature。新增有顶层副作用的模块时必须列入该字段；脚本导入不得写成无绑定的 `import "..."`。
- 文件名使用 kebab-case，除 `*.test.ts(x)`、`*.spec.ts(x)`、`*.config.ts` 外不带额外点号段。
- `components/ui` 不引用 `features/` 或 `entities/`。

### Feature 文件组织

业务域内部按职责拆分，子目录按需创建：

- `components/`：页面外壳、section、表格、弹窗、图表和编辑器等业务组件。`components/` 根目录只放 feature 的顶层入口（如 `app-files.tsx`、`admin-shell.tsx`）；其余组件放在 `components/sections/`，被多个 section 复用的放在 `components/shared/`（按职责命名，不加 feature 前缀，如 `sortable-list.tsx`）。多页面 feature（`admin`、`settings`）按 `components/sections/<section>/` 组织：入口为 `<domain>-<section>.tsx`（如 `admin-users.tsx`、`settings-account.tsx`），其余文件以 `<section>-` 为前缀（如 `tools-mcp-order-sheet.tsx`）。
- `context/`：业务域内部共享的上下文状态。
- `hooks/`：加载、筛选、提交、乐观更新、轮询和批量操作等状态编排；组件不直接调用 API 函数。文件名为 `use-<domain>-<purpose>.ts`，导出的 hook 以 `use<Domain>` 开头；`<domain>` 取 feature 目录名的单数（`files` → `file`、`knowledge-bases` → `knowledge-base`、`announcements` → `announcement`，`library` 为不可数的区域名、保持原样，`settings` 作为产品术语保留复数），`entities/<entity>` 取实体名。管理侧 hook 还需带上 section：`use-admin-<section>-<purpose>.ts`，导出 `useAdmin<Section>...`。映射表见 `scripts/check-architecture.mjs` 的 `FEATURE_DOMAINS`。
- `model/`：纯业务模型、常量、映射和排序规则，不放 React 副作用和展示文案。
- `types/`：业务域内部的 UI 状态和表单类型，不重复定义后端传输契约。
- `utils/`：业务域内部的格式化和展示工具。
- `events/`：仅本 feature 内部使用的事件。

请求函数默认放在 `shared/api/`，按资源拆成 `<resource>.ts` + `<resource>-types.ts`；只有管理侧专属端点放在 `features/admin/api/`。

拆分以表达业务边界为目标：简单页面可以保留为单文件，复杂页面再按可见 section 和清晰功能边界拆分。

## 路由

Next.js 的 route group（`(app)`、`(auth)`、`(project)`、`(shell)`）只用于组织代码，不会出现在 URL 中。当前页面入口如下：

| 路径 | 用途 |
| --- | --- |
| `/` | 重定向到 `/chat` |
| `/login` | 用户登录 |
| `/auth/callback` | OAuth/OIDC 回调 |
| `/chat` | 对话工作区 |
| `/recent` | 最近会话 |
| `/files` | 文件管理 |
| `/knowledge-bases` | 知识库管理 |
| `/library` | 指令库（技能与提示词） |
| `/share` | 公开分享内容 |
| `/preview/image-loading` | 开发用图片生成加载态预览 |
| `/settings/general` | 通用偏好 |
| `/settings/chat` | 对话偏好 |
| `/settings/subscription` | 订阅与用量 |
| `/settings/account` | 账户与身份源 |
| `/settings/about` | 产品信息 |
| `/admin` | 管理后台首页 |
| `/admin/about` | 版本信息和更新检查 |
| `/admin/announcements` | 公告管理 |
| `/admin/billing` | 计费与支付 |
| `/admin/content-moderation` | 内容审核 |
| `/admin/conversation` | 会话配置与参数策略 |
| `/admin/files` | 文件、提取、OCR、RAG 和存储配额 |
| `/admin/groups` | 权限组 |
| `/admin/knowledge-bases` | 平台知识库 |
| `/admin/login` | 管理员登录与登录策略 |
| `/admin/logs` | 日志与审计信息 |
| `/admin/models` | 模型、路由、能力和官方原生工具 |
| `/admin/statistics` | 统计信息 |
| `/admin/tools` | MCP 工具 |
| `/admin/upstreams` | 上游渠道 |
| `/admin/users` | 用户与账户 |
| `/desktop/tabs` | 桌面端壳 UI：标签栏 |

新增管理后台页面时，必须同时在 `features/admin/model/admin-sections.ts` 的 `ADMIN_SECTIONS` 中登记，且路由段、`ADMIN_SECTIONS` 的 `id` 与 `features/admin/components/sections/<section>/` 目录同名，否则 `pnpm check:arch` 会失败。

## API 契约

后端 HTTP DTO、JSON/校验标签和 Swagger annotation 是传输契约的唯一事实源。契约生成链路为：

```text
backend HTTP DTO / Swagger annotations
  -> backend/docs/{docs.go,swagger.json,swagger.yaml}
  -> packages/api-contract/src/types.generated.ts
  -> frontend shared/api and feature API adapters
```

生成文件由工具维护，禁止手工修改：

- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`
- `packages/api-contract/src/types.generated.ts`

变更路由、DTO、JSON 标签、校验标签、响应文档或 Swagger annotation 后，从仓库根目录执行：

```bash
pnpm api:generate
pnpm api:check
```

前端传输类型必须从 `@deeix/api-contract` 导入。表单草稿、未提交状态、视图模型和格式化结果属于前端模型，可以定义在对应 feature 中；不要复制生成字段，也不要用 `Required<>` 修补后端 requiredness。标准响应沿用生成契约中的 `errorMsg + data` envelope，错误解析统一读取 `errorMsg`。

对话消息的 `processTrace` 由后端产生，前端按职责展示为处理链路、思考链路和工具链路。模型能力 JSON 中的 `defaultOptions`、`optionControls` 和 `nativeToolKeys` 负责默认参数、设置控件和管理员允许的官方原生工具；用户输入最终仍由后端参数策略治理。

## 静态导出与配置

`next.config.ts` 的关键行为：

- `output: "export"`：构建结果写入 `apps/web/out`。
- `images.unoptimized: true`：保持静态导出，不依赖 Next.js 图片优化服务。
- `NEXT_PUBLIC_API_BASE_URL`：浏览器请求 API 的地址，会在构建时进入静态资源。

本地开发在 `apps/web/.env.local` 设置：

```env
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

分离部署时，必须在构建前设置正式 API 地址：

```bash
NEXT_PUBLIC_API_BASE_URL=https://api.example.com pnpm --filter @deeix/web build
```

`NEXT_PUBLIC_*` 变量会进入浏览器包，不放任何敏感信息。产品品牌和其他浏览器运行时配置由后端公开接口提供，因此修改品牌通常只需要更新后端配置并重启服务。

构建后的 `apps/web/out` 可以交给静态服务器或 Go 后端托管。静态服务器需要把无扩展名页面映射到对应的 `index.html`；使用 Go 服务时由 `server.frontend_dist_dir` 指向该目录。由于这是静态导出应用，不使用 `next start` 作为生产启动方式。

## 本地开发

以下命令从仓库根目录执行：

```bash
pnpm install
cp apps/web/.env.example apps/web/.env.local
pnpm dev:web
```

如果需要同时启动 Go API：

```bash
cp deploy/config.example.yaml config.yaml
pnpm dev
```

如果本机没有 PostgreSQL 和 Redis，可以启动完整本地依赖：

```bash
cd deploy && docker compose -f docker-compose.full.yml up -d
```

只启动前端时访问 `http://localhost:3000`。后端默认监听 `http://localhost:8080`。API 地址按“运行时覆盖（桌面端）→ `NEXT_PUBLIC_API_BASE_URL` → 页面 origin”的顺序解析；未设置构建变量且页面运行在本机回环地址的非 8080 端口时，自动指向同主机的 8080 端口。

从 `apps/web/` 目录工作时，等价命令是：

```bash
pnpm dev
pnpm check        # lint + typecheck + check:arch
pnpm lint
pnpm typecheck
pnpm check:arch
pnpm build
```

`pnpm install` 会同步资源；`predev` 和 `prebuild` 会检查版本并同步资源。需要手动同步时，可以使用：

```bash
pnpm --filter @deeix/web sync:assets
pnpm --filter @deeix/web sync:icons
pnpm --filter @deeix/web sync:pwa-assets
pnpm --filter @deeix/web sync:screenshot-worker
```

## 开发约束

- 路由文件保持薄，业务逻辑放在 `features/*` 或 `entities/*` 中。
- API 访问统一通过 `shared/api`（管理侧专属端点为 `features/admin/api`）完成；需要鉴权的请求一律走 `authedRequest` / `authedFetch`。
- 组件不直接调用请求函数：请求、加载/提交状态和 toast 放在 hook 中（`pnpm check:arch` 强制）。
- `unknown` 数据（API 响应、存储、事件、JSON）用 `shared/lib/type-guards.ts` 的守卫或 `instanceof` 收窄，字符串联合用 `as const` 数组 + `isOneOf`；不用 `as` 断言，库类型缺口确需断言时写 `// Type assertion: <原因>`（`pnpm check:arch` 强制，`components/animate-ui`、`components/reactbits` 豁免）。
- 功能显隐读取服务器能力声明（`useFeature` / `FeatureGate`）或管理员开关（`useFeaturePolicy`），不按 `isDesktopApp()` 判断；Tauri API 只在 `shared/platform/` 调用。
- 保持静态导出能力，不引入依赖常驻 Next.js Server、Server Action 或服务端 API Route 的实现。
- 除 API 定位等构建期常量外，不新增必须重新构建才能修改的品牌环境变量。
- 不在 React render 阶段读取 `window`、`document`、`getComputedStyle` 或本地存储；使用现有 store、effect 或明确的客户端边界，保持静态导出和 hydration 一致。
- Refresh token 在 Web 端只由后端写入 HttpOnly Cookie，桌面端存于系统 keychain；access token 只保存在前端内存中。
- 用户可见文案（含 `aria-label`）走 i18n；代码注释使用英文。
- 不在前端硬编码上游模型私有规则；模型参数以模型能力 JSON、用户配置和后端策略为准。
- 文件、MCP 工具、官方原生工具和消息轨迹只消费后端结构化状态，不在前端复制业务状态机。
- AI 生成 HTML 只能使用项目允许的安全标签、内联样式属性和 `shared/lib/html-visual-theme.ts` 白名单变量；主题变量变更时同步后端 prompt 并运行相关检查。
- 图标优先使用 `lucide-react`，新增复杂 UI 时复用现有 Dialog、Sheet、Table、Form、Tabs 和 Switch 组件风格。

## 提交前验证

从仓库根目录执行：

```bash
pnpm --filter @deeix/web check
pnpm api:check
```

涉及路由、依赖、静态导出或 Next.js 配置时，再执行：

```bash
pnpm --filter @deeix/web build
```

根目录的 `pnpm check`、`pnpm test`、`pnpm build` 和 `pnpm verify` 会通过 Turborepo 运行对应工作区任务。Biome 规则与例外见 [BIOME.md](./BIOME.md)。
