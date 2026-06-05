# DEEIX Chat Frontend

DEEIX Chat 前端是基于 Next.js App Router 的管理与对话界面，负责聊天工作区、模型参数配置、文件页、最近会话、用户设置、MCP 工具选择、官方原生工具配置和管理员后台。

## 技术栈

- Next.js 16
- React 19
- TypeScript
- Tailwind CSS
- shadcn/ui 风格组件
- Radix UI / Base UI
- lucide-react
- Streamdown / KaTeX / Mermaid
- Recharts

## 目录结构

- `app/`：Next.js App Router 路由与布局
- `components/ui/`：基础 UI 组件
- `components/common/`：跨页面通用组件
- `features/`：按业务域组织的页面组件、hooks、types、utils
- `features/chat`：对话工作区、输入框、消息渲染、附件、模型参数可视化配置、MCP 工具选择、官方原生工具开关、处理/思考/工具链路展示
- `features/files`：文件管理、上传状态、文件卡片、预览、单个/批量删除和存储配额展示
- `features/settings`：用户侧通用、偏好、订阅和账户设置
- `features/admin`：后台账户、上游、模型、计费、日志、身份源、登录、会话、文件、官方原生工具计费和 MCP 工具设置
- `shared/api/`：API 请求封装与通用类型
- `shared/auth/`：会话 token、登录态与鉴权辅助
- `shared/hooks/`：跨业务复用 hooks
- `shared/lib/`：通用工具函数
- `public/`：静态资源

主要路由：

- `/chat`：对话工作区
- `/files`：文件管理
- `/recent`：最近会话
- `/setting`：用户设置入口
- `/setting/general`：通用偏好
- `/setting/chat`：会话偏好
- `/setting/subscription`：订阅与用量
- `/setting/account`：账户与身份源
- `/setting/about`：产品信息
- `/admin`：后台管理入口
- `/admin/models`：模型、路由、能力 JSON、可视化参数控件和官方原生工具能力
- `/admin/tools`：MCP 工具设置
- `/admin/chat-files`：文件、提取、OCR、RAG 和用户存储配额设置
- `/admin/conversation`：会话配置和参数透传策略
- `/admin/login`：登录、注册、身份源和安全策略
- `/admin/about`：版本信息和新版本检查

## API 契约

标准后端响应统一为：

```ts
export type ApiEnvelope<T> = {
  errorMsg: string;
  data: T;
};
```

前端错误解析统一读取 `errorMsg`。新增接口时不要再使用历史 snake_case envelope 字段。

对话消息的处理轨迹来自后端 `processTrace`，前端按职责渲染为：

- 处理链路：文件预处理、全文注入、RAG、上下文压缩等发送前准备。
- 思考链路：模型 reasoning/think 内容，流式时展开，结束后按时机折叠。
- 工具链路：MCP Tool 调用与模型读取工具结果的循环，支持流式更新和长结果展开。

Markdown 渲染统一使用聊天消息组件，支持基础 Markdown、代码块、表格、脚注、行内/块级公式、图片和链接外跳确认。

模型能力 JSON 中的 `defaultOptions` 会写入用户侧参数 JSON；`optionControls` 只负责用户参数 Dialog 的可视化控件；`nativeToolKeys` 负责展示和提交管理员允许的官方原生工具。用户手写 JSON 时，前端保留用户输入，后端按模型能力和参数策略做最终治理。

应用启动后会通过 `/api/v1/version` 获取 `buildID` 并写入本地缓存，随后低频检查版本变化。检测到新部署后，前端通过 toast 提示刷新，并提供刷新按钮。

## 本地启动

先确保 PostgreSQL、Redis 和后端 API 可用。可以直接使用完整 Docker Compose 启动后端容器：

```bash
cd ..
docker compose -f docker-compose.full.yml up -d
```

也可以在已有 PostgreSQL、Redis 的情况下单独运行后端：

```bash
cd backend
make run
```

启动前端：

```bash
cd frontend
pnpm install
cp .env.example .env.local
pnpm dev
```

访问地址：

```text
http://localhost:3000
```

前端请求后端使用 `NEXT_PUBLIC_API_BASE_URL`。本地开发可写入 `frontend/.env.local`：

```env
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

不配置时，本地默认指向 `localhost:8080`；同源部署默认请求当前 origin。

常用前端环境变量：

- `NEXT_PUBLIC_API_BASE_URL`：后端 API 地址；分离部署时必须在 `pnpm build` 前设置。

## 常用命令

```bash
pnpm dev
pnpm lint
pnpm build
pnpm start
```

当前 `package.json` 未配置单独的 `typecheck` 脚本，需要类型检查时使用框架构建或临时执行 TypeScript 检查命令。

## 开发约束

- 业务页面按 `features/*` 组织，避免把复杂业务逻辑堆在 `app/` 路由文件中。
- 与后端交互统一走 `shared/api` 或对应业务域 API 封装。
- 认证 refresh token 只允许由后端写入 HttpOnly Cookie；access token 只保存在前端内存中。
- 管理后台和用户侧页面复用基础 UI 组件，但业务组件保持边界清晰。
- 图标优先使用 `lucide-react`。
- 新增复杂 UI 时优先复用现有 Dialog、Sheet、Table、Form、Tabs、Switch 等组件风格。
- 不在前端硬编码上游模型私有规则；模型请求参数以模型能力 JSON、用户配置和后端参数策略为准。
- `optionControls`、`nativeToolKeys` 和图像流式开关只做配置展示与提交，最终请求治理由后端执行。
- 文件、MCP 工具、官方原生工具和消息链路展示只消费后端结构化状态，不在前端补业务状态。
- 用户侧和后台侧可以复用基础布局与表格工具，但业务组件不互相穿透。

## 提交前验证

```bash
pnpm lint
```

涉及构建、路由、依赖或 Next.js 配置变更时再执行：

```bash
pnpm build
```
