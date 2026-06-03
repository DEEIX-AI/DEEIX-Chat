<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./frontend/public/logo-white.svg" />
    <img src="./frontend/public/logo-black.svg" alt="DEEIX Chat" width="160" />
  </picture>
</p>

<p align="center">
  面向企业模型路由、多模态对话、文件、工具、计费、身份和运维的一体化 AI 工作台。
</p>

<p align="center">
  <a href="./README.md">English</a> | 简体中文
</p>

<p align="center">
  <a href="https://deeix.com"><img alt="官网" src="https://img.shields.io/badge/官网-deeix.com-black" /></a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0"><img alt="开源协议" src="https://img.shields.io/badge/License-Apache%202.0-blue" /></a>
  <img alt="Next.js" src="https://img.shields.io/badge/Next.js-16-black" />
  <img alt="React" src="https://img.shields.io/badge/React-19-149eca" />
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26-00add8" />
</p>

## 项目简介

DEEIX Chat 为团队提供统一的 AI 工作台，用一个清晰的使用入口承载多个上游模型和服务商。它将多模态对话、模型路由、文件与 RAG、MCP 工具、用量计费、身份认证、审计日志和运维控制整合到同一个产品中。

系统围绕简单部署、高效静态分发和可预期的 Go 运行时占用设计。后台集中管理上游渠道、平台模型名、路由优先级、定价、订阅、用户和安全策略，对话工作区则保持稳定、专注的用户体验。

![DEEIX Chat 工作区](./frontend/public/DEEIX-Chat.jpg)

## 核心能力

| 模块 | 能力 |
| --- | --- |
| 对话体验 | 多分支会话、流式响应、重试、编辑、反馈、公开分享、克隆分享会话、富文本 Markdown、文件卡片、模型元信息、用量明细和执行链路。 |
| 媒体生成 | 独立的图片生成和图片编辑链路，按任务类型路由到 OpenAI、Google 和 xAI 的原生图片协议，生成结果统一入库为文件，支持预览、下载和独立运行记录。 |
| 模型控制面 | 平台模型目录、上游渠道、真实上游模型、路由绑定、优先级/权重路由、能力 JSON、展示顺序、厂商映射、自动图标和熔断状态。 |
| 协议适配 | OpenAI Responses、Chat Completions、Images Generations 和 Images Edits，Anthropic Messages，Google/Gemini Generate Content 和 Image Generation，xAI Responses、Images Generations 和 Images Edits，OpenRouter 默认协议和自定义 OpenAI 兼容路由。 |
| 请求治理 | 按协议组装上游请求，支持用户参数白名单/黑名单、系统保护字段、协议支持时的 previous response 续接，以及可回看的上下文快照。 |
| 文件与 RAG | 文件上传、预览、下载、删除、配额控制、MIME 探测、文本提取、OCR、全文上下文注入、图片上下文、分片、向量嵌入和语义检索。 |
| 记忆与上下文 | 消息数截断、Token 预算截断、上下文压缩、会话记忆、用户长期记忆、RAG 证据记录和提示词链路查看。 |
| 工具调用 | 后台管理 MCP Server、工具发现、工具启停、用户侧工具选择、执行轮数限制、重试、链路渲染和工具结果处理。 |
| 计费与支付 | 订阅套餐、充值、余额账户、按 Token/按次/按秒/阶梯定价、免费模型、预付费阈值、用量账本、计费快照、Stripe Checkout、易支付和 Webhook 校验。 |
| 身份与安全 | 本地登录、注册、会话管理、HttpOnly Refresh Cookie、2FA/TOTP、恢复码、可信设备、SSO/OIDC/OAuth 身份源、联系方式验证、时区和语言区域。 |
| 后台管理 | 用户、角色、身份源、上游、平台模型、路由绑定、模型定价、订阅、余额、调用日志、审计日志、认证事件、系统事件和运行时设置。 |
| 运维能力 | 高效静态分发、可预期的 Go 运行时占用、Docker 构建、单运行时托管前端和 API、Swagger、结构化日志、请求 ID、Redis 缓存、PostgreSQL pgvector、可选 GeoIP、可选 OpenTelemetry 和 S3 兼容存储。 |

<p>
  <img src="./frontend/public/DEEIX-Chat-Image.png" alt="DEEIX Chat 图片生成" width="32%" />
  <img src="./frontend/public/DEEIX-Chat-Dark.png" alt="DEEIX Chat 深色模式" width="32%" />
  <img src="./frontend/public/DEEIX-Chat-Usage.png" alt="DEEIX Chat 用量与计费" width="32%" />
</p>

## 架构

```text
frontend/  Next.js App Router 前端应用
backend/   Go API 服务、领域/应用层、基础设施适配和 Swagger 文档
docker/    可选文档提取和 OCR 服务
```

后端按分层结构组织：

```text
cmd -> internal/cli -> internal/app
transport/http -> application -> repository interfaces -> infra implementations
domain -> 领域类型和常量
pkg -> 无业务依赖的技术工具
```

数据库使用领域前缀组织身份、模型路由、计费、对话、文件、RAG、设置、工具、审计日志和系统事件等表。财务流水、审计日志、系统事件和高增长向量数据保持独立事实源。

## 技术栈

- 前端：Next.js 16、React 19、TypeScript、Tailwind CSS、shadcn/ui 风格组件、Radix/Base UI、Streamdown、KaTeX、Mermaid、Recharts、Motion
- 后端：Go 1.26、Gin、Gorm、PostgreSQL、pgvector、Redis、Swagger、OpenTelemetry、Zap
- 存储：本地文件系统或 S3 兼容对象存储
- 文件处理：内置提取、Apache Tika、Docling、RapidOCR、Tesseract OCR、Paddle OCR、云 OCR 适配、MinerU、LLM OCR 回退
- 工具协议：MCP Streamable HTTP JSON-RPC
- 运行依赖：Docker、Docker Compose、PostgreSQL、Redis

## 快速开始

### 本地开发

```bash
cp config.example.yaml config.yaml
cd backend
make run
```

```bash
cd frontend
pnpm install
cp .env.example .env.local
pnpm dev
```

地址：前端 `http://localhost:3000`，API `http://localhost:8080`，Swagger `http://localhost:8080/swagger/index.html`。

前端请求后端使用 `NEXT_PUBLIC_API_BASE_URL`。本地开发可写入 `frontend/.env.local`：

```env
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

不配置时，本地默认指向 `localhost:8080`；同源部署默认请求当前 origin。

### Docker 部署

优先级：`环境变量 > config.yaml > 代码内置默认值`。

Docker 镜像工作目录是 `/app`。使用默认 compose 挂载时，后端会自动读取 `/app/config.yaml`：

```yaml
volumes:
  - ./config.yaml:/app/config.yaml:ro
```

如果需要指定其他配置文件路径，使用 `CONFIG_FILE`，值必须是容器内路径：

```yaml
environment:
  CONFIG_FILE: "/app/config.yaml"
```

自定义配置文件路径只读取 `CONFIG_FILE`。如果 compose 的 `environment` 和 `config.yaml` 同时配置同一个键，环境变量优先。`docker-compose.full.yml` 默认在 `environment` 中写入了 `POSTGRES_DSN`、`REDIS_ADDR`、`REDIS_PASSWORD`，因此会覆盖 `config.yaml` 中的 PostgreSQL 和 Redis 配置。

`config.yaml` 只负责静态基础设施和安全配置，例如服务地址、数据库、Redis、存储、GeoIP、Trace、JWT 和加密密钥。运行时业务配置存储在数据库中，并通过后台管理修改；这些配置启动后不以 YAML 为准。

`APP_ENV` 支持 `dev`/`development` 和 `prod`/`production`，内部会规范化为 `dev` 或 `prod`；未配置时默认 `prod`。`dev` 只用于本地开发；公网生产部署应保持 `APP_ENV=prod` 或 `APP_ENV=production` 并使用生产密钥。

#### 轻量启动

只启动 `app` 容器，PostgreSQL 和 Redis 使用外部服务。适合已有数据库/缓存的部署环境。

```bash
cp config.docker.example.yaml config.yaml
# 修改 database.postgres.dsn 和 database.redis.*
docker compose up -d
```

这种方式主要使用 `config.yaml`。除非明确希望用环境变量覆盖配置文件，否则 compose 中不需要额外写同名 `environment`。

#### 全量启动

启动 `app`、`postgres`、`redis` 三个容器。适合本机试用、开发自测或无外部数据库的单机部署。

```bash
cp config.docker.example.yaml config.yaml
docker compose -f docker-compose.full.yml up -d
```

这种方式会通过 compose 环境变量连接内置 PostgreSQL 和 Redis。如果希望这些连接配置完全来自 `config.yaml`，需要修改 `docker-compose.full.yml` 或删除对应 `environment` 项。

默认应用镜像为 `ghcr.io/deeix-ai/deeix-chat:latest`。测试自定义构建时可通过 `DEEIX_CHAT_IMAGE` 覆盖：

```bash
DEEIX_CHAT_IMAGE=deeix-chat:local docker compose up -d --build
```

Docker 地址：`http://localhost:8080`。除非修改端口或公网域名，否则不要改 Docker 配置里的 `server` 段；如需修改，端口映射、公开 URL 和 CORS 必须一起调整。

排查配置是否挂载成功时，可检查容器内文件和启动日志：

```bash
docker compose exec app ls -l /app/config.yaml
docker compose logs app
```

#### 分离部署

当前端和后端分别暴露在不同公网地址时使用分离部署，例如 `https://chat.example.com` 和 `https://api.example.com`。

1. 配置公开地址。

   - 前端构建变量：`NEXT_PUBLIC_API_BASE_URL=https://api.example.com`
   - 后端配置：`server.public_api_base_url=https://api.example.com`
   - 后端配置：`server.public_web_base_url=https://chat.example.com`
   - 后端配置：`server.cors_allow_origin=https://chat.example.com`

   Docker 镜像构建时需要传入前端 API 地址：

   ```bash
   docker build --build-arg NEXT_PUBLIC_API_BASE_URL=https://api.example.com -t deeix-chat .
   ```

2. 构建并发布前端。

   ```bash
   cd frontend
   pnpm install
   NEXT_PUBLIC_API_BASE_URL=https://api.example.com pnpm build
   ```

   静态产物在 `frontend/out`，可由 Nginx、CDN、对象存储或任意静态服务托管。如需由 Go 后端托管前端，把 `frontend/out` 放到 `server.frontend_dist_dir` 指向的目录；Docker 镜像默认是 `/app/frontend/out`。

3. 配置 CDN 规则。

   | 路径 | 规则 |
   | --- | --- |
   | `/_next/static/*` | 缓存 1 年，并启用 immutable 静态资源缓存。 |
   | `/logo*.svg`、`/*.ico`、`/*.png`、`/*.jpg`、`/*.webp`、`/*.woff2` | 缓存 1 天到 30 天。 |
   | `/`、`/*.html`、`/chat*`、`/recent*`、`/files*`、`/setting*`、`/admin*`、`/share*` | 不做长期缓存，建议使用 `no-cache` 或较短 TTL。 |
   | `/api/*`、`/healthz`、`/readyz`、`/swagger/*` | 绕过 CDN 缓存，并完整转发请求头、方法、查询参数和请求体。 |

   如果 CDN 从对象存储托管 `frontend/out`，需要开启路由回退，让无扩展名地址能命中导出的 `index.html`，例如 `/chat` -> `/chat/index.html`。

4. 如果启用 Stripe，配置 Stripe Webhook。

   在 Stripe Dashboard 添加此 Endpoint：

   ```text
   https://api.example.com/api/v1/billing/payments/stripe/webhook
   ```

   启用 `checkout.session.completed` 事件，并将 Stripe 生成的 `whsec_...` 签名密钥填入后台「计费 -> 支付配置 -> Stripe Webhook Secret」。该 Endpoint 必须绕过 CDN 缓存，并保留原始请求体和 `Stripe-Signature` 请求头。

## 主要路由

- `/chat`：对话工作区
- `/share`：公开会话快照页面
- `/recent`：最近会话、分享状态、星标和归档状态
- `/files`：文件管理
- `/setting`：用户账户、订阅、偏好、安全设置和产品信息
- `/admin`：后台管理

## 常用命令

后端：

```bash
cd backend
go build ./cmd/server
go test ./...
go vet ./...
make swagger
```

前端：

```bash
cd frontend
pnpm lint
pnpm build
```

## 配置说明

静态基础设施配置从仓库根目录的 `config.yaml` 读取，并支持环境变量覆盖。运行时业务配置存储在 `system_settings`，由后台管理。

Docker 部署默认通过 `./config.yaml:/app/config.yaml:ro` 挂载并读取 `/app/config.yaml`。如需自定义路径，使用 `CONFIG_FILE` 指向容器内路径。

前端构建期变量：

| 变量 | 作用 |
| --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | 浏览器请求后端 API 的地址；本地写入 `frontend/.env.local`，分离部署在构建时传入。 |

常用后端环境变量：

| 变量 | 作用 |
| --- | --- |
| `APP_ENV` | 运行环境。支持 `dev`/`development` 和 `prod`/`production`；未配置时默认 `prod`。 |
| `CONFIG_FILE` | 可选的配置文件路径，Docker 场景应填写容器内路径；默认 compose 挂载后会读取 `/app/config.yaml`。 |
| `HTTP_PORT` | API/运行时端口。 |
| `JWT_SECRET` | JWT 签名密钥，生产环境必须使用强随机值。 |
| `DATA_ENCRYPTION_KEY` | 上游 API Key、SSO Client Secret、MCP Token、敏感设置和 TOTP Secret 的加密密钥材料。 |
| `POSTGRES_DSN` | PostgreSQL DSN。 |
| `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB` | Redis 连接配置。 |
| `STORAGE_BACKEND` | `local` 或 `s3`。 |
| `STORAGE_ROOT_DIR` | 本地存储目录。 |
| `STORAGE_S3_ENDPOINT`, `STORAGE_S3_REGION`, `STORAGE_S3_BUCKET`, `STORAGE_S3_PREFIX`, `STORAGE_S3_ACCESS_KEY_ID`, `STORAGE_S3_SECRET_ACCESS_KEY` | S3 兼容对象存储配置。 |
| `PUBLIC_API_BASE_URL`, `PUBLIC_WEB_BASE_URL` | 用于链接、回调和公开地址生成的外部访问地址。 |
| `GEOIP_PROVIDER` | GeoIP 服务。默认 `ipwhois` 使用内置公共地址。 |
| `OTEL_ENABLED`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_INSECURE`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_SAMPLING_RATE` | OpenTelemetry Trace 配置。 |

生产模式会拒绝不安全的默认密钥、过短的加密密钥、通配 CORS 和非 HTTPS 公开地址。

初始化超级管理员用户名固定为 `admin`，密码会在数据库中不存在超级管理员时随机生成，并只在首次创建账号的后端启动日志中输出一次。首次登录会强制修改用户名和密码；后续账号变更通过账户流程完成，不再通过 `config.yaml` 修改。

获取初始化管理员密码时，请查看首次启动后端服务的日志，搜索 `bootstrap superadmin created`；其中的 `username` 和 `password` 分别是初始登录用户名和密码。如果数据库中已经存在超级管理员，服务不会重新生成或再次输出该密码。

## 安全说明

- 用户密码使用 bcrypt 哈希存储。
- Refresh Token 和恢复类凭证只存储哈希。
- 上游 API Key、SSO Client Secret、MCP 鉴权 Token、敏感系统设置和 TOTP Secret 使用 `DATA_ENCRYPTION_KEY` 通过 AES-GCM 加密。
- Access Token 为短期令牌并保存在前端内存中；Refresh Token 由后端写入 HttpOnly Cookie。
- 用户输入的模型参数会在请求上游前经过白名单/黑名单过滤。模型名、消息、工具、系统提示词、请求头和 previous response 标识等系统链路字段不允许被用户 options 覆盖。

## 可选服务

下面的 compose 文件会接入 `deeix-chat-network`。可先执行 `docker network create deeix-chat-network`，或先启动一次根目录 compose。

Apache Tika：

```bash
docker compose -f docker/tika/docker-compose.yml up -d
```

Tesseract OCR：

```bash
docker compose -f docker/tesseract/docker-compose.yml up -d --build
```

Docling：

```bash
docker compose -f docker/docling/docker-compose.yml up -d --build
```

RapidOCR：

```bash
docker build -t deeix-chat-rapidocr ./docker/rapidocr
```

这些服务都是可选能力，具体启用哪个提取或 OCR 引擎由后台文件设置决定。

## 文档入口

- 后端说明：[backend/README.md](./backend/README.md)
- 后端规范：[backend/docs/README.md](./backend/docs/README.md)
- 前端说明：[frontend/README.md](./frontend/README.md)
- 贡献指南：[CONTRIBUTING.md](./CONTRIBUTING.md)
- 安全策略：[SECURITY.md](./SECURITY.md)
- Swagger UI：`http://localhost:8080/swagger/index.html`

## 鸣谢

DEEIX Chat 基于开源生态构建，感谢所有 AI 工具生态中的维护者和社区。

- [Next.js](https://nextjs.org)
- [Go](https://go.dev)
- [LINUX DO](https://linux.do)

## 联系&交流

- 邮箱：[support@deeix.com](mailto:support@deeix.com)
- Telegram：[t.me/deeix_chat](https://t.me/deeix_chat)

## 开源协议

DEEIX Chat 使用 [Apache License 2.0](./LICENSE) 授权。
