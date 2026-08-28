# DEEIX Chat Backend

DEEIX Chat 后端是 Go API 服务，负责认证、用户、对话、模型渠道、模型能力、文件处理、知识库、MCP 工具、官方原生工具、记忆、计费、支付、系统设置、审计日志与可观测性等核心业务。

## 技术栈

- Go 1.26
- Gin
- Gorm
- PostgreSQL + pgvector 或 SQLite + sqlite-vec
- Redis 或进程内 memory cache
- Swagger (`swag`)
- S3 兼容对象存储（可选）
- OpenTelemetry Trace（可选）
- MCP Streamable HTTP JSON-RPC（可选）

## 文档入口

- `docs/README.md`：后端文档索引
- `docs/swagger.json` / `docs/swagger.yaml`：Swagger API 文档

## 核心约束

- 启动链路为 `cmd -> internal/cli -> internal/app`。
- Handler 只负责 HTTP 入参、鉴权上下文、响应转换，不写业务逻辑。
- Application 层承载用例编排，不直接依赖 Gorm、Redis、Docker 等基础设施实现。
- Repository 接口位于 `internal/repository`，具体实现位于 `internal/infra/persistence`。
- 共享基础设施位于 `internal/infra`，通用响应、请求元数据等位于 `internal/shared`。
- HTTP DTO 和 Swagger annotation 是传输契约唯一事实源；Handler 在 HTTP 边界把 DTO 转换为 Application Input，不向领域层或基础设施层泄漏 Gin DTO。
- JSON、校验标签和指针类型必须准确表达必填、可选、可空以及显式 `0`/`false`；不要让前端修补错误的 Swagger 语义。
- 模型能力 JSON 是请求参数、可视化控件、官方原生工具和图像流式能力的后端事实源。
- 只为明确支持的公开契约保留兼容行为，不增加推测性的兼容 helper；破坏性 API 或数据变更必须说明迁移与兼容影响。

## HTTP 响应

标准响应统一为 `errorMsg + data`：

```json
{
  "errorMsg": "",
  "data": null
}
```

分页响应统一放入 `data`：

```json
{
  "errorMsg": "",
  "data": {
    "total": 0,
    "results": []
  }
}
```

所有标准接口通过 `internal/shared/response` 返回，不新增重复 response 包。

## 配置

默认读取仓库根目录下的 `config.yaml`，常用配置也支持环境变量覆盖。从 `backend/` 目录启动时会读取 `../config.yaml`。
本地开发可先在仓库根目录复制示例配置；Docker 部署使用 Docker 示例配置：

```bash
cp config.example.yaml config.yaml
# Docker Compose full stack
cp config.full.example.yaml config.yaml
# SQLite + memory cache
cp config.sqlite.example.yaml config.yaml
```

关键配置：

- `APP_ENV`：运行环境，支持 `dev`/`development` 和 `prod`/`production`；未配置时默认 `prod`
- `HTTP_PORT`：HTTP 端口
- `JWT_SECRET`：JWT 签名密钥
- `POSTGRES_DSN`：PostgreSQL DSN
- `REDIS_ADDR` / `REDIS_USERNAME` / `REDIS_PASSWORD` / `REDIS_DB` / `REDIS_TLS_ENABLED` / `REDIS_TLS_INSECURE_SKIP_VERIFY`：Redis 连接配置；`REDIS_TLS_INSECURE_SKIP_VERIFY` 会跳过证书校验，除非非标准 TLS 端点要求，否则保持关闭
- `STORAGE_BACKEND`：`local` 或 `s3`
- `GEOIP_PROVIDER`：`ipwhois`、`ipinfo`、`mmdb` 或 `none`
- `GEOIP_DATABASE_URL` / `GEOIP_DATABASE_PATH`：MMDB 数据库下载地址与本地缓存路径
- `OTEL_ENABLED`：是否启用 OpenTelemetry Trace；未设置时，配置了 OTLP Endpoint 会自动启用
- `OTEL_EXPORTER_OTLP_ENDPOINT`：OTLP Collector 地址
- `OTEL_EXPORTER_OTLP_HEADERS`：OTLP 请求头，格式为 `key=value,key2=value2`
- `OTEL_EXPORTER_OTLP_INSECURE`：是否使用明文传输
- `OTEL_EXPORTER_OTLP_PROTOCOL`：OTLP exporter 协议，支持 `grpc`、`http`、`http/protobuf`，默认 `grpc`
- `OTEL_TRACES_SAMPLER_ARG` / `OTEL_SAMPLING_RATE`：Trace 采样率，范围 `0~1`

对应 YAML：

```yaml
observability:
  tracing:
    # 未配置 enabled 时，endpoint 非空会自动启用 Trace。
    # enabled 为 true 时 endpoint 必填；enabled 为 false 时强制关闭。
    # enabled: true
    endpoint: "http://127.0.0.1:4317"
    headers: ""
    insecure: true
    protocol: grpc
    sampling_rate: 1
```

`config.yaml` 是静态基础设施配置入口，环境变量优先级高于 YAML。未显式配置 `enabled` 时，`endpoint` 非空会自动启用 Trace；显式配置 `enabled: true` 时，`endpoint` 必填。运行时业务设置由数据库 settings 覆盖，不把 OpenTelemetry collector、header/token 等部署层配置放入后台管理。

初始化超级管理员用户名为 `admin`。当数据库中没有超级管理员时，后端会生成随机密码并只在首次创建账号的启动日志中输出一次，日志关键字为 `bootstrap superadmin created`。首次登录会强制修改用户名和密码；后续账号变更不通过 `config.yaml`。

`APP_ENV` 未配置时默认 `prod`。`dev`/`development` 只用于本地开发；公网生产部署应保持 `APP_ENV=prod` 或 `APP_ENV=production` 并使用生产密钥。

## 邮箱注册 Turnstile

邮箱注册可选启用 Cloudflare Turnstile 人机验证，作用范围仅限邮箱注册；OAuth/OIDC 登录或注册不需要 Turnstile 校验。

相关运行时设置：

- `auth:turnstile_registration_enabled`：是否在邮箱注册时启用 Turnstile。
- `auth:turnstile_site_key`：前端渲染 Turnstile 组件使用的 Site Key，会通过 `/api/v1/auth/login-options` 返回。
- `auth:turnstile_secret_key`：后端调用 Cloudflare siteverify 使用的 Secret Key，属于敏感设置。
- `TURNSTILE_SITEVERIFY_URL` / `security.turnstile_siteverify_url`：可选覆盖 siteverify 端点，默认使用 Cloudflare 官方地址。

启用 Turnstile 需要同时启用 `auth:email_registration_enabled`，并配置 Site Key 与 Secret Key。开启邮箱验证码注册时，前端在 `/api/v1/auth/register/email/start` 提交 `turnstileToken`；关闭邮箱验证码但允许邮箱注册时，前端在 `/api/v1/auth/register/email/complete` 提交 `turnstileToken`。

## OAuth 公共客户端授权桥（多端暂未发布）

Web、App 和桌面端统一通过当前实例完成第三方 OAuth 回调。部署必须提供外部可访问的 `PUBLIC_API_BASE_URL`，身份源回调格式为：

```text
<PUBLIC_API_BASE_URL>/api/v1/auth/providers/<provider-slug>/callback
```

`POST /auth/providers/:slug/authorize` 创建短时事务并使用服务端独立 PKCE 访问上游；`GET /auth/providers/:slug/callback` 在服务端兑换上游授权码；`POST /auth/providers/:slug/exchange` 使用公共客户端 PKCE verifier 原子兑换一次性 DEEIX grant。事务与 grant 使用现有 Redis/内存缓存后端，外部 provider code、Client Secret 和 Token 均不会进入公共客户端。旧 `/start` 与 `POST /callback` 流程继续保留，用于账号身份绑定与旧版 Web 客户端兼容。

生产环境安全校验：

- `APP_ENV` 支持 `dev`/`development` 和 `prod`/`production`，其他值会启动失败。
- `APP_ENV=prod` 时，`JWT_SECRET` 不能为空、不能过短、不能使用默认开发值。
- `APP_ENV=prod` 时，`DATA_ENCRYPTION_KEY` 不能为空、不能过短、不能使用默认开发值。
- `APP_ENV=prod` 时，`CORS_ALLOW_ORIGIN` 不能为空或 `*`，`PUBLIC_API_BASE_URL` / `PUBLIC_WEB_BASE_URL` 必须是 HTTPS。

Stripe Webhook 使用公开 API 地址：

```text
https://api.example.com/api/v1/billing/payments/stripe/webhook
```

在 Stripe Dashboard 中监听 `checkout.session.completed`，并把生成的 `whsec_...` 填入后台「计费 / 支付配置 / Stripe Webhook Secret」。

易支付当前采用 `submit.php` 页面跳转 + MD5 签名协议。后台「页面跳转网关」可填写易支付站点地址或完整的 `submit.php` 地址，例如：

```text
https://pay.example.com
https://pay.example.com/epay/
https://pay.example.com/epay/submit.php
```

系统会为站点地址自动追加 `/submit.php`，并兼容既有的子目录站点配置。要求直接提交商户密钥的私有支付 API 不属于该协议；商户密钥只用于服务端签名，不会加入支付跳转 URL。

## 本地启动

先确保 PostgreSQL 和 Redis 可用。若本机已有依赖，可以只启动默认应用容器；若需要完整本地栈，使用 `docker-compose.full.yml`：

```bash
docker compose up -d
```

```bash
docker compose -f docker-compose.full.yml up -d
```

启动后端：

```bash
cd backend
make run
```

Swagger UI：

```text
http://localhost:8080/swagger/index.html
```

## 存储

默认本地存储：

```yaml
storage:
  backend: local
  local:
    root_dir: ./storage
```

S3 兼容对象存储：

```yaml
storage:
  backend: s3
  s3:
    endpoint: ""
    region: auto
    bucket: ""
    prefix: ""
    access_key_id: ""
    secret_access_key: ""
    force_path_style: true
```

R2、OSS、MinIO、AWS S3 等统一走 S3 兼容协议，不为不同厂商维护重复实现。

管理员为模型、厂商或展示分组上传的自定义图标属于实例级公共展示资产，三处共用同一图标库和对象存储，但不计入任何用户的文件空间或配额。后端仅接受不超过 1 MiB、最大边长 2048 像素的 PNG、JPEG 与 WebP，并按内容哈希去重。管理员可从图标库移除无引用资产；移除后立即隐藏，但在连续 24 小时无引用保护期内仍可公开读取。后台任务会在物理删除前再次检查模型、厂商、展示分组及保留的会话运行快照引用；重新上传或保存引用会自动取消待回收状态。图标读取接口无需登录，图标内容不应包含敏感信息。

内置技术厂商受系统保护，不允许删除。自定义厂商仅在没有平台模型引用时可删除；冲突响应会返回关联模型总数和有界预览，管理员需先将这些模型迁移到其他厂商。删除自定义展示分组时，组内模型会自动恢复为按技术厂商展示。

## 向量存储

Embedding 输出支持 64–4096 维。系统会通过 OpenAI-compatible `dimensions` 参数请求目标维度，并校验上游实际返回的向量长度；维度不一致时明确失败，不会通过截断或补零伪装成目标维度。PostgreSQL 按模型原始维度保存向量，SQLite 因 vec0 固定槽限制在持久化边界补零至 4096 维。文件、历史消息和用户记忆向量都会记录模型、服务端点与维度共同生成的空间签名，检索只使用当前向量空间的数据。

PostgreSQL 使用 4000 维 `halfvec` HNSW 表达式索引召回候选，再按完整 4096 维向量精确重排；这样既避开标准 `vector` ANN 的维度限制，也不会用降维距离作为最终排序结果。使用 PostgreSQL 时需安装 pgvector 0.8.0 或更高版本，以支持 `halfvec`、HNSW 迭代扫描和 4096 维 `vector` 存储；启用向量能力时，版本不满足要求会在启动迁移阶段明确失败。

从旧版本升级时，PostgreSQL 会移除 `vector(1536)` 的固定维度约束，但不会扩展或重写已有向量行；SQLite 的 `FLOAT[1536]` vec0 表会迁移为固定 4096 维并在尾部补零。旧 PostgreSQL IVFFlat 索引会通过并发 DDL 替换为 HNSW 候选索引。没有向量签名的既有文件会进入待重建状态，旧历史消息和用户记忆向量在重新生成前不会参与语义检索。大型 PostgreSQL 实例首次构建 HNSW 索引仍可能消耗较多时间与数据库资源，应在维护窗口升级并先完成数据库备份。

管理员切换 Embedding 模型、服务地址或输出维度时，系统只将不属于新空间的文件标记为待重建，不修改原始文件。后台重建按固定并发执行并按文件任务签名原子发布；新空间任务领取后，旧任务即使更晚完成也不能覆盖新分片或状态。1536 与 4096 可以双向切换，但切换完成前相关文件暂不参与新空间检索。

## GeoIP

默认使用 HTTP GeoIP 服务：

```yaml
geoip:
  provider: ipwhois
```

生产环境如果希望降低外部依赖并提升审计稳定性，可改用本地 MMDB 数据库：

```yaml
geoip:
  provider: mmdb
  database_url: "https://example.com/geoip.mmdb"
  database_path: "./data/geoip/geoip.mmdb"
  database_max_bytes: 104857600
  refresh_interval_hours: 168
  timeout_ms: 2500
```

启用 `provider: mmdb` 时，启动会优先加载本地文件；本地文件不存在或过期时，根据 `database_url` 下载并校验新数据库。刷新成功后热切换内存中的 reader，刷新失败则保留上一份可用数据库。

## 文件处理

文件链路支持三类上下文策略：

- 图片：默认按模型能力直接传原图上下文；开启图片 OCR 后进入 OCR 文本提取链路。
- 文本类文件：小文件可全文注入；超出阈值时按配置走 RAG 或回退策略。
- PDF/Office 等文档：通过内置提取、Tika、Docling、MinerU 或 OCR 引擎提取文本；PDF OCR 回退可单独控制。

MinerU 可在设置中选择处理的文件类型；云端 MinerU 支持 `.doc/.docx/.ppt/.pptx/.xls/.xlsx`，自部署 MinerU 支持 `.docx/.pptx/.xlsx`。

OCR 引擎配置由后台文件设置管理，当前支持 RapidOCR、Tesseract OCR、Paddle OCR、腾讯云 OCR、阿里云 OCR、Mistral OCR 与 LLM OCR。服务地址、鉴权密钥和超时时间按具体引擎配置。

用户文件存储配额由运行时设置 `storage:user_storage_quota_bytes` 管理。后台 `/admin/chat-files` 页面中的 `storage:max_upload_file_bytes`、`storage:user_storage_quota_bytes`、`file:image_max_bytes`、`file:doc_max_bytes` 和 `file:file_full_context_max_bytes` 统一按 MB 输入，设置值在 API、数据库和运行时内部统一按字节保存与计算；值为 `0` 表示不限制。非零时，上传、分享克隆和文件复用链路都会按用户维度校验并同步最新配额。前端 `/files` 页支持单个删除和批量删除，后端会在删除后释放对应配额。

## 知识库

知识库是独立于文件管理页面的检索集合。后端按 `domain/knowledgebase -> application/knowledgebase -> repository.KnowledgeBaseRepository -> infra/persistence/postgres/knowledgebase -> transport/http/knowledgebase` 分层，用户与管理员接口统一使用 `/api/v1/knowledge-bases` 资源路径。知识库只关联文件对象，不复制文件内容；删除知识库时仅在用户明确选择后清理未被其他资源引用的文件。

## 模型能力与官方原生工具

模型能力 JSON 支持：

- `defaultOptions`：写入用户侧默认参数 JSON，并作为请求参数来源。
- `lockedOptionPaths`：声明不可由用户覆盖的参数路径；对应值仍从 `defaultOptions` 读取，后端发送前会恢复为管理员默认值。
- `optionControls`：定义用户参数配置对话框的可视化控件，不会单独传给上游。
- `nativeToolKeys`：定义当前模型允许的厂商官方原生工具，例如 OpenAI、xAI、Google 和 Anthropic 的原生搜索、代码执行或图片生成能力。
- `image.stream`：仅对图像类模型能力生效；未配置时保持默认流式，显式写 `false` 时关闭图像流式调用。

用户手写 `tools` 时，只有命中 `nativeToolKeys` 的官方原生工具会作为官方工具保留，工具子参数会随该工具透传；普通用户不能通过 JSON 自行启用未被管理员允许的 MCP Tool 或官方原生工具。MCP Tool 仍必须由管理员在工具页配置和启用。

## 模型熔断

模型与上游两级熔断默认关闭，可在后台模型管理页统一开启。旧版本的 `circuit_breaker.defaults` 设置没有 `enabled` 字段时同样按关闭处理，不需要逐个模型调整阈值。

关闭后，路由不会读取熔断状态，也不会因上游失败累计并触发自动熔断；HTTP 429 的路由级短期退避仍独立生效。退避优先采用上游 `Retry-After`，没有有效响应头时使用有上限的指数退避；同一上游的其他路由不会被连带暂停，成功请求会清除对应路由的累计退避。重新开启熔断前必须成功清理已有模型与上游熔断状态和失败计数；关闭后的清理由系统尽力执行。最近成功/失败健康元数据、API Key 轮询状态与限流状态不会被清理。

## 上游动态请求头

上游和路由的附加请求头支持动态变量模板。变量仅在真实会话生成请求中展开；模型列表、渠道探测、标题与标签生成、上下文压缩和媒体任务不会携带这些标识。

- `${DEEIX_CONVERSATION_ID}`：公开会话 ID，适合自建网关按会话关联日志、缓存和长期记忆。
- `${DEEIX_SESSION_ID}`：会话上下文键，适合需要独立会话亲和键的自建网关。
- `${DEEIX_REQUEST_ID}`：DEEIX 当前请求 ID，同一轮生成、工具调用和路由重试保持一致，适合关联完整请求链路。
- `${DEEIX_UPSTREAM_REQUEST_ID}`：每次上游 HTTP 请求单独生成的 UUID，适合要求请求级唯一标识的 Provider。

功能默认关闭。管理员可在目标上游或路由的附加请求头中显式配置；未配置的官方 Provider 不会收到额外动态标识：

```json
{
  "X-Conversation-Id": "${DEEIX_CONVERSATION_ID}",
  "X-Session-Id": "${DEEIX_SESSION_ID}",
  "X-Request-Id": "${DEEIX_REQUEST_ID}"
}
```

OpenAI 的 `X-Client-Request-Id` 要求每次请求使用唯一值，应配置为 `${DEEIX_UPSTREAM_REQUEST_ID}`，不要使用稳定的会话或链路标识。

部分 Codex 兼容中转站会把 Codex CLI 使用的会话头作为账号或缓存分片的亲和键。仅在中转站明确支持该约定时，可在目标路由（不要在官方 OpenAI 上游）配置：

```json
{
  "session-id": "${DEEIX_SESSION_ID}",
  "thread-id": "${DEEIX_CONVERSATION_ID}",
  "x-client-request-id": "${DEEIX_CONVERSATION_ID}"
}
```

其中 `session-id` 与 DEEIX 发送的 `prompt_cache_key` 使用同一个会话上下文键。该配置只提供中转站亲和提示，不能替代 `promptCache` 能力声明，也不应作为所有 OpenAI 兼容上游的全局默认值。

## OpenAI Prompt Cache

官方 OpenAI Responses 与 Chat Completions 请求会使用同一会话上下文键作为服务端受控的 `prompt_cache_key`，以保持跨轮缓存亲和；未启用显式模式时仍使用 OpenAI 默认的 implicit breakpoint 行为。兼容中转站默认不接收 OpenAI Prompt Cache 新字段；确认中转站支持后，需要在模型能力 JSON 中显式声明：

```json
{
  "promptCache": {
    "enabled": true
  }
}
```

官方 OpenAI 也可以用 `promptCache.enabled=false` 显式关闭。缓存策略完全由模型能力配置控制，用户消息请求中的同名 Options 会被忽略。启用显式缓存时，官方 OpenAI 默认发送消息块断点；兼容中转站必须再声明 `messageBreakpoints=true` 才会收到 `prompt_cache_breakpoint`。DEEIX 会把最后一条非空的前导 system 消息和每条非空历史 user 消息保留为断点；本轮 user、动态 RAG、本轮图片及其他当前轮上下文始终不标记。旧轮次断点必须继续保留，避免删除 marker 后改写后续缓存前缀；正常连续对话每轮只为上一轮 user 新增一个断点。OpenAI 每个请求最多创建 4 个新写入，并从最近 50 个对话断点中读取最长匹配前缀。该历史策略只作用于 explicit 模式，不改变仅使用稳定 `prompt_cache_key` 的 implicit 行为：

```json
{
  "promptCache": {
    "enabled": true,
    "mode": "explicit",
    "ttl": "30m",
    "messageBreakpoints": true
  }
}
```

若中转站接受顶层 `prompt_cache_options`，但拒绝消息内容中的 `prompt_cache_breakpoint`，省略 `messageBreakpoints` 或将其设为 `false`。此时 DEEIX 仍发送稳定的 `prompt_cache_key` 和显式缓存选项，由中转站选择缓存边界。

显式缓存当前只接受 `ttl=30m`。隐式缓存使用上游默认保留策略；DEEIX 不配置或透传保留策略。未声明能力的兼容中转站不会收到 `prompt_cache_key`、`prompt_cache_options` 或 `prompt_cache_breakpoint`。DEEIX 不再依赖上游错误文本执行无记忆缓存重试。

## MCP 工具

MCP 能力由后台工具设置管理：

- 后台配置 MCP Server，服务地址必填，可配置鉴权密钥与请求头。
- Server 创建后默认启用，可同步远端 MCP Tool。
- 工具可单独启停；用户在聊天输入区选择可用工具。
- 单次 run 支持最大 LLM 调用轮数、最大工具调用次数、并发数、超时和失败重试配置。
- 工具调用结果会进入消息处理轨迹，前端与“处理链路 / 思考链路”并列展示工具链路。

计费侧把一次用户触发的多轮 LLM + 工具调用视为一次 run 汇总统计。

官方原生工具按上游返回的调用次数生成独立服务项；是否计费和每次调用价格由管理员在计费设置中统一配置，价格填 `0` 表示不单独计费。工具返回内容产生的模型 token 仍按模型定价计算。

## 版本信息

`GET /api/v1/version` 是公开接口，返回当前版本、提交、构建时间和 `buildID`，用于前端定期检测新部署并提示用户刷新。该接口设置为 no-store，避免被 CDN 或浏览器长期缓存。

## 可观测性

后端日志保持 JSON 外层字段克制，业务上下文集中写入 `msg`，`/healthz` 不输出访问日志。生产环境 Gorm `record not found` 不作为错误日志输出。

OpenTelemetry Trace 为可选能力，当前覆盖：

- Gin HTTP 请求，跳过 `/healthz`
- PostgreSQL Gorm callback
- Redis 命令
- S3 Put/Open/Delete，其中 Open 覆盖完整 reader 生命周期
- 出站 HTTP：LLM、MCP、Embedding、OAuth/OIDC、GeoIP、文件提取/OCR
- 会话关键路径：发送、RAG 检索、LLM 生成、工具执行、持久化

Trace 不记录 prompt、文件内容、工具参数、API Key 或鉴权密钥。

## 可选文件处理服务

Apache Tika：

```bash
docker compose -f ../docker/tika/docker-compose.yml up -d
```

Tesseract OCR：

```bash
docker compose -f ../docker/tesseract/docker-compose.yml up -d --build
```

Docling：

```bash
docker compose -f ../docker/docling/docker-compose.yml up -d --build
```

RapidOCR：

```bash
docker build -t deeix-chat-rapidocr ../docker/rapidocr
```

这些服务默认使用 `deeix-chat-network`。可先执行 `docker network create deeix-chat-network`，或先启动一次根目录 compose 创建基础网络。

## 常用命令

```bash
make run
make fmt
make test
make swagger
go build ./cmd/server
go vet ./...
go mod tidy
```

接口或 DTO 变更后必须执行：

```bash
make swagger
```

该命令会调用根工作区的 `pnpm api:generate`，使用 `backend/go.mod` 中锁定的 `swag` 版本，并同时更新：

- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`
- `packages/api-contract/src/types.generated.ts`

这些文件全部由生成器维护，不允许手工修改。仅检查漂移时，在仓库根目录运行 `pnpm api:check`。

## 提交前验证

```bash
go build ./cmd/server
go test ./...
go vet ./...
cd ..
pnpm api:check
```
