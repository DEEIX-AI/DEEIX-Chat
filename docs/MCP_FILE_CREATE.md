# MCP 工具产物创建为用户文件（create-only 评审提案）

关联 [#818](https://github.com/DEEIX-AI/DEEIX-Chat/issues/818)。本文描述本 Draft PR 的可运行契约，**不代表维护者已批准协议**。基线为 `dev@19d60fec8f9356a4bdd997e8b71a5a8a9a0baec9`。

## 授权和范围

管理员在已有 MCP 服务创建/更新 API 中显式设置 `fileCreateEnabled: true`，授权该服务在持久化聊天的实际 `tools/call` 中自动创建一个当前用户文件。默认关闭；省略此字段的更新保留原值。首版仅通过管理员 API 配置，不增加设置页。

这个开关是管理员对自动创建的授权，**不是逐次用户确认**。没有把模型传入的 `confirm=true` 当成用户同意。临时聊天、附件预处理和工具发现不签发凭证。

DEEIX 在获取现有工具并发许可后，为一次实际调用生成 32 字节随机 capability，通过 `X-Deeix-File-Create-Token` 仅发给登记服务的 `tools/call`。自动重试沿用同一凭证和调用 ID；签名用户上下文存在时，其 `jti` 使用同一调用 ID。`request_id` 只作请求关联，不是文件幂等键。重新发起聊天或新工具调用取得新的授权，不能当作前一次调用的重试。

数据库 `mcp_file_create_grants` 只保存 capability 的 SHA-256，绑定服务、用户、工具、调用、服务撤销代数和过期时间。**只能由 DEEIX 签发和验证**：既不接受共享 HMAC 签名头作为反向授权，也不复用用户 JWT、MCP 出站 token 或模型 `_meta`。外部服务无法自行签发、改归属或扩大操作范围。凭证是 bearer capability；取得它就取得该次受限创建权，不是另一套长期服务身份平台。

有效期固定 10 分钟，不能续期。管理员更改服务状态、权限、地址、鉴权 token 或 headers 会增加撤销代数；配置快照过期时拒绝签发新授权。禁用工具、停用用户或关闭全局 MCP 会同时永久失效相关未过期凭证，重新启用不恢复旧授权。验证还读取数据库中的当前全局设置及用户/工具/服务状态；数据库不可用时失败关闭。已进入文件提交事务的调用与禁用操作以锁顺序线性化：先完成的提交可成功，禁用完成之后的新提交不能成功。注销浏览器会话、取消聊天本身不撤销独立的 10 分钟 capability。

凭证不进入模型参数、工具发现、初始化握手或静态 headers；带凭证的 MCP 请求禁止跨 origin 重定向。结果中的原文及 JSON 转义凭证在存储/展示前移除。MCP 服务仍必须保护其日志和网络传输，不应把凭证返回到工具结果。生产使用 HTTPS 或受控内部网络。

## 字节交接

外部服务向其登记 ID 的端点上传字节：

```http
POST /api/v1/mcp/servers/42/files
Authorization: Bearer <本次 tools/call 收到的 capability>
Content-Type: multipart/form-data; boundary=...

... 唯一一个名为 file 的文件 part ...
```

外部 MCP 的 DEEIX 地址及服务 ID 由部署者提供，不从模型参数或客户端 Host 头推断。示例：

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $DEEIX_FILE_CREATE_TOKEN" \
  -F 'file=@report.txt;type=text/plain' \
  'https://chat.example/api/v1/mcp/servers/42/files'
```

只允许一个 `file` part，不允许 `user_id`、`purpose`、目标 `file_id`、URL、额外文件/字段或 query 参数。归属来自已验证授权，用途固定为 `mcp_output`。DEEIX 不拉取工具提供的 URL。

HTTP 总体积限制为既有 `MaxUploadFileBytes` 加 1 MiB multipart 开销；应用层仍按实际字节校验文件上限、类型允许列表、危险类型和分类大小限制。复用 `UploadService.PrepareTemporaryFile`、对象存储 port、文件元数据创建、内容去重和配额事务。类型判定沿用当前上传策略，包括扩展名归类，**不新增格式完整性/恶意软件扫描保证**。已有策略不允许的格式（例如未加入允许列表的 PPTX）仍被拒绝。

新文件成为用户拥有的正式 `file_objects` 行，计入同一存储配额，出现在原有 `/api/v1/files` 列表。有效的同用户相同内容可复用既有文件 ID，不重复扣配额；已有对象缺失或校验失败时返回冲突，委托入口不修复/删除已有用户文件。

现有浏览器 `/api/v1/files` 仍只接受登录 JWT/session，不接受 capability 或任意 MCP 签名头。新端点先套用既有按 IP 公开限流，再按授权用户套用 `file_upload` 限流策略；沿用当前默认关闭、缓存错误失败开放等行为，不额外引入一套限流配置。

## 返回与重试

成功返回 HTTP 200、既有 `errorMsg + data` envelope：

```json
{
  "errorMsg": "",
  "data": {
    "file_id": "file_example",
    "file_name": "report.txt",
    "size_bytes": 123,
    "sha256": "<64 hex characters>",
    "reused": false,
    "replayed": false,
    "processing_status": "queued",
    "extract_status": "none",
    "embed_status": "none",
    "processing_ready": false
  }
}
```

`file_id` 是可靠的创建/复用结果。`reused` 表示内容去重，`replayed` 表示本次 capability 已成功提交过。处理状态从数据库重新读取；创建成功不保证文本提取、OCR、向量化或 RAG 已就绪，状态可随后变化，甚至失败。原有用户文件 UI 继续使用现有处理查询。

| 情形 | 结果 |
| --- | --- |
| 未授权、伪造、过期、撤销、错服务 | 401 `mcp.file_create.unauthorized` |
| 非唯一文件、额外字段、非法元数据 | 400 `mcp.file_create.invalid_input` |
| 类型拒绝 | 400，沿用既有文件错误码 |
| 文件或 HTTP 总体积超限 | 413 |
| 配额不足 | 409，沿用既有配额错误码 |
| 同一 capability 改变文件内容、规范化文件名或声明 MIME；既有去重内容不可用 | 409 `mcp.file_create.conflict` |
| 成功结果后来被删除 | 404 `mcp.file_create.result_gone`，不补建文件 |
| 存储/数据库暂时失败或提交结果不确定 | 500，用**同一 capability、同一文件**在有效期内重试 |
| 限流 | 429，沿用既有策略 |

首个通过文件策略校验的提交，在对象 I/O 前持久绑定规范化文件名、声明 MIME、实际 SHA-256 和大小。后续即使首次配额/存储失败也只能重试这份文件；被类型/大小校验拒绝的请求尚未绑定。没有另一个客户端任意选择的 idempotency key，也不将内容摘要当成请求 ID。

提交按数据库 receipt 锁串行，receipt 的 `file_id`、文件行与配额扣减在同一事务中提交。SQLite 使用其已有单写者模型，PostgreSQL 支持共享 DB/对象存储上的多个服务实例。最小实现持有用户配额锁完成最多两分钟的请求/对象 I/O；这是明确的吞吐上限，没有引入通用分布式任务/权限框架。

候选 ID 和精确存储 key 在写字节前持久化。对象写入成功但回包丢失、事务回滚、提交确认丢失时，不凭一次错误删除候选：同一提交可校验并恢复同一个 key；不会改用新 ID 或再次扣配额。文件/receipt 提交前不启动处理。

每分钟有界巡检最多 100 条记录，复用现有处理服务恢复 `uploaded/queued` 的入队；初始化采用条件状态更新，现有 memory/Redis 处理队列按未结算用户/文件任务去重，worker 仍使用原有 attempt-ID fencing。没有新任务队列。Redis 入队确认丢失可复用仍在 stream 的任务；内存队列重启丢失时 receipt 可补交。凭证过期不延长授权；未完成处理交接的记录保留到可以确认交接，暂时数据库读取失败保留记录。过期候选无活跃引用时清理，删除失败则保留待重试；正式文件和跨用户共享引用不会被清理。

过期并完成交接的 receipt 被清理，幂等回放窗口就是凭证的 10 分钟有效期。过期后只能在已有用户文件列表确认产物，没有 MCP 文件查询入口。

## 审计、迁移和评审点

沿用既有审计服务，动作 `mcp_create_file`，记录目标用户、登记服务/工具 ID、调用 ID、文件 ID、实际大小及 `reused/replayed`；不记录 capability、文件内容或文件名。沿用当前审计的结构化日志与独立 DB 写入流程，不承诺审计和文件提交是同一事务。进程在提交后、审计前退出的窗口仍可能缺少成功审计，receipt 支持恢复创建但不是审计 outbox。

使用项目既有 schema migration 注册一个 receipt 表及 MCP 服务的默认关闭开关/撤销代数。没有新 YAML、环境变量、共享存储卷、反向 JWT 或依赖。Swagger 与 `packages/api-contract/src/types.generated.ts` 由 `pnpm api:generate` 生成。

维护者需决定的事项：

1. 是否接受管理员按服务授权后自动创建的 v1 语义，以及临时聊天/预处理暂不授权。
2. 是否接受每调用一文件、10 分钟 bearer capability、上传端点/头命名与回放窗口。
3. 是否需要在合并前提供管理员 UI，或将成功审计与文件结算进一步做成原子记录。

## 验证入口

```sh
make -C backend lint test
pnpm api:check
pnpm --filter @deeix/web check

# 可选真实 PostgreSQL 和专用 Redis 集成测试
DEEIX_TEST_DATABASE_DSN='postgres://.../test?sslmode=disable' \
  go -C backend test ./internal/transport/http/mcp -run TestFileCreatePostgresConcurrentInstances

# 使用 Redis DB 15；必须是专用测试实例，该测试会清空 DB 15。
DEEIX_TEST_REDIS_ADDR='127.0.0.1:6379' \
  go -C backend test ./internal/infra/cache/redis -run TestFileProcessingRedisRecoveryEnqueueIsIdempotent
```
