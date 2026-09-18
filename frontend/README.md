# DEEIX Chat Frontend

`@deeix/web` 是 DEEIX Chat 的浏览器端工作区，基于 Next.js App Router 实现对话、文件、知识库、提示词、用户设置和管理员后台。前端负责界面、客户端状态和展示层流程；认证授权、模型路由、文件处理、计费、持久化和审计等业务规则由 Go 后端负责。

前端采用 Next.js 静态导出模式（`output: "export"`）。开发时使用 Next.js 开发服务器，构建后生成 `frontend/out`，生产环境可由 Go 服务托管，也可以交给 Nginx、CDN 或其他静态文件服务。

相关文档：[项目主 README](../README.md) · [后端 README](../backend/README.md) · [API 文档索引](../backend/docs/README.md)

## 技术栈

- Next.js 16.3.4、React 19.2.8、TypeScript 7
- Tailwind CSS 4、Shadcn/UI、Radix UI、Base UI
- Biome 2（lint）
- Streamdown、KaTeX、Mermaid、Recharts、Motion
- `@deeix/api-contract`：从 Go Swagger 契约生成的 TypeScript 类型

## 目录结构

```text
frontend/
├── app/                       # App Router 路由入口、页面和布局
│   ├── (auth)/                # 登录和 OAuth 回调路由组
│   └── (project)/             # 用户工作区、设置和管理员路由组
├── features/                  # 按业务域组织页面组件、hooks 和模型
│   ├── admin/                 # 管理后台
│   ├── announcements/         # 公告
│   ├── auth/                  # 登录和会话流程
│   ├── chat/                  # 对话工作区
│   ├── files/                 # 文件管理与处理状态
│   ├── knowledge-bases/       # 知识库
│   ├── layouts/               # 工作区布局
│   ├── prompts/               # 提示词
│   ├── recent/                # 最近会话
│   ├── settings/              # 用户设置
│   └── share/                 # 分享页
├── entities/conversation/     # 会话实体及其分享、导出能力
├── shared/                    # 跨业务复用的基础能力
│   ├── api/                   # HTTP client、API adapter 和契约类型
│   ├── auth/                  # 会话与访问令牌管理
│   ├── components/            # 跨页面业务组件
│   ├── config/                # 品牌与运行时配置
│   ├── generated/             # 资源同步生成文件
│   ├── hooks/                 # 通用 hooks
│   ├── lib/                   # 通用工具
│   ├── model/                 # 前端模型
│   └── pwa/                   # PWA 资源与迁移
├── components/                # 基础和视觉组件
│   ├── ui/                    # 通用 UI primitives
│   ├── animate-ui/            # 动画组件与图标
│   └── reactbits/             # 视觉效果组件
├── i18n/                      # en-US / zh-CN 国际化资源
├── public/                    # 静态资源
├── scripts/                   # 图标、PWA、截图 worker 等资源同步脚本
├── next.config.ts             # 静态导出与 Next 配置
└── package.json               # @deeix/web workspace 脚本
```

`app/` 中的页面文件负责路由挂载、布局和边界处理。复杂业务放在对应的 `features/<domain>` 中；真正跨业务复用的能力放在 `shared/`。`components/ui`、`components/animate-ui` 和 `components/reactbits` 分别承载基础 UI、动画基础设施和视觉效果；跨页面业务组件放在 `shared/components/`。

### Feature 文件组织

业务域内部按职责拆分：

- `api/`：该业务域的请求封装和契约适配。传输类型从 `@deeix/api-contract` 引入，跨业务的基础接口放在 `shared/api/`。
- `components/`：页面外壳、侧边栏、表格、弹窗、图表和编辑器等业务组件。
- `hooks/`：加载、筛选、乐观更新、轮询和批量操作等状态编排。
- `model/`：纯业务模型、常量、映射和排序规则，不放 React 副作用。
- `types/`：业务域内部的 UI 状态和表单类型，不重复定义后端 wire contract。
- `utils/`：业务域内部的格式化、错误解析和展示工具。

拆分以表达业务边界为目标。简单页面可以保留为单文件，复杂页面再按可见 section 和清晰功能边界拆分。

## 路由

Next.js 的 route group（`(auth)`、`(project)`）只用于组织代码，不会出现在 URL 中。当前页面入口如下：

| 路径 | 用途 |
| --- | --- |
| `/` | 重定向到 `/chat` |
| `/login` | 用户登录 |
| `/auth/callback` | OAuth/OIDC 回调 |
| `/chat` | 对话工作区 |
| `/recent` | 最近会话 |
| `/files` | 文件管理 |
| `/knowledges` | 知识库管理 |
| `/skills-prompt` | 技能与提示词入口 |
| `/share` | 公开分享内容 |
| `/preview/image-loading` | 图片预览加载页 |
| `/setting/general` | 通用偏好 |
| `/setting/chat` | 对话偏好 |
| `/setting/subscription` | 订阅与用量 |
| `/setting/account` | 账户与身份源 |
| `/setting/about` | 产品信息 |
| `/admin` | 管理后台首页 |
| `/admin/about` | 版本信息和更新检查 |
| `/admin/announcements` | 公告管理 |
| `/admin/billing` | 计费与支付 |
| `/admin/chat-files` | 文件、提取、OCR、RAG 和存储配额 |
| `/admin/content-moderation` | 内容审核 |
| `/admin/conversation` | 会话配置与参数策略 |
| `/admin/groups` | 权限组 |
| `/admin/knowledge-bases` | 平台知识库 |
| `/admin/login` | 管理员登录与登录策略 |
| `/admin/logs` | 日志与审计信息 |
| `/admin/models` | 模型、路由、能力和官方原生工具 |
| `/admin/statistics` | 统计信息 |
| `/admin/tools` | MCP 工具 |
| `/admin/upstreams` | 上游渠道 |
| `/admin/users` | 用户与账户 |

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

- `output: "export"`：构建结果写入 `frontend/out`。
- `images.unoptimized: true`：保持静态导出，不依赖 Next.js 图片优化服务。
- `NEXT_PUBLIC_API_BASE_URL`：浏览器请求 API 的地址，会在构建时进入静态资源。

本地开发在 `frontend/.env.local` 设置：

```env
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

分离部署时，必须在构建前设置正式 API 地址：

```bash
NEXT_PUBLIC_API_BASE_URL=https://api.example.com pnpm --filter @deeix/web build
```

`NEXT_PUBLIC_DEV_AUTO_LOGIN`、`NEXT_PUBLIC_DEV_USERNAME` 和 `NEXT_PUBLIC_DEV_PASSWORD` 只用于本地开发调试，不应带入生产构建。产品品牌和其他浏览器运行时配置由后端公开接口提供，因此修改品牌通常只需要更新后端配置并重启服务。

构建后的 `frontend/out` 可以交给静态服务器或 Go 后端托管。静态服务器需要把无扩展名页面映射到对应的 `index.html`；使用 Go 服务时由 `server.frontend_dist_dir` 指向该目录。由于这是静态导出应用，不使用 `next start` 作为生产启动方式。

## 本地开发

以下命令从仓库根目录执行：

```bash
pnpm install
cp frontend/.env.example frontend/.env.local
pnpm dev:web
```

如果需要同时启动 Go API：

```bash
cp config.example.yaml config.yaml
pnpm dev
```

如果本机没有 PostgreSQL 和 Redis，可以启动完整本地依赖：

```bash
docker compose -f docker-compose.full.yml up -d
```

只启动前端时访问 `http://localhost:3000`。后端默认监听 `http://localhost:8080`；未设置 `NEXT_PUBLIC_API_BASE_URL` 时，开发配置默认使用该地址。

从 `frontend/` 目录工作时，等价命令是：

```bash
pnpm dev
pnpm check
pnpm lint
pnpm typecheck
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

- 路由文件保持薄，业务逻辑放在 `features/*` 或合适的实体模块中。
- API 访问统一通过 `shared/api` 或业务域 API 模块完成。
- 保持静态导出能力，不引入依赖常驻 Next.js Server、Server Action 或服务端 API Route 的实现。
- 除 API 定位等构建期常量外，不新增必须重新构建才能修改的品牌环境变量。
- 不在 React render 阶段读取 `window`、`document`、`getComputedStyle` 或本地存储；使用现有 store、effect 或明确的客户端边界，保持静态导出和 hydration 一致。
- Refresh Token 只由后端写入 HttpOnly Cookie，access token 只保存在前端内存中。
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
