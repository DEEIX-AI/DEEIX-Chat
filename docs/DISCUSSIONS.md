# 多模型讨论（多轮讨论）

本文档描述「多轮讨论」模板的定义方式、后端编排、流式事件契约与前端渲染规则。代码位置：

| 层 | 位置 |
|---|---|
| 模板领域模型 | `backend/internal/domain/discussion/` |
| 模板 CRUD | `backend/internal/transport/http/discussion/`、`backend/internal/application/discussion/` |
| 讨论编排 | `backend/internal/application/conversation/service_discussion.go` |
| 会话级快照 | `backend/internal/application/conversation/service_discussion_snapshot.go`、`backend/internal/infra/persistence/postgres/conversation/repository_discussion_snapshot.go` |
| 快照与流事件传输 | `backend/internal/transport/http/conversation/handler_discussion.go`、`handler_discussion_snapshot.go` |
| 前端事件折叠 | `apps/web/features/chat/model/discussion-stream.ts` |
| 前端面板 | `apps/web/features/chat/components/message/message-discussion.tsx` |
| 后台编辑器 | `apps/web/features/admin/components/sections/discussions/` |
| 对话时绑定条 | `apps/web/features/chat/components/sections/chat-discussion-config.tsx`、`apps/web/features/chat/hooks/use-chat-discussion.ts` |

## 1. 概念

- **模板**在后台定义结构，不绑定真实模型；**占位符**（placeholder）在对话时绑定真实模型。
- **轮次**（round）串行执行；每轮的**轮内步骤**（step）可配置为并行或串行，默认并行。串行按模型 1、2、3…N 的占位符顺序执行。
- **总结模型**是模板的最后一个节点，不属于任何轮次。
- 占位符分两类：`summary.participant` 指向总结占位符，其余都是讨论模型。展示名由位置决定（总结固定为「总结模型」，其余按顺序为「模型 1 … 模型 n」），不读存储的 `label`。

## 2. 模板结构

```json
{
  "placeholders": [
    { "key": "judge", "label": "总结模型", "defaultModel": "", "required": true },
    { "key": "p1", "label": "模型 1", "defaultModel": "gpt-6-astra", "required": true }
  ],
  "rounds": [
    { "index": 1, "parallel": true,
      "parallelPrompt": "请独立回答用户问题。",
      "serialPrompt": "请先独立回答，再回应本轮前序模型的分歧。",
      "steps": [
        { "id": "s_ab12", "participant": "p1", "prompt": "请独立回答用户问题。" }
      ]}
  ],
  "summary": { "participant": "judge", "prompt": "请综合成功发言，给出最终答案。" }
}
```

- 一轮只有一个 prompt：`round.steps[*].prompt` 始终同步为当前执行方式的 prompt，编辑器只让用户填一次。
- `round.parallelPrompt` / `round.serialPrompt` 分别保存两种执行方式的 prompt，互不覆盖；运行时优先使用当前执行方式的那一份。
- `round.parallel` 决定执行方式，缺省为 `true`。切换执行方式时，若目标方式从未编辑过，编辑器自动填入该方式的默认 prompt；也可用「恢复默认」按钮重新填入。
- 展示与串行执行都按 `placeholders` 的模型编号排序，不依赖 `steps` 的存储顺序（见 §5）。
- 新增讨论模型会追加到每一轮；删除会从每一轮移除。
- 模板存储于 `chat_discussion_templates`，接口前缀 `/api/v1/admin/discussion-templates`。

对话侧：模型选择器把启用中的模板暴露为 `discussion:<publicID>`，`SendMessage` 请求体携带 `templatePublicID` 与 `models`（占位符 key → 真实模型名）。

## 3. 后端编排

`Service.StreamDiscussionMessage` 的顺序：

1. 优先读取会话级快照：有 pending 使用 pending，否则使用 active；只有首次使用才通过 `ResolvePlan` 解析实时模板。active 忽略请求中的新绑定，pending 允许修改绑定并校验版本。
2. `FilterDisabledModels` 按占位符过滤本次执行，空轮次整体跳过但保留原编号；快照仍保存过滤前的完整计划。
3. 复用普通消息的建对逻辑，写入 user / assistant 两条消息，会话模型记为 `discussion:<publicID>`。
4. 沿当前分支取本次提问之前的对话历史（`ListMessageAncestors`），以**真实 user/assistant 消息**的形式放进 prompt，与普通对话的多轮结构一致。之前的讨论只以「你的提问 + 最终总结」出现一次，各模型的逐轮发言（`discussion_json`）不进入历史。
5. 逐轮执行：并行轮次共享同一份此前轮次 transcript，整轮完成后进入下一轮；串行轮次按模型顺序逐个执行，每次成功后立即把发言加入下一个模型的 transcript。
6. 每轮的 messages = `system（该轮 prompt + 执行方式说明）` + 历史消息 + 当前 user（本次问题 + 当前模型可见的讨论记录）。该轮 prompt 取当前执行方式对应的 `parallelPrompt` / `serialPrompt`；当前执行方式未单独配置时回落到 `steps[*].prompt`。仍然为空则使用内置默认值：并行首轮「独立、具体回答」，并行后续轮「阅读已有讨论并补充纠正」，串行首轮「先独立回答，再回应本轮前序发言的分歧」，串行后续轮「阅读此前轮次及本轮前序发言并回应分歧」。串行上下文标题明确包含「本轮在你之前」，并行明确不包含同轮其他模型。
7. 讨论结束后调用总结模型（同样带历史）；总结失败时用讨论记录拼出回退正文，而不是把整条消息判为失败。
8. 历史按 `ContextMaxTurns` 限制条数，并在 `generateDiscussion` 里用 `fitGenerateInputToModelBudget` 按**当前模型**的上下文预算裁掉最老的轮次（保留 system 与本次问题），与普通对话同一套规则。
9. 结束时把完整状态写入 `chat_messages.discussion_json`，并把汇总用量写入 assistant 消息。只有所有轮次和总结均为 `completed` 才自动提交 pending 或写入首次 active；`completed_with_errors` / `stopped` / 提前错误均不提交。

每次调用实际发送的提示词会被记录下来（`turn.prompt` / `state.summaryPrompt`），供前端调试模式展示；记录的是 `buildTextTaskGenerateInput` 归一化之后真正发往上游的消息。

讨论记录（transcript）格式：

```
第 1 轮
- gpt-6-astra: <内容>
- gemini-4-argon: <内容>
```

- 只有 `status=completed` 且有内容的发言进入记录；失败发言不是证据。
- 单条发言截断 12000 字符，整份记录截断 60000 字符。
- 发言人用模型名，占位符 key 不会泄漏进提示词。

## 4. 流式事件契约

接口 `POST /api/v1/conversations/{id}/messages/discussion/stream` 返回 NDJSON。除通用事件外，讨论专用事件：

| `type` | 负载 | 前端处理 |
|---|---|---|
| `discussion_start` | `template`, `templateName`, `placeholders`, `rounds`, `summary` | 建立实时状态；同一模板的重复 start 保留已收集的发言 |
| `discussion_turn_start` | `round`, `stepId`, `participant`, `model`, `prompt`, `promptMessages` | 模型调用前建立运行中发言并写入实际 Prompt 与结构化消息，首 token 前显示「思考中」且可查看 Prompt |
| `discussion_turn_delta` | `round`, `stepId`, `participant`, `model`, `delta` | 按 `(round, stepId)` 追加正文 |
| `discussion_turn_done` | `turn`（含 `prompt`、`promptMessages`） | 用最终发言替换（含用量、耗时、错误、发送的完整提示词） |
| `discussion_round_done` | `round`, `turns` | 用该轮结果整体替换该轮发言 |
| `discussion_summary_start` | `participant`, `model`, `prompt`, `promptMessages` | 总结调用前切换为「总结中」并写入实际 Prompt 与结构化消息 |
| `discussion_summary_delta` | `delta` | 追加到 assistant 正文，同时保持总结状态，实现总结流式 |
| `discussion_done` | `state`、`snapshotCommitted`、`activeSnapshot`、`activeVersion`、`pendingVersion` | 换成服务端最终状态，并直接同步已提交的快照，无需额外查询 |
| `delta`（收尾） | `delta`, `replace: true` | 用最终正文覆盖流式预览，避免重复 |

讨论状态与快照负载使用同一套小写驼峰契约，由 `service_discussion_payload.go` 的 `discussion*Payload` 产出。领域结构保持协议无关，不携带 JSON 标签，也不要直接塞进事件或 HTTP 响应。

所有事件同时写入 generation stream 缓存，因此切走会话再回来时，`resumeMessageGenerationStream` 能按 `afterSeq` 回放（见 `apps/web/features/chat/hooks/use-chat-data.ts`）。

## 5. 前端渲染

一条讨论消息 = 面板 + 总结正文。

面板自上而下：

1. **头部**：模板名、状态（讨论中 / 总结中 / 已完成 / 已停止 / 出错）、轮数、模型数、总结模型，以及「调试」「详情」两个开关。
2. **每轮一条发言**：`第 N 轮` 徽标 + 状态 + 该轮模型胶囊（`模型 1 <model>` 按占位符顺序排列，带状态图标）。点胶囊切换查看该轮哪个模型的发言；内容用 Markdown 渲染并随 delta 流式增长。默认跟随：用户显式选择 → 正在运行的发言 → 第一个有内容的发言 → 第一个模型。
3. **总结**：`总结 <model>` + 状态，紧接其后的消息正文就是总结。
4. **详情**（可折叠）：`全部` 胶囊（点击复位各轮选择并滚到第一轮）+ 轮次用量网格。用量卡片可点击，直接选中并滚动到对应轮次的对应模型。

**颜色区分**：轮次按序号取固定色板（天蓝 / 紫 / 琥珀 / 翠绿 / 玫瑰 / 青 / 靛 / 黄绿 / 橙 / 品红，循环使用），左侧竖条、`第 N 轮` 徽标、用量网格标题同色；总结段固定用主色（primary），与所有轮次区分开。轮次之间仍有分隔线。

**调试模式**：头部「调试」按钮把开关写入 `localStorage`（`deeix:discussion-debug`），对所有讨论消息生效。开启后：

- 每轮的「查看 Prompt」位于思考状态/正文上方。外层展开后，实际发送的消息按 `system` / `user` / `assistant` 分段；每条消息可独立展开/收起，折叠时显示单行摘要，默认展开第一条。切换模型胶囊即可查看同轮其他模型的提示词。
- 开始事件在模型调用前携带经过实际上下文预算裁剪的 Prompt 与结构化消息，因此思考中与流式回答期间即可查看，不必等发言完成；结构化消息也随最终状态持久化，刷新后仍可分段查看。
- 总结段同样支持两层展开/收起。
- 只有旧格式 Prompt 字符串的消息保留整体文本展示；更早且未记录 Prompt 的消息提示「该消息未记录 Prompt」。

模型编号规则：`discussionPlaceholderLabels` 按 `placeholders` 顺序生成「模型 N」，所以即使某轮 `steps` 存储顺序被打乱，展示也永远是 模型 1 … 模型 n。

流式期间不再渲染「空正文骨架」：面板里正在发言，正文为空是正常状态（`message-bot.tsx` 的 `discussionSpeaking`）。

## 6. 后台编辑器

`/admin/discussions`（侧栏名：多轮讨论）。

- **模型占位符**：一行一个，总结模型在最上（带「总结」徽标），下面依次是模型 1 … n；新增按钮是「添加讨论模型」。
- **讨论轮次**：可拖拽排序；每轮只有一个 prompt 文本框；下方「参与模型」是胶囊列表。
  - prompt 按执行方式隔离：`并行提示词` 与 `串行提示词` 分别保存。切换「并行 / 串行」时，若目标方式已经编辑过，恢复它的文本；若从未编辑过，自动填入该方式的默认 prompt（首轮与后续轮不同）。
  - 文本框右上角「恢复默认」按钮把当前执行方式的 prompt 重置为默认模板，不影响另一方式。
  - 首个胶囊是 `全部`：选中后其余胶囊置灰（仍显示为已选），并记住选中前的逐项选择；取消时恢复该选择（没有可恢复的则保留一个模型）。
  - 新增讨论模型会进入每一轮，因此「全部」保持满足；轮次重排会清空记忆（记忆按位置索引）。
- **工作流**：右侧贴边、整屏高的侧栏（xl 23rem / 2xl 27rem），不挤压表单；窄屏则在页面底部展开。
- 保存时会按当前位置重算 `label`，并把 `rounds.index` 归一化为 1..n。

## 7. 对话时的绑定条

选择 `discussion:*` 模型后，输入区上方出现绑定条：

- 总结模型独占一行（主色边框 + 「总结」徽标），讨论模型带电源开关。关闭只影响本次执行，不产生该占位符的发言或用量；总结不可关闭，至少保留一个非空轮次。
- 临时关闭偏好按「会话 + 模板」存入 localStorage（`deeix:discussion-disabled:<conversation>:<template>`），与绑定草稿同生命周期，不写入 active/pending。
- 首次完整成功后，服务端在 `chat_conversation_discussion_snapshots` 按 `(conversation_id, template_public_id)` 保存 active 完整模板和绑定。后台修改或删除模板不影响 active；平台模型被删除时也不自动替换，调用会按原绑定失败。
- 「刷新讨论配置」通过 `POST .../discussion-snapshots/{templateID}/refresh` 生成 pending：已有占位符保留绑定，新占位符使用最新模板默认值，移除的占位符丢弃。pending 阶段下拉可编辑，active 阶段只读。
- 下一次发送携带 `disabledModels` 与 `expectedPendingVersion`。严格完整成功才原子提交 pending；失败、停止或部分失败保留 active 和 pending。`rollback` 清空 pending，部分成功后可用 `adopt` 手动采用。
- 页面刷新只读取 `GET .../discussion-snapshots/{templateID}`，不会自动拉取后台改动。再次刷新需要内联二次确认；版本冲突返回 409，提示重新加载。
- 新会话仍使用本地绑定草稿（`deeix:discussion-bindings:<conversation>:<template>`）；已有会话优先使用快照。后台模板删除后，前端用快照合成 `discussion:<templateID>` 选项，绑定条不会消失。

## 8. 已知边界

- 若回放窗口跳过了 `discussion_start`，面板会从发言事件自建状态、并按发言推导出轮次，仍然流式显示（只是没有模板名与占位符标签）；`discussion_done` 到达后换成完整状态。
- 同一轮内模型并行，正文一次只展示一个模型的发言，其余通过胶囊切换查看。
- 讨论记录会截断（单条 12000 / 整份 60000 字符），超长讨论的总结只基于截断后的记录。
- 讨论消息整体计费为 0（`Billable: false`），各模型的用量记录在 `discussion_json` 中。
- 历史对话条数受 `ContextMaxTurns`（默认 48 轮，压缩策略也用它）约束，超出模型上下文预算时再按 token 裁最老轮次；更早的历史依赖普通对话的压缩摘要机制，讨论自身不做摘要。
- 调试用的提示词只对开启该能力之后生成的讨论消息可用。

## 9. v0.4.6-fork.12 变更明细

本版把讨论从「结束后一次性展示」改成「按轮次流式披露」，并重排了后台与输入区的布局。

### 9.1 讨论按轮次分段发言（`feat: stream every discussion round as its own utterance`）

后端早在发 `discussion_turn_delta` 等事件，但前端把它们当成未知事件丢弃了。本版补齐契约与渲染：

- `apps/web/shared/api/conversation-types.ts`：新增 6 个讨论事件类型与 `DiscussionStreamEvent`。
- `apps/web/shared/api/conversation.ts`：事件类型加入 `STREAM_MESSAGE_EVENT_TYPES`，统一路由到 `onDiscussionEvent`；错误兜底分支改为只在 `type === "error"` 时读取错误字段（原先直接读联合类型字段）。
- `apps/web/features/chat/model/discussion-stream.ts`（新增）：`applyDiscussionEvent` 把事件折叠进实时状态，发言按 `(round, stepId)` 建索引。
- `apps/web/features/chat/types/chat-runtime.ts`：`PendingExchange.assistantDiscussion`。
- `apps/web/features/chat/hooks/use-chat-run-stream.ts`：接收讨论事件；`discussion_summary_delta` 走 `enqueueStreamText` 流式写入正文。
- `apps/web/features/chat/hooks/use-chat-branch-state.ts`：把实时状态映射到 `ChatAreaMessage.discussion`，合并阶段不丢。
- `apps/web/features/chat/model/message-submit-exchange.ts`：结算时采用持久化的讨论状态。
- `apps/web/features/chat/hooks/use-chat-data.ts`：resume 流同样处理讨论事件，切回会话仍能看到轮次进度。
- `apps/web/features/chat/components/message/message-discussion.tsx`：重写为「每轮一条发言 + 胶囊切换 + 用量卡片可点击 + 只剩全部胶囊」。
- `apps/web/features/chat/components/message/message-bot.tsx`：讨论进行中不渲染空正文骨架。
- `backend/internal/application/conversation/service_discussion.go`：`discussion_start` 带上 `placeholders`（前端据此生成「模型 N」）；收尾 `delta` 改为 `replace: true`（总结已流式输出）。

效果：N 轮模板会自主发言 N+1 次（N 轮 + 1 次总结），不再等用户输入。

### 9.2 工作流贴右与「全部」开关（`feat: dock the discussion workflow to the right edge and add a select-all switch`）

- `admin-discussions.tsx`：工作流从 320px 悬浮卡片改成贴右、整屏高侧栏；新增 `全部` 开关与选择记忆（`roundSelectionsRef`），重排轮次时清空记忆。
- `i18n/messages/{zh-CN,en-US}/admin-discussions.json`：新增 `allModels`。

### 9.3 绑定条布局（`feat: give the summary model its own row in the discussion bar`）

- `chat-discussion-config.tsx`：总结模型独占一行；讨论模型 `flex-wrap` + 最小 13rem 横向铺满；`select` 增加 `aria-label`（容器不再是 `<label>`）。

### 9.4 验证

- `pnpm lint`、`pnpm typecheck`、`pnpm check:arch`、`pnpm build` 通过。
- PR CI：API / Web / Desktop / Docker / CodeQL 全绿。
- 线上：`/readyz` 返回 `db=ok, redis=ok`；镜像内二进制含 `discussion_summary_delta`，前端 chunk 含 `allModels`。

## 10. v0.4.6-fork.13 变更明细

### 10.1 调试模式：查看每轮真实提示词

- `backend/internal/domain/discussion/types.go`：`Turn.Prompt`、`State.SummaryPrompt`。
- `backend/internal/application/conversation/service_discussion.go`：`generateDiscussion` 返回归一化后实际发送的消息文本（`renderDiscussionPrompt`），轮次与总结都记录；`discussion_turn_done` / `discussion_round_done` / `discussion_done` 的负载随之携带。
- `backend/internal/application/conversation/service_discussion_payload.go`：`discussionTurnPayload.prompt`、`discussionStatePayload.summaryPrompt`。
- `apps/web/shared/api/discussion-types.ts`：`DiscussionTurn.prompt`、`DiscussionState.summaryPrompt`。
- `apps/web/features/chat/components/message/message-discussion.tsx`：头部「调试」开关（`localStorage: deeix:discussion-debug`）；开启后每轮发言可展开当前模型的完整提示词，总结段可展开总结提示词。
- `i18n/messages/{zh-CN,en-US}/chat.json`：`debug*` 六个文案。

### 10.2 轮次与总结的颜色区分

- 同一文件新增 `ROUND_TONES` 色板（10 色循环）与 `SUMMARY_TONE`：轮次左侧竖条、`第 N 轮` 徽标、用量网格标题同色；总结段固定主色，轮次之间保留分隔线。

### 10.3 追问带上上次讨论结果

- `service_discussion.go` 新增 `discussionPriorContext`：用 `ListMessageAncestors` 取当前分支最近一条有内容的助手消息（截断 8000 字符），拼进每轮与总结的用户消息：

  ```
  上次讨论的结果（同一会话上一轮的总结）：
  <上一轮 assistant 正文>

  用户本次问题：
  <本次输入>

  上一轮及更早的讨论记录：
  <本轮之前的发言>
  ```

- 新增 `discussionTurnUserContent` / `discussionSummaryUserContent` 两个组装函数，保证轮次与总结的上下文结构一致。

### 10.4 事件负载形状修复

之前 `discussion_start`、`discussion_turn_done`、`discussion_round_done`、`discussion_done` 直接把 Go 领域结构体塞进事件，序列化后是 `Round` / `StepID` 等大驼峰字段，前端读不到，导致流式阶段的轮次与状态显示异常（结束后才被 `discussion_done` 的落库状态纠正）。现在统一经 `service_discussion_payload.go` 转成与 `discussion_json` 相同的小驼峰负载。

### 10.5 验证

- `CGO_ENABLED=0 go build ./internal/application/conversation/ ./internal/domain/discussion/ ./internal/transport/http/conversation/` 通过（完整 `go build ./...` 需要 libsqlite3 头文件，属本机环境限制）。
- `pnpm lint` / `pnpm typecheck` / `pnpm check:arch` / `pnpm build` 通过。

## 11. v0.4.6-fork.14 变更明细

### 11.1 输入区模型与实际讨论不一致

- 原因：绑定 effect 在「会话 + 模板」作用域不变时直接返回，而最近一次讨论的绑定（seed）是随消息列表异步到达的。若 seed 到达时作用域已初始化，绑定就停留在模板默认值，看起来像「实际用的模型和对话框里显示的不一样」。
- 修复：`apps/web/features/chat/hooks/use-chat-model-options.ts` 的同作用域分支现在会在 seed 到达或变化时重新套用，并写回 localStorage；用户手动改过模型后不再覆盖。

### 11.2 讨论过程一直显示「未开始」

- 现象：面板只有骨架/未开始，结束后内容一次性出现。
- 原因：generation stream 只保留有限条事件（默认 1024），一次讨论的 delta 很容易把它打满，`discussion_start` 被挤出回放窗口。恢复流（切走再切回、刷新页面）时前端拿不到 start，`applyDiscussionEvent` 在 state 为空时丢弃所有发言事件，于是只能等 `discussion_done`。
- 修复：`apps/web/features/chat/model/discussion-stream.ts` 在没有 state 时也会自建运行中状态；`message-discussion.tsx` 在没有计划（rounds 为空）时按发言推导轮次，模型名取自发言本身。

### 11.3 验证

- 用 Node 直接回放真实前端 reducer：有序列（start → delta → turn_done → round_done）与「无 start 只有 delta」两种场景，状态与轮次推导均正确。
- `pnpm lint` / `pnpm typecheck` / `pnpm check:arch` / `pnpm build` 通过。

## 12. v0.4.6-fork.15 变更明细

### 12.1 讨论历史改为真实多轮消息

之前第二次提问时，每个模型只收到「上次讨论结果 + 本次问题」两条消息，上次的用户原文被丢弃，也与普通对话的多轮结构不一致。

- `backend/internal/application/conversation/service_discussion.go`：
  - 新增 `discussionHistoryMessages`，用 `ListMessageAncestors` 取当前分支历史（排除本次提问），保留 user/assistant 原角色，过滤空内容与不可入历史的消息（`isPromptHistoryMessage`）。
  - 每轮 messages = system + 历史 + 当前 user；总结同理。
  - `discussionTurnUserContent` / `discussionSummaryUserContent` 不再拼接「上次讨论的结果」文本块。
  - `generateDiscussion` 接入 `fitGenerateInputToModelBudget`，按 `route.UpstreamModel` + `ModelCapabilitiesJSON` 做 token 预算裁剪，与普通对话一致；裁剪时记 `logPromptBudgetFit`。
- 调试模式因此能直接看到每次调用带上的完整历史。
- 测试 `TestDiscussionTurnUserContentKeepsQuestionAndTranscript` 锁定新结构（问题 + 本轮记录，无「上次讨论的结果」文本块）。

## 13. v0.4.6-fork.16 变更明细

### 13.1 首 token 前显示真实进度

- 新增 `discussion_turn_start`：每个模型调用前即建立 `running` 发言，不再等首个正文 delta 才从「未开始」变成「思考中」。
- 新增 `discussion_summary_start`：总结模型调用前切换为「总结中」；总结 delta 现在同时更新讨论状态和 assistant 正文，正常发送与恢复流行为一致。
- 新增后端测试，锁定轮次与总结的 start → delta 事件顺序。

### 13.2 总结到达后收起轮次正文

- 讨论与总结生成期间，各轮默认展开并持续流式显示。
- `discussion_done` 带来最终总结后，未操作过的轮次自动收起正文，但保留轮次标题、状态和模型胶囊。
- 点击轮次标题可展开/收起；点击模型胶囊会选中并展开该轮。用户已手动设置展开状态的轮次不被自动收起覆盖。

### 13.3 验证

- 后端 `go test ./internal/application/conversation -count=1` 与 Go 质量检查通过。
- 前端 `pnpm --filter @deeix/web check`、`pnpm --filter @deeix/web build` 通过。
- 用 Node 回放真实 reducer，验证空状态轮次开始、正文 delta、总结开始与总结 delta。

## 14. v0.4.6-fork.17 变更明细

### 14.1 生成期间即可查看实际 Prompt

- `discussion_turn_start` / `discussion_summary_start` 新增 `prompt`，在真正调用模型前携带经过路由规范化与上下文预算裁剪的完整 Prompt。
- 前端开始事件立即保存 Prompt；后续 delta 保留该值，不再等 `turn_done` 或 `discussion_done` 才展示。
- 每轮「查看 Prompt」移到思考状态/正文上方，总结 Prompt 也在总结调用前可见。
- 后端测试锁定 start 事件的 Prompt 内容与 start → delta 顺序；reducer 回放验证开始和流式阶段均保留 Prompt。

### 14.2 验证

- 后端会话包测试与 Go 质量检查通过。
- 前端 `pnpm --filter @deeix/web check`、`pnpm --filter @deeix/web build` 通过。

## 15. v0.4.6-fork.18 变更明细

### 15.1 Prompt 按消息两层折叠

- 保留外层「查看/收起 Prompt」，内层按实际发送的 `system` / `user` / `assistant` 消息逐条展开/收起。
- 内层默认展开第一条，折叠消息显示单行摘要；轮次与总结使用相同交互。
- start 事件与最终状态新增 `promptMessages` / `summaryPromptMessages`，生成中、完成后及刷新后均可分段查看。
- 不通过拆分文本猜测消息边界；只有旧格式 Prompt 字符串的消息回退为整体展示。

### 15.2 验证

- 后端会话包与讨论领域测试、Go 质量检查通过。
- 前端 `pnpm --filter @deeix/web check`、`pnpm --filter @deeix/web build` 通过。
- reducer 回放验证 start/delta 保留结构化轮次与总结 Prompt。

## 16. v0.4.6-fork.19 变更明细

### 16.1 每轮可配置并行或串行

- 后台每轮新增「并行 / 串行」开关，默认并行；拖拽、保存与新增模型不会重置该选择。
- 串行按模型 1、2、3…N 的占位符顺序执行，不依赖 steps 存储顺序。
- 并行轮次共享此前轮次记录；串行轮次每次成功后立即把同轮发言加入后续模型上下文。
- 两种模式分别追加执行说明，串行强调针对具体分歧回应、指出假设和证据问题，避免无意义重复或抬杠。
- 后台工作流与讨论消息显示每轮执行方式；缺少 `parallel` 的旧配置和旧记录默认并行。

### 16.2 验证

- 测试覆盖串行上下文逐模型递增、并行同轮隔离、显式串行配置保留和模型顺序。
- 后端会话包、讨论领域测试及前端检查通过。

## 17. v0.4.6-fork.20 变更明细

### 17.1 每轮 prompt 按执行方式隔离

- 模板配置新增 `rounds[*].parallelPrompt` / `rounds[*].serialPrompt`，两种执行方式各存一份 prompt；`steps[*].prompt` 继续镜像当前方式，兼容旧读取方与运行时回落。
- 后台编辑器切换「并行 / 串行」时恢复目标方式自己的 prompt；目标方式从未编辑过则自动填入默认模板，不再沿用另一方式的文本。
- 每轮 prompt 文本框新增「并行提示词 / 串行提示词」标签与「恢复默认」按钮，一键填入当前方式的默认模板，且不影响另一方式。
- 新增串行默认 prompt：首轮「先独立回答，再针对本轮前序模型的分歧回应」，后续轮「阅读此前轮次及本轮前序模型并回应分歧」；后端在 prompt 为空时也使用这组串行默认值，而不是并行默认值。

### 17.2 验证

- 领域测试覆盖并行 / 串行 prompt 选择、旧配置 `steps[*].prompt` 回落。
- 会话测试覆盖串行首轮与后续轮默认 prompt，确保不会误用并行默认值。
- 前端 `pnpm --filter @deeix/web check`、`pnpm --filter @deeix/web build` 通过；后端 `go test` 与会话包测试通过。
