# 多端架构

本文档定义 DEEIX Chat 的仓库结构、各端职责边界、共享代码规则与安全约束。新增客户端（桌面、移动）必须遵循本文档；与本文档冲突的实现视为架构违规。

## 1. 仓库结构

```
DEEIX-Chat/
├── backend/                  Go 服务，唯一的业务真相（鉴权、授权、模型路由、文件、计费、审计）
├── apps/                     客户端应用，每个平台一个目录
│   ├── web/                  Next.js 静态导出（现有）
│   ├── desktop/              ✅ Tauri 2 壳，加载 apps/web 的构建产物（Rust + keychain + 托盘 + 更新）
│   └── mobile/               Expo / React Native（规划中）
├── packages/
│   ├── api-contract/         后端 Swagger → TypeScript 类型，所有客户端的唯一 API 来源
│   └── core/                 平台无关的客户端逻辑：鉴权状态机、服务器发现、流解析
├── deploy/                   Docker 部署：compose 方案、配置模板、可选服务；可整体拷到服务器
├── scripts/sync-version.mjs  根目录 VERSION 驱动所有包与 Go 版本号
├── docs/
├── Dockerfile                应用镜像（唯一留在根目录的部署文件，构建上下文是仓库根）
└── .github/workflows/
```

依赖方向只允许向下：

```
apps/*  →  packages/core  →  packages/api-contract  →  backend/docs/swagger.json
```

- `apps/*` 之间不得互相 import。
- `packages/core` 不得 import 任何 `apps/*`。
- `packages/api-contract` 只包含生成产物，不得手写逻辑。

## 2. 各端职责边界

逐功能支持情况见 [平台矩阵](./platform-matrix.md)。

| 端 | 定位 | 功能范围 | 不做什么 |
|---|---|---|---|
| Web (`apps/web`) | 完整产品 | 全部功能，包括 Admin | — |
| 桌面 (`apps/desktop`) | Web 的安装版，可**本地独立运行**或连接服务器，**多标签页**同时连多个服务器 | Web 全部功能 + 托盘、自动更新、系统 keychain、OAuth 回环、内置本地服务器（sidecar）、每服务器一个 webview 的标签页 | **不写任何业务 UI 或业务逻辑**。Tauri 工程只包含 Rust 壳、窗口、托盘、更新器、sidecar 进程管理与会话命令 |
| 移动 (`apps/mobile`) | 消费端 | 聊天、会话、文件上传、相机/语音、推送通知、账户设置 | 不做 Admin、账单后台、知识库管理、MCP/Skill 配置 |

原则：如果发现要在桌面工程里写聊天逻辑，说明 Web 端缺了一个抽象；如果发现移动端和 Web 端各写了一份"什么时候该登出"，说明 `packages/core` 缺了一个模块。

### 本地模式（桌面端内置服务器）

桌面端把 Go 后端作为 sidecar 打进安装包，用户可以不部署任何东西直接使用。设计约束：

- **同一份后端代码、同一套安全策略。** 本地模式 = SQLite 部署方案 + 按安装实例生成的密钥（0600）+ `Env=prod` 的全部生产校验。后端只新增 `--local --data-dir` 两个标志和一个仅在本地模式挂载的 grant 兑换端点，不存在任何"本地就放宽"的分支。
- **只监听回环、端口随机。** 父进程通过 stdout 上的一行握手 JSON 拿到端口与一次性登录 grant；stdout 之后不再写任何东西（日志走 stderr）。
- **grant 单次、两分钟、只在 Rust 手里。** 桌面壳用它换会话并把 refresh token 存入 keychain，webview 从头到尾看不到 grant。要拿新 grant 只能重启 sidecar 进程——这是刻意的，grant 不可通过网络签发。
- **本地用户无密码。** `PasswordEnabled=false`，密码登录对它天然不可用；也不会触发首次登录引导。
- **模式切换即登出。** 本地与远程各自一个 keychain 条目，切换时删除另一方，token 不会被重放到另一家运营方。

### 多标签页（桌面端）

一个窗口、一条 40px 的标签栏 webview、每个标签页一个内容 webview。隔离单位是 **webview** 而不是前端状态：每个标签页都是一份完整的 Web 应用实例，有自己的 DOM、缓存、SSE 连接和内存里的会话，前端代码不需要知道"同时有多个服务器"这件事。约束：

- 一个标签页最多绑定一个服务器；两个标签页不会指向同一个服务器（再开一次 = 切到已有的那页）。
- 会话命令按**调用方 webview 的 label** 找服务器，标签页只能碰自己的凭据。同源 `BroadcastChannel` 在桌面端禁用，避免一个服务器的 token 同步到另一个。
- 关闭标签页 = 忘记该服务器：删 refresh token；最后一个本地标签页关闭时停 sidecar。
- 标签栏页面 `/desktop/tabs` 是壳 UI，走同一套设计系统，但只拿到 `tabs_*` 命令；内容标签页拿不到。

## 3. 共享代码规则

### `packages/core`

- **零平台依赖。** `tsconfig.json` 不含 `DOM` lib，`biome.jsonc` 通过 `noRestrictedImports` 禁止 `react`、`next`、`react-native`、`expo`、`@tauri-apps/*`。违规直接导致 `pnpm check` 失败。
- **不做 I/O。** 不直接调用 `fetch`、`localStorage`、`SecureStore`、Tauri command。所有 I/O 通过接口注入，由宿主应用实现。
- **可在 Node 里单测。** `pnpm --filter @deeix/core test` 使用 Node 内置 test runner，不需要浏览器或设备。
- **按需增长。** 只在第二个客户端真的需要时才把逻辑从 `apps/web` 抽进来，不预建空目录。

### UI 不共享，逻辑必须共享

- Web 用 React DOM，移动端用 React Native，两者 UI 层完全独立。
- 鉴权、token 续期、服务器发现、SSE 解析、消息 reducer 这类逻辑只允许存在于 `packages/core`。

## 4. 客户端与服务端的契约

所有客户端都是"连接用户自己部署的服务器"，因此：

- **API 地址是运行时配置，不是构建期常量。** 优先级为"运行时覆盖 → 构建期变量 → 页面 origin"，由 `packages/core` 的 `resolveApiBaseUrl` 统一实现。桌面端在首次启动时让用户填写服务器地址，写入 localStorage，并通过 `registerPlatformApiBaseURLResolver` 注入到 API 客户端。
- **凭据投递方式由后端按请求头决定，不由客户端的构建标志决定。** 客户端发送 `X-Client-Platform: desktop|mobile` 时，后端把 refresh token 放进响应体（客户端存入 keychain / SecureStore）；不带该头时使用 HttpOnly cookie。两条路径共用同一套轮换与吊销逻辑。
- **协议版本协商。** 后端暴露 `serverVersion` 与客户端协议版本；客户端启动时协商，不兼容则明确提示，而不是让功能随机失效。
- **后端不为单一客户端开特例。** 桌面与移动端复用同一套：服务器发现 → 登录 → refresh 续期 → 深链接回调。

## 5. 安全约束

以下约束在加入桌面/移动端时**不得放松**：

1. **CORS 保持显式 allowlist。** 后端 `middleware/cors.go` 是 Origin 白名单 + `Access-Control-Allow-Credentials: true`。桌面 webview 的 origin（如 `tauri://localhost`、`http://tauri.localhost`）必须显式加入白名单，禁止改为 `*`。
2. **Access token 只存内存。** 现有 Web 端以 `Authorization: Bearer` 携带内存中的 access token，这一模型保持不变，任何端都不得把 access token 写入持久存储。
3. **Refresh token 的存放按端区分，读取路径互不回退。**
   - Web：HttpOnly cookie（现状），服务端管理生命周期。
   - 桌面：操作系统 keychain（Service `com.deeix.chat.desktop`，Account `refresh-token:<origin>`）。**JS 没有读取入口**：登录时经 `store_session` 一次性交给 Rust，续期由 Rust 侧 `refresh_session` 对固定 origin 发起并只返回 access token。这使桌面端与浏览器 HttpOnly cookie 等价——页面被 XSS 也拿不到长期凭据。服务器地址同样由 Rust 持久化，切换服务器即丢弃旧会话，token 不会被重放到另一家运营方。
   - 移动：`SecureStore` / Keystore。
   - `X-Client-Platform` 声明为原生客户端时，后端**只**从请求体读 refresh token，且**只**经响应体下发；浏览器路径**只**认 cookie。刻意不做交叉回退，避免两种投递方式混用导致凭据状态不一致。
   - 轮换写回由持有 token 的一方完成（浏览器：服务端 Set-Cookie；桌面：Rust `refresh_session` 内部）。`packages/core` 的 `AuthHost.refreshSession` 只拿到 `{accessToken, sessionID}`。
4. **续期与登出的顺序只有一份。** 401 判定、并发续期去重、续期失败后的清理顺序都在 `packages/core` 的状态机里实现，各端只注入存储与网络。
5. **深链接必须校验。** 自定义 scheme 回调携带的 state/nonce 必须与发起时匹配，防止回调劫持。
6. **桌面端不暴露 Node/系统能力给页面。** `capabilities/main.json` 只授予 `core:default`、窗口聚焦、deep-link 与 updater；不开放 shell、fs、http 插件。CSP 的 `connect-src` 限定为 `self`、IPC 与本地开发端口，不允许页面被注入的脚本外连任意主机。
7. **刷新令牌重用检测。** 轮换后旧 token 保留 15s 宽限期（`refreshTokenPreviousHashGrace`），用于容忍丢失的轮换响应。**宽限期外再次出现已轮换的 token 视为泄露，整个会话立即吊销**（OAuth 2.1 §4.3.1），`revoke_reason = refresh_token_reuse`，并记录 `refresh_token_reuse_detected` 审计事件。这保证攻击者即使在宽限期内截获并使用了旧 token，也无法在受害者下一次刷新后继续持有会话。仓储层的吊销在事务内提交、事务外报告，避免被回滚。
8. **密钥与证书只进 CI secrets。** Apple Developer ID、notarization、Windows 代码签名证书、Android keystore 不入库。

## 6. 版本与发布

- 根目录 `VERSION` 是唯一版本号来源。`scripts/sync-version.mjs` 的 `targets` 表列出所有需要同步的文件；新增客户端只需在表里加一项（如 `tauri.conf.json`、`app.json`）。
- `node scripts/sync-version.mjs --check` 在 `predev` / `prebuild` 与 CI 中执行，版本不同步即失败。
- 一次 tag 触发全部构建：Web → Docker 镜像（现有）；桌面 → 三平台矩阵 + 签名 + updater manifest；移动 → EAS Build。
- Docker 镜像内静态产物路径固定为 `/app/frontend/out`，与 `FRONTEND_DIST_DIR` 及现有部署配置兼容；仓库内构建路径为 `apps/web/out`。

## 7. CI 门禁

```
PR:   pnpm check（所有包 lint + typecheck + 桌面 cargo check）
      pnpm test（所有包）
      api-contract --check（后端 Swagger 变了必须重新生成）
Tag:  上面 + Docker 镜像 + 桌面四目标构建（macOS arm64/x64、Linux、Windows）
```

`turbo.json` 已按包声明依赖，新增 `apps/*` 或 `packages/*` 会自动纳入 `pnpm check` / `pnpm test`。`apps/desktop` 的 `check` 脚本会运行 `cargo check`，因此 PR 阶段即可发现 Rust 侧编译错误，而不必等到发布。

## 8. 落地顺序

每步独立可交付：

1. ✅ 目录重构：`frontend/` → `apps/web/`，建立 `packages/core` 骨架与隔离守卫。
2. ✅ 鉴权状态机进 core：`createAuthClient(host)` 承载并发去重、revision 守卫、401→续期→重试→终态清理；Web 端只剩 `apps/web/shared/auth/auth-client.ts` 这一个 host 适配文件。API 地址改为 `resolveApiBaseUrl` 三级解析（运行时覆盖 → 构建期变量 → 页面 origin），`setRuntimeApiBaseURL()` 是桌面端的接入点。
3. ✅ 桌面端落地：`apps/desktop`（Tauri 2）加载 `apps/web/out`；后端按 `X-Client-Platform` 经响应体投递 refresh token；`apps/web` 的平台层负责服务器地址持久化、keychain 适配与首次启动的服务器设置页；`AuthHost.onSessionRefreshed` 统一处理轮换写回；`deploy/` 与后端默认值已加入 Tauri webview Origin。
4. ✅ CI：PR 阶段 `cargo check`，tag 触发桌面四目标构建 + 签名 + updater manifest（`.github/workflows/desktop-release.yml`）。
5. 移动端立项时再做第二次抽取与 `apps/mobile`；`X-Client-Platform: mobile` 与 `packages/core` 已为它预留。
