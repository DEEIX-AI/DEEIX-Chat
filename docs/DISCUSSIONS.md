# 多模型讨论（多轮讨论）

本文档描述「多轮讨论」模板的定义方式、后端编排、流式事件契约与前端渲染规则。代码位置：

| 层 | 位置 |
|---|---|
| 模板领域模型 | `backend/internal/domain/discussion/` |
| 模板 CRUD | `backend/internal/transport/http/discussion/`、`backend/internal/application/discussion/` |
| 讨论编排 | `backend/internal/application/conversation/service_discussion.go` |
| 流事件传输 | `backend/internal/transport/http/conversation/handler_discussion.go` |
| 前端事件折叠 | `apps/web/features/chat/model/discussion-stream.ts` |
| 前端面板 | `apps/web/features/chat/components/message/message-discussion.tsx` |
| 后台编辑器 | `apps/web/features/admin/components/sections/discussions/` |
| 对话时绑定条 | `apps/web/features/chat/components/sections/chat-discussion-config.tsx` |

## 1. 概念

- **模板**在后台定义结构，不绑定真实模型；**占位符**（placeholder）在对话时绑定真实模型。
- **轮次**（round）串行执行；**轮内步骤**（step）并行执行。
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
    { "index": 1, "parallel": true, "steps": [
      { "id": "s_ab12", "participant": "p1", "prompt": "请独立回答用户问题。" }
    ]}
  ],
  "summary": { "participant": "judge", "prompt": "请综合成功发言，给出最终答案。" }
}
```

- 一轮只有一个 prompt：`round.steps[*].prompt` 始终同步为同一个值，编辑器只让用户填一次。
- 参与模型没有先后顺序，`steps` 的数组顺序不决定展示顺序；展示顺序来自 `placeholders`（见 §5）。
- 新增讨论模型会追加到每一轮；删除会从每一轮移除。
- 模板存储于 `chat_discussion_templates`，接口前缀 `/api/v1/admin/discussion-templates`。

对话侧：模型选择器把启用中的模板暴露为 `discussion:<publicID>`，`SendMessage` 请求体携带 `templatePublicID` 与 `models`（占位符 key → 真实模型名）。

## 3. 后端编排

`Service.StreamDiscussionMessage` 的顺序：

1. `ResolvePlan` 解析模板与绑定，产出运行时的 `Rounds`（含每步绑定的模型）与 `Summary`。
2. 复用普通消息的建对逻辑，写入 user / assistant 两条消息，会话模型记为 `discussion:<publicID>`。
3. 逐轮执行：同一轮内所有步骤 `go` 并发，整轮 `wg.Wait()` 后才进入下一轮。
4. 第 1 轮不带历史；第 2 轮起，每个模型都会收到「上一轮及更早的讨论记录」。
5. 讨论结束后调用总结模型；总结失败时用讨论记录拼出回退正文，而不是把整条消息判为失败。
6. 结束时把完整状态写入 `chat_messages.discussion_json`，并把汇总用量写入 assistant 消息。

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
| `discussion_turn_delta` | `round`, `stepId`, `participant`, `model`, `delta` | 按 `(round, stepId)` 追加正文 |
| `discussion_turn_done` | `turn` | 用最终发言替换（含用量、耗时、错误） |
| `discussion_round_done` | `round`, `turns` | 用该轮结果整体替换该轮发言 |
| `discussion_summary_delta` | `delta` | 追加到 assistant 正文，实现总结流式 |
| `discussion_done` | `state` | 换成服务端最终状态 |
| `delta`（收尾） | `delta`, `replace: true` | 用最终正文覆盖流式预览，避免重复 |

所有事件同时写入 generation stream 缓存，因此切走会话再回来时，`resumeMessageGenerationStream` 能按 `afterSeq` 回放（见 `apps/web/features/chat/hooks/use-chat-data.ts`）。

## 5. 前端渲染

一条讨论消息 = 面板 + 总结正文。

面板自上而下：

1. **头部**：模板名、状态（讨论中 / 总结中 / 已完成 / 已停止 / 出错）、轮数、模型数、总结模型。
2. **每轮一条发言**：`第 N 轮` + 状态 + 该轮模型胶囊（`模型 1 <model>` 按占位符顺序排列，带状态图标）。点胶囊切换查看该轮哪个模型的发言；内容用 Markdown 渲染并随 delta 流式增长。默认跟随：用户显式选择 → 正在运行的发言 → 第一个有内容的发言 → 第一个模型。
3. **总结条**：`总结 <model>` + 状态，紧接其后的消息正文就是总结。
4. **详情**（可折叠）：`全部` 胶囊（点击复位各轮选择并滚到第一轮）+ 轮次用量网格。用量卡片可点击，直接选中并滚动到对应轮次的对应模型。

模型编号规则：`discussionPlaceholderLabels` 按 `placeholders` 顺序生成「模型 N」，所以即使某轮 `steps` 存储顺序被打乱，展示也永远是 模型 1 … 模型 n。

流式期间不再渲染「空正文骨架」：面板里正在发言，正文为空是正常状态（`message-bot.tsx` 的 `discussionSpeaking`）。

## 6. 后台编辑器

`/admin/discussions`（侧栏名：多轮讨论）。

- **模型占位符**：一行一个，总结模型在最上（带「总结」徽标），下面依次是模型 1 … n；新增按钮是「添加讨论模型」。
- **讨论轮次**：可拖拽排序；每轮只有一个 prompt 文本框；下方「参与模型」是胶囊列表。
  - 首个胶囊是 `全部`：选中后其余胶囊置灰（仍显示为已选），并记住选中前的逐项选择；取消时恢复该选择（没有可恢复的则保留一个模型）。
  - 新增讨论模型会进入每一轮，因此「全部」保持满足；轮次重排会清空记忆（记忆按位置索引）。
- **工作流**：右侧贴边、整屏高的侧栏（xl 23rem / 2xl 27rem），不挤压表单；窄屏则在页面底部展开。
- 保存时会按当前位置重算 `label`，并把 `rounds.index` 归一化为 1..n。

## 7. 对话时的绑定条

选择 `discussion:*` 模型后，输入区上方出现绑定条：

- 总结模型独占一行（主色边框 + 「总结」徽标）。
- 讨论模型横向铺满，最小宽度 13rem，排不开才换行。
- 绑定按「会话 + 模板」存入 localStorage（`deeix:discussion-bindings:<conversation>:<template>`），新会话回退到模板默认模型。

## 8. 已知边界

- 若回放窗口跳过了 `discussion_start`（同一会话内切走再切回且序号已消费），面板要等 `discussion_done` 才完整；刷新页面会从 0 回放，正常。
- 同一轮内模型并行，正文一次只展示一个模型的发言，其余通过胶囊切换查看。
- 讨论记录会截断（单条 12000 / 整份 60000 字符），超长讨论的总结只基于截断后的记录。
- 讨论消息整体计费为 0（`Billable: false`），各模型的用量记录在 `discussion_json` 中。

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
