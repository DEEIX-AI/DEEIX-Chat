# DEEIX Chat Backend

DEEIX Chat 后端是 Go API 服务，负责认证、用户、对话、模型渠道、模型能力、文件处理、MCP 工具、官方原生工具、记忆、计费、支付、系统设置、审计日志与可观测性等核心业务。

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
- 模型能力 JSON 是请求参数、可视化控件、官方原生工具和图像流式能力的后端事实源。
- 不新增兼容 helper，不保留无意义历史字段；项目早期允许直接重构。

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
# or
cp config.docker.example.yaml config.yaml
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

OCR 引擎配置由后台文件设置管理，当前支持 RapidOCR、Tesseract OCR、Paddle OCR、腾讯云 OCR、阿里云 OCR 与 LLM OCR。服务地址、鉴权密钥和超时时间按具体引擎配置。

用户文件存储配额由运行时设置 `storage:user_storage_quota_bytes` 管理，单位为字节。值为 `0` 表示不限制；非零时，上传、分享克隆和文件复用链路都会按用户维度校验并同步最新配额。前端 `/files` 页支持单个删除和批量删除，后端会在删除后释放对应配额。

## 模型能力与官方原生工具

模型能力 JSON 支持：

- `defaultOptions`：写入用户侧默认参数 JSON，并作为请求参数来源。
- `lockedOptionPaths`：声明不可由用户覆盖的参数路径；对应值仍从 `defaultOptions` 读取，后端发送前会恢复为管理员默认值。
- `optionControls`：定义用户参数配置对话框的可视化控件，不会单独传给上游。
- `nativeToolKeys`：定义当前模型允许的厂商官方原生工具，例如 OpenAI、xAI、Google 和 Anthropic 的原生搜索、代码执行或图片生成能力。
- `image.stream`：仅对图像类模型能力生效；未配置时保持默认流式，显式写 `false` 时关闭图像流式调用。

用户手写 `tools` 时，只有命中 `nativeToolKeys` 的官方原生工具会作为官方工具保留，工具子参数会随该工具透传；普通用户不能通过 JSON 自行启用未被管理员允许的 MCP Tool 或官方原生工具。MCP Tool 仍必须由管理员在工具页配置和启用。

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

## 提交前验证

```bash
go build ./cmd/server
go test ./...
go vet ./...
make swagger
```
