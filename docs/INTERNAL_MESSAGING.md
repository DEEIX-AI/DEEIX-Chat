# 站内消息（VoceChat）部署说明

DEEIX 使用自己的 UI 与 API；VoceChat 仅在 Docker 内网保存消息并提供实时事件。浏览器不会直接访问 VoceChat，也不会拿到它的 `X-SECRET`、文件路径或访问令牌。

当前版本支持最近会话、持久化未读与多设备同步、历史分页、消息搜索、置顶与静音、浏览器通知、图片/文件、回复、复制、编辑与撤回。聊天按钮和窗口的位置、尺寸按用户保存在浏览器。语音和群聊不在本版本范围。

管理员可在“管理 → 站内消息”动态启停入口，设置单文件上限、每用户文件配额、保留天数和浏览器通知策略，并查看 VoceChat 健康、SSE 连接、请求失败、延迟、索引消息数与文件占用。VoceChat 地址和 third-party secret 仍属于启动级敏感配置，不通过管理页面写入。

## Docker Compose 自动初始化

全量 Compose 已经内置 VoceChat，首次启动时会构建一个包含 VoceChat 服务、`config.toml` 和 `init.py` 的 bundle 镜像。轻量安装和默认安装则需要叠加 `docker-compose.vocechat.yml`。构建使用已经通过 DEEIX 集成验证的 `privoce/vocechat-server:v0.5.32`，不要改成浮动的 `latest`：

```bash
docker compose -f docker-compose.full.yml up -d --build
```

轻量和默认方案使用 overlay，网络名都是 `deeix-chat-network`：

```bash
docker compose -f docker-compose.sqlite.yml -f docker-compose.vocechat.yml up -d --build
docker compose -f docker-compose.yml -f docker-compose.vocechat.yml up -d --build
```

后面的恢复、轮换命令按默认安装写了 `-f docker-compose.yml`。如果实际运行的是轻量方案，把第一份 compose 文件换成 `docker-compose.sqlite.yml`；如果使用全量方案，只保留 `-f docker-compose.full.yml`。

`vocechat-init` 会在 VoceChat 健康后自动完成以下操作：

1. 首次部署时生成随机管理员凭据并调用初始化 API；凭据只保存在独立的 `vocechat_init_state` 卷。
2. 获取 third-party secret，以 `0400` 写入 `vocechat_secrets` 卷。
3. DEEIX app 只读挂载 secret 卷，通过 `INTERNAL_MESSAGING_SECRET_FILE` 读取密钥。
4. 初始化成功后该容器退出，DEEIX app 才开始启动。后续启动会验证并复用已有密钥。

运行时不需要挂载 `docker/vocechat/init.py` 或 `docker/vocechat/config.toml`；它们在 bundle 镜像构建阶段复制进去，构建上下文也只有 `docker/vocechat`。修改这两个文件后需要重新构建。Compose 仍保留两个服务，因为 VoceChat 服务需要持续运行，而 `vocechat-init` 只在初始化或恢复时运行一次。

使用该 overlay 时不需要在 `config.yaml` 中填写 `secret`，也不需要手工创建 VoceChat 管理员。overlay 会自动设置：

```yaml
internal_messaging:
  enabled: true
  vocechat_url: "http://vocechat:3000"
  secret_file: "/run/secrets/vocechat/third-party-secret"
  timeout_ms: 10000
```

随附的 VoceChat 配置已经开启 `[login] third_party = true`，不要关闭。

## CDN 与反向代理

浏览器只访问 DEEIX。不要把 VoceChat（容器内 `:3000`）映射到宿主机、反代或 CDN。`docker-compose.vocechat.yml` 只用 `expose`，没有 `ports`，请保持这一边界。

站内消息走已有的 DEEIX 源，路径都在 `/api/v1/internal-messaging/*` 和 `/admin/internal-messaging`：

| 路径 | 规则 |
| --- | --- |
| `/api/*` | 绕过 CDN 缓存，完整转发请求头、方法、查询参数和请求体。 |
| `/api/v1/internal-messaging/events` | 长连接 SSE。关闭响应缓冲，拉长读超时和空闲超时。 |
| `/api/v1/internal-messaging/users/*/files` | 附件上传。请求体上限至少为管理页单文件上限加 1 MiB（默认 20 MiB，最大 100 MiB）。 |
| `/api/v1/internal-messaging/messages/*/file` | 鉴权下载。浏览器可按 `private, max-age=3600` 缓存；CDN 不得缓存，否则可能把私聊文件提供给其他用户。 |
| `/admin*`、`/chat*` 等 HTML | 与现有前端规则相同，不要长期缓存。`/admin/internal-messaging` 已包含在 `/admin*` 中。 |

AI 对话流式接口同样需要关闭缓冲；站内消息的 SSE 是常驻连接，更容易被边缘节点的默认超时切断。API 已发送 `X-Accel-Buffering: no` 和 `Cache-Control: no-cache, no-transform`。反代仍需显式关闭缓冲，否则 Nginx 会攒齐响应再发给浏览器，实时消息会卡住。

同源反代示例（应用监听 `127.0.0.1:8080`）：

```nginx
client_max_body_size 101m;

location /api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Connection "";
}
```

分离部署时，把上述 `/api/` 规则打在 API 源上，HTML 规则打在静态/CDN 源上。不要为 VoceChat 再配一条公网 location。

边缘 CDN 注意：

- `/api/*` 必须 bypass，不能当静态对象缓存。
- 部分厂商免费档会在约 100 秒切断 SSE。连接被掐后前端会重连并补历史，但未读角标和在线状态会滞后；需要稳定实时通道时，选用支持长连接的套餐，或让浏览器直连源站反代。
- 上传走 `multipart/form-data`，确认 CDN 没有把 POST body 截断到小于管理页文件上限。

## 本地源码调试

一条命令同时启动 VoceChat、DEEIX API 和 DEEIX Web：

```bash
./scripts/dev-deeix.sh
```

脚本以前台方式运行 DEEIX，Web 明确监听 `0.0.0.0:3000`，启动时会检测主机主要 IPv4 并同时输出本机和外部访问地址。按 `Ctrl+C` 后会停止开发容器但保留本地数据。已有根目录 `config.yaml` 时脚本会沿用它；没有时自动使用隔离的 SQLite/内存缓存配置，数据位于 `.deeix/deeix/`。

自动检测适用于直接拥有可达 IPv4 的主机。NAT、多网卡或域名访问时通过 `DEEIX_DEV_ADVERTISE_HOST` 指定浏览器实际使用的地址：

```bash
DEEIX_DEV_ADVERTISE_HOST='203.0.113.10' ./scripts/dev-deeix.sh
```

脚本会把该地址同步传入 Next.js 的 `allowedDevOrigins`，因此外部浏览器可以加载开发资源和 HMR。需要同时允许多个域名或 IP 时可使用逗号分隔：

```bash
DEEIX_DEV_ADVERTISE_HOST='203.0.113.10' \
DEEIX_NEXT_ALLOWED_DEV_ORIGINS='203.0.113.10,dev.example.com' \
  ./scripts/dev-deeix.sh
```

开发 Web 和 API 会临时暴露在网络上，请用防火墙或安全组限制来源，并在调试完成后按一次 `Ctrl+C` 关闭。启动器会删除临时 API 容器并终止整个 Turbo/Next/pnpm 进程组，最多等待 3 秒后强制清理，避免残留后台进程继续占用 `3000` 或 `8080`。VoceChat 仍只监听 `127.0.0.1`，并会随启动器退出而停止。

后端使用固定 Go 1.26.8 的开发工具链容器，不依赖宿主机的 `make` 或 Go 版本。第一次运行会构建该轻量镜像并下载 Go 模块，后续启动复用 `.deeix/deeix/` 下的模块与构建缓存；源码和 SQLite 数据仍位于当前工作区。

默认 Web 编译器是 Turbopack。内存较小的开发机可以按需为 Next.js 的 V8 堆设置保护上限，不会关闭聊天功能：

```bash
DEEIX_DEV_NODE_HEAP_MB=1024 \
  ./scripts/dev-deeix.sh
```

`DEEIX_DEV_WEB_ENGINE` 支持 `turbopack`（默认）和 `webpack`，用于必要的兼容性对比。当前完整 `/chat` 模块图在 Webpack 下需要超过 1024 MiB V8 堆，因此低内存机器应继续使用 Turbopack。`DEEIX_DEV_NODE_HEAP_MB` 最低为 `512`，设置过低会让开发编译因堆不足退出，因此它是保护上限而不是通用的节省比例。

如需覆盖浏览器访问的 API 地址或保留 VoceChat 容器：

```bash
DEEIX_DEV_API_URL='http://server.example:8080' \
DEEIX_DEV_WEB_URL='http://server.example:3000' \
DEEIX_DEV_KEEP_VOCECHAT=true \
  ./scripts/dev-deeix.sh
```

以下命令只启动 VoceChat，适合分别调试前后端：

一条命令启动并自动初始化本地 VoceChat：

```bash
./scripts/dev-vocechat.sh up
```

脚本会输出可直接放入本地 `config.yaml` 的配置。生成的数据、管理员状态和 secret 位于被 Git 忽略的 `.deeix/vocechat/`；不需要复制或显示 key。

每次执行 `up` 都会重新创建无状态的开发容器，以确保端口、镜像、配置和目录挂载使用本次参数；消息数据、管理员状态和 secret 会继续复用 `.deeix/vocechat/` 中的持久化内容。

当 `VOCECHAT_DEV_BIND=0.0.0.0` 时，脚本会检测主机的主要 IPv4 地址，并用该地址输出可访问 URL 和 `vocechat_url`。多网卡、NAT 或需要公布公网地址时，可通过 `VOCECHAT_DEV_ADVERTISE_HOST` 明确指定，例如：

```bash
VOCECHAT_DEV_BIND=0.0.0.0 \
VOCECHAT_DEV_PORT=3101 \
VOCECHAT_DEV_ADVERTISE_HOST=203.0.113.10 \
  ./scripts/dev-vocechat.sh up
```

常用命令：

```bash
./scripts/dev-vocechat.sh status
./scripts/dev-vocechat.sh logs
./scripts/dev-vocechat.sh down
```

`down` 只移除开发容器，保留消息数据和密钥。

默认只监听 `127.0.0.1:3001`。SSH 远程开发时，建议继续使用本地监听并通过 SSH 转发端口。如果确实需要允许局域网或外部主机直接访问，可重建开发容器并设置监听地址：

```bash
./scripts/dev-vocechat.sh down
VOCECHAT_DEV_BIND=0.0.0.0 ./scripts/dev-vocechat.sh up
```

也可通过 `VOCECHAT_DEV_PORT` 修改宿主机端口。监听 `0.0.0.0` 会将 VoceChat 暴露到服务器网络，请同时使用防火墙或安全组限制来源。

## 恢复与轮换

正常重启不需要任何管理员输入。如果 VoceChat 数据仍在，但 `vocechat_secrets` 卷损坏，初始化器会使用独立状态卷中的凭据恢复 secret。

如果 secret 和初始化状态卷同时丢失、而 VoceChat 数据仍然存在，则必须提供已有管理员凭据一次，不能绕过 VoceChat 权限边界：

```bash
VOCECHAT_ADMIN_EMAIL='admin@example.test' \
VOCECHAT_ADMIN_PASSWORD='replace-me' \
  docker compose -f docker-compose.yml -f docker-compose.vocechat.yml \
  run --rm \
  -e VOCECHAT_ADMIN_EMAIL \
  -e VOCECHAT_ADMIN_PASSWORD \
  vocechat-init
```

轮换 third-party secret：

```bash
docker compose -f docker-compose.yml -f docker-compose.vocechat.yml \
  run --rm -e VOCECHAT_ROTATE_SECRET=true vocechat-init
docker compose -f docker-compose.yml -f docker-compose.vocechat.yml restart app
```

已用 VoceChat OpenAPI `0.5.32` 验证的接口包括：`/health`、第三方密钥与登录、用户资料同步、文本发送与历史、SSE 事件、回复、编辑、撤回、文件上传与鉴权下载。具体方法和路径由 `scripts/check-vocechat-upgrade.sh` 维护；升级镜像前还需重新验证第三方登录开关和消息时间格式。

### 备份与恢复

开发环境应先停容器再备份三个关联目录，避免消息数据库、secret 与初始化凭据处于不同时间点：

```bash
./scripts/dev-vocechat.sh down
./scripts/vocechat-state.sh backup /safe/path/vocechat-$(date +%F).tar.gz
```

恢复会先把当前状态移动到带时间戳的 `before-restore` 目录，不会直接删除：

```bash
./scripts/dev-vocechat.sh down
./scripts/vocechat-state.sh restore /safe/path/vocechat-2026-08-21.tar.gz
./scripts/dev-vocechat.sh up
```

生产环境需要在同一维护窗口备份 `vocechat_data`、`vocechat_secrets`、`vocechat_init_state` 三个卷以及 DEEIX 的 PostgreSQL/SQLite 数据库。VoceChat 保存消息正文，DEEIX 数据库保存身份绑定、未读、会话偏好和搜索索引；只恢复其中一侧会产生状态不一致。

### 升级检查

先在测试环境启动候选 VoceChat 镜像，然后检查 DEEIX 依赖的 OpenAPI 契约：

```bash
./scripts/check-vocechat-upgrade.sh http://127.0.0.1:3001
```

契约通过后仍需用两个 DEEIX 测试用户验收文本、实时未读、回复、编辑、撤回、图片/文件和历史分页，再升级生产镜像。不要直接用生产服务做带写入的兼容性测试。

## 数据保留边界

- 保留天数大于 0 时，DEEIX 每 6 小时扫描一批过期消息，并以原发送者身份调用 VoceChat 撤回，然后把本地索引标记为已删除；失败项保留等待下次重试。
- VoceChat 的消息撤回不等同于立即擦除底层文件对象；需要物理销毁文件时，应结合 VoceChat 版本提供的资源清理能力和备份保留策略执行。
- 消息搜索只覆盖已经进入 DEEIX 本地索引的消息；打开历史页会补齐相应索引。

## 验收清单

1. 两个普通用户互发消息，验证离线未读、按钮角标、最近会话和多设备已读同步。若前面有 CDN 或反代，确认实时事件能持续推送、附件上传不被截断、文件下载未命中公共缓存。
2. 分别验证文本回复、编辑、撤回，以及搜索结果定位到历史消息。
3. 上传图片和普通文件，确认预览/下载需要 DEEIX 登录，且浏览器网络请求中没有 VoceChat token 或文件路径。
4. 验证置顶、静音、通知策略、窗口拖动缩放和布局持久化。
5. 管理员禁用后入口消失；重新启用后历史与偏好仍在。检查运行状态和审计日志。
6. 执行一次停机备份、恢复和候选镜像契约检查。

## 运行边界

- DEEIX 用户目录是唯一可信来源；只有 `active` 用户可被选择和收发消息。
- 用户第一次打开聊天或成为消息接收方时，DEEIX 后端会自动创建或恢复 VoceChat 身份，并持久化绑定关系。
- VoceChat 暂时停止时，DEEIX 的 AI 聊天、登录与其他功能不受影响；站内消息保留窗口和草稿，显示连接提示并自动重连。管理员禁用功能时仍会关闭入口。
- 请在实际对外、超过免费限制或商业使用前确认 VoceChat 当前许可证和授权条件。

## 消息可靠性与交互

- 草稿、回复目标、编辑目标和待发送消息按联系人隔离，保存在当前页面会话的内存中。关闭浮窗或切换联系人不会串用草稿；刷新页面后不会恢复这些内存状态。
- 文本、编辑和附件均显示独立的发送中或失败状态。失败项可以重试或移除，不会覆盖之后输入的文字。网络中断可能使发送结果不确定，重试前应检查消息记录；后台索引修复不会重新发送消息。
- 中文输入法确认候选词和 `Shift+Enter` 不触发发送。正文上限为 4000 个 Unicode 字符。
- 只有页面可见、拥有焦点、当前会话打开且处于消息底部时才推进已读。后台到达的消息仍按通知和静音设置提醒；向上翻阅时通过“有新消息”返回底部。
- 最近会话支持继续分页加载。桌面窗口支持展开和还原；超过 120 条消息时启用虚拟列表，并在行高变化后继续跟随底部。
- 实时连接采用带随机抖动的指数退避，最大重试间隔为 30 秒；长时间没有数据时重新连接。重连后补齐当前会话的最新历史，正在向上翻阅时保留位置并提示返回最新消息。

## 索引恢复

发送、回复、编辑、撤回和附件发送之前，DEEIX 先在本地数据库写入 `internal_messaging_index_repairs` 恢复记录。无法写入恢复记录时，不执行消息变更；VoceChat 已经成功但本地索引失败时，保留恢复记录，不把已成功的发送误报为失败。

后台每 15 秒检查恢复任务。为避免与执行中的请求竞争，任务最早在创建两分钟后执行；失败后稍后重试。每次最多读取 10 页历史，并持久化分页进度，重启后可继续处理。修复通过读取历史补齐索引和未读状态，不重放发送操作。消息的 `last_event_mid` 防止旧编辑或重复事件覆盖较新的状态。

新增表和字段随现有数据库 schema migration 一起创建。管理页展示索引写入失败次数和待修复任务数；失败次数是当前进程计数，待修复任务来自持久化数据库。多设备收到相同实时事件时，同一 DEEIX 实例会合并重复索引工作。

## 修复验证（2026-09-08）

前端单元测试涵盖草稿隔离、失败后继续输入、原目标重试、输入法回车、已读条件、分页与取消、缓存、事件解析和重连退避。后端测试覆盖索引故障后重启恢复、不重复发送或累加未读、跨多页恢复、事件顺序保护、批量用户查询及流式附件转发。

```bash
pnpm --dir frontend test:internal-messaging
pnpm --dir frontend typecheck
cd backend
go test -race ./internal/application/internalmessaging ./internal/transport/http/internalmessaging ./internal/infra/integration/vocechat ./internal/infra/persistence/postgres/internalmessaging ./internal/infra/persistence/postgres/user
go test ./internal/infra/integration/vocechat -run '^$' -bench BenchmarkUploadFileStream -benchmem
```

另外使用 Chromium、实际窗口组件和 Tailwind 样式验证了草稿切换、中文输入事件、失败重试、后台已读与通知、索引恢复后的同游标已读对齐、翻阅时的新消息提醒、重连补历史、125 个会话分页、窗口展开及 390×844 移动端布局。测试接口和 Markdown 正文渲染使用隔离替身，没有连接生产服务。

该隔离页面一次注入 1000 条实时事件，本机最终测试约 368 毫秒后显示最新消息，挂载 19 个消息节点。流式上传基准中，1 MiB 和 20 MiB 文件每次分配均约 21 KB。前者反映前端事件与虚拟列表表现，后者反映客户端转发的内存分配；均不代表真实 VoceChat、数据库和网络组成的生产环境容量。
