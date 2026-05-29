# DEEIX Chat Docker 全量安装

本文件供 coding agent 使用。不要一开始就 clone 或执行 compose。必须先做执行前环境预检，确认服务器区域、GitHub 连通性、Docker 可用性和镜像拉取能力；否则第一步就可能卡在 GitHub clone、`ghcr.io` 或 `docker.io` 镜像拉取。预检通过后，如果 DEEIX Chat 仓库尚未 clone 或当前目录不是仓库根目录，非中国大陆服务器 clone `https://github.com/DEEIX-AI/DEEIX-Chat.git`，中国大陆服务器直接 clone `https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git`，然后从仓库根目录继续执行。

## 目标

在用户机器上用风险最低、步骤最少的方式准备并启动 DEEIX Chat 的 Docker 全量环境。

默认路径：Docker 全量安装。

全量安装会启动三个容器：

- `app`：DEEIX Chat 应用，包含后端 API 和已构建的前端静态文件。
- `postgres`：内置 PostgreSQL + pgvector。
- `redis`：内置 Redis。

不要假设生产密钥、公网域名、上游模型 API Key 已经存在。能安全自动完成的步骤先完成，涉及密钥、域名、公网暴露和真实模型配置时停下来说明需要用户提供什么。

## 操作规则

- 保持幂等。重复执行本文档不应破坏已有数据或用户配置。
- 优先使用仓库已有文件和命令，不要发明新的部署结构。
- 不要使用 `sudo` 或安装系统软件，除非用户明确批准。
- 不要覆盖已有 `config.yaml`，除非用户明确要求。
- 不要删除 Docker volume，除非用户明确要求重置数据。
- 不要把本地 demo 配置、密钥或临时 compose override 提交到 Git。
- 如果某一步失败，停止并说明阻塞原因，给出最小下一步操作。
- 如果 Docker 不可用或 daemon 不可达，停止并提示用户先安装/启动 Docker。
- 必须检查 Docker Engine 版本和 Compose 命令形式。优先使用 Compose v2：`docker compose`；如果只有旧版 `docker-compose`，需要先确认它是否支持本项目 compose 文件语法。
- 必须先判断服务器是否在中国大陆。中国大陆服务器默认使用 GitHub clone 加速、华为云依赖镜像源和 GHCR 加速，否则 clone 仓库、拉取 `ghcr.io/deeix-ai/deeix-chat:latest`、`docker.io`、`pgvector/pgvector` 等资源可能失败或非常慢。
- 修改 `/etc/docker/daemon.json`、重启 Docker daemon、修改全局 Git 配置、安装系统软件都需要用户明确批准。
- 第一次 Docker 启动或构建可能很慢。必须区分是在拉镜像、安装前端依赖、下载 Go modules、执行容器内 apt-get，还是 Next.js 构建本身耗时。宿主机可以是 Ubuntu、Debian、CentOS、Rocky、AlmaLinux、Fedora、openSUSE、Arch 等 Linux 发行版，不要假设宿主机一定是 Debian。
- 全量部署启动成功后，必须询问用户最终访问方式：IP 访问或域名访问。域名访问需要提供反向代理方案。
- 首次启动会在日志中输出初始化管理员账户和密码，通常只出现一次。启动成功后必须立即读取日志，提取账户密码并输出给用户；不要写入 Git、文档或长期保存。

## 成功标准

认为安装成功需要同时满足：

- 当前目录是 DEEIX Chat 仓库根目录。
- 存在 `Dockerfile`、`docker-compose.full.yml`、`config.docker.example.yaml`。
- `config.yaml` 存在，且没有被误创建成目录。
- 已明确服务器网络区域：中国大陆或非中国大陆。
- Docker daemon 可用。
- 已确认 Docker Engine 版本、Compose v2 或旧版 `docker-compose` 是否可用，并选择了后续实际命令。
- 如果是中国大陆服务器，已使用 `https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git` 作为 GitHub clone 加速地址，或已明确用户提供的其它可用方案。
- 如果是中国大陆服务器，已检查华为云依赖镜像源和 GHCR 加速镜像拉取能力；需要配置 Docker Hub 加速时已在用户批准后完成。
- 如果需要本地构建镜像，已检查 pnpm、Go modules、容器内 apt 源等构建期依赖下载是否需要加速。
- 已按选定 Compose 命令启动成功；安装部署默认使用预构建镜像，并通过 `pull` + `up -d --no-build` 避免误触发本地构建。
- `app`、`postgres`、`redis` 容器处于运行状态。
- 已读取 `deeix-chat-app` 启动日志，提取并回显首次初始化管理员账户和密码；如果日志中没有找到，已明确说明原因和下一步。
- `http://127.0.0.1:8080` 返回 HTTP 200 或可访问的应用页面。
- 已明确最终访问方式：IP 访问或域名访问。
- 如果选择域名访问，已给出反向代理方案和需要修改的 DEEIX 配置项。
- 用户收到准确的访问地址和剩余配置事项。

## 步骤

1. 执行前环境预检。

   在 clone、pull、build、compose 之前先做预检。不要把 `git clone` 当作第一步，因为中国大陆服务器可能会卡在 GitHub；也不要直接 `docker compose up`，因为可能会卡在 `ghcr.io`、`docker.io` 或 `pgvector/pgvector` 镜像拉取。

   先确认基础命令是否存在，并记录 Docker/Compose 版本：

   ```bash
   command -v git || true
   command -v curl || true
   command -v docker || true
   docker --version || true
   docker version || true
   docker compose version || true
   command -v docker-compose || true
   docker-compose version || true
   ```

   如果 `git`、`curl`、`docker` 缺失，停止并告诉用户缺什么。安装系统软件需要用户明确批准。

   Compose 命令选择规则：

   - 优先使用 `docker compose`，即 Docker Compose v2 插件。
   - 如果 `docker compose version` 可用，后续所有命令使用 `docker compose ...`。
   - 如果 `docker compose` 不可用但 `docker-compose version` 可用，先记录后续可能使用 `docker-compose ...`。不要在仓库 clone 前执行 `docker-compose -f docker-compose.full.yml config`，因为 compose 文件可能还不存在；进入仓库后再验证兼容性。
   - 如果两者都不可用，停止并提示用户安装 Docker Compose v2。
   - 本文档的可执行部署命令统一使用 `$COMPOSE` 表示前面选出的 Compose 命令。使用旧版 `docker-compose` 时，只替换命令前缀，不改变 compose 文件和参数。进入仓库后用 `$COMPOSE -f docker-compose.full.yml config` 验证。

   如果仓库已经存在，也仍然先做网络和 Docker 预检，再进入仓库目录继续。

2. 判断服务器网络区域。

   必须先判断服务器是在中国大陆还是非中国大陆。不要只凭用户语言判断。

   优先使用用户提供的云厂商、机房地域或服务器公网 IP 信息。如果用户没有提供，使用以下命令辅助判断：

   ```bash
   curl -fsSL https://ipinfo.io/country || true
   curl -fsSL https://ipapi.co/country/ || true
   curl -fsSL https://ifconfig.co/country-iso || true
   ```

   判断规则：

   - 返回 `CN`，或云厂商地域是中国大陆，按中国大陆服务器处理。
   - 返回非 `CN`，或明确是香港、台湾、新加坡、日本、美国、欧洲等，按非中国大陆服务器处理。
   - 如果三个接口都不可用或结果冲突，不要猜测；询问用户服务器所在地区。

3. 检查 GitHub 连通性和加速需求。

   先按服务器区域选择 GitHub 检查方式。非中国大陆服务器检查官方 GitHub：

   ```bash
   git ls-remote https://github.com/DEEIX-AI/DEEIX-Chat.git HEAD
   curl -I --connect-timeout 10 --max-time 20 https://github.com
   curl -I --connect-timeout 10 --max-time 20 https://raw.githubusercontent.com
   ```

   如果服务器在非中国大陆且以上命令可用，继续下一步。

   如果服务器在中国大陆，不要先卡官方 GitHub；GitHub clone 默认使用用户指定的加速地址：

   ```bash
   git ls-remote https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git HEAD
   curl -I --connect-timeout 10 --max-time 20 https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git
   ```

   如果服务器在非中国大陆但以上命令超时、连接失败、TLS 失败、速度明显不可用，先向用户说明 GitHub 访问可能需要加速，并请求用户选择或提供可用方案。不要擅自把未知第三方镜像写入脚本或全局配置。

   可接受的 GitHub 加速方式包括：

   - 用户提供的 HTTP/HTTPS 代理，并临时用于当前命令。
   - 用户提供的企业代理、内网 GitHub 镜像或云厂商代码镜像。
   - 用户明确认可的 GitHub clone/raw 加速前缀。
   - 用户已经配置好的全局 Git proxy 或系统代理。

   使用代理时优先采用一次性环境变量，不要默认写入全局 Git 配置：

   ```bash
   HTTPS_PROXY=http://<proxy-host>:<proxy-port> git clone https://github.com/DEEIX-AI/DEEIX-Chat.git
   HTTPS_PROXY=http://<proxy-host>:<proxy-port> curl -I --connect-timeout 10 --max-time 20 https://raw.githubusercontent.com
   ```

   只有用户明确要求持久化代理时，才可以修改全局 Git 配置，并在最终回复中说明：

   ```bash
   git config --global http.proxy http://<proxy-host>:<proxy-port>
   git config --global https.proxy http://<proxy-host>:<proxy-port>
   ```

   如果 GitHub 或 `gh-proxy.org` 仍不可用，停止并说明需要用户提供可用代理、镜像地址，或手动上传源码包。

4. 检查 Docker Engine 和 Compose。

   ```bash
   docker --version
   docker version
   docker info
   docker compose version || true
   docker-compose version || true
   ```

   如果 `docker info` 失败，停止。告诉用户需要先启动 Docker daemon。

   选择 Compose 命令。此时仓库可能还没有 clone，所以不要在这里执行 `-f docker-compose.full.yml config`：

   ```bash
   if docker compose version >/dev/null 2>&1; then
     COMPOSE="docker compose"
   elif command -v docker-compose >/dev/null 2>&1; then
     COMPOSE="docker-compose"
   else
     echo "Docker Compose is not available. Install Docker Compose v2."
     exit 1
   fi
   printf 'COMPOSE=%s\n' "$COMPOSE"
   ```

   Compose 文件兼容性验证必须放到仓库 clone 之后执行。旧版 `docker-compose` 可能不支持部分 Compose v2 语法或 override 行为，应优先安装 Docker Compose v2。

   如需判断宿主机发行版，只用于选择 Docker/Compose 安装或 Docker 重启方式，不用于改变 DEEIX 部署步骤：

   ```bash
   cat /etc/os-release || true
   uname -a
   ```

5. 中国大陆服务器检查华为云依赖镜像源和 GHCR 镜像加速。

   DEEIX 全量部署即使不本地构建，也会拉取官方预构建镜像 `ghcr.io/deeix-ai/deeix-chat:latest`。中国大陆服务器如果不能稳定访问 GHCR，官方镜像也会很慢或失败。不要只检查依赖镜像源。

   中国大陆服务器默认使用 `ghcr.1ms.run` 加速 GHCR 镜像。也就是把官方镜像：

   ```text
   ghcr.io/deeix-ai/deeix-chat:latest
   ```

   替换为：

   ```text
   ghcr.1ms.run/deeix-ai/deeix-chat:latest
   ```

   如果服务器位于中国大陆，可以记录 Docker daemon 是否已有 registry mirrors，但标准国内安装不依赖 Docker Hub mirror：

   ```bash
   docker info | sed -n '/Registry Mirrors:/,/Live Restore Enabled:/p'
   ```

   标准国内安装优先使用华为云同步源拉取 `pgvector/pgvector:pg16` 和 `redis:7`，再 `docker tag` 回 compose 使用的原始镜像名。这样不需要依赖 Docker Hub mirror，也能让 compose 文件继续使用原始镜像名 `pgvector/pgvector:pg16`、`redis:7`。

   如果已有可用镜像加速地址，继续做实际镜像拉取验证。不要只看 `docker info`，必须拉取 DEEIX 实际需要的镜像。中国大陆环境优先用华为云同步源验证 `pgvector` 和 `redis`，官方应用镜像仍通过 GHCR 加速源验证：

   ```bash
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
   docker pull ghcr.1ms.run/deeix-ai/deeix-chat:latest
   ```

   `docker.1ms.run` 是 Docker Hub 加速域名，可作为备选排查手段，但不要把它作为国内 `pgvector`/`redis` 的默认路径。如果要临时验证 `docker.1ms.run`，可以测试：

   ```bash
   docker pull docker.1ms.run/pgvector/pgvector:pg16
   docker pull docker.1ms.run/library/redis:7
   ```

   只有在必须访问 Docker Hub、且华为云同步源或离线镜像不可用时，才考虑配置 Docker Hub mirror。修改 `/etc/docker/daemon.json` 前必须请求用户批准，不要擅自覆盖已有配置。

   Docker Hub 加速可先使用 `https://docker.1ms.run`。如果用户允许覆盖 Docker daemon 配置，可以写入以下配置：

   ```bash
   echo '{"registry-mirrors":["https://docker.1ms.run"],"dns":["8.8.8.8","114.114.114.114"]}' | sudo tee /etc/docker/daemon.json > /dev/null
   ```

   这会覆盖已有 `/etc/docker/daemon.json`。覆盖前如果文件已存在，必须先提示用户并建议备份：

   ```bash
   sudo cp /etc/docker/daemon.json /etc/docker/daemon.json.bak.$(date +%Y%m%d%H%M%S)
   ```

   注意：Docker `registry-mirrors` 主要影响 Docker Hub，不一定代理 `ghcr.io`。中国大陆服务器默认用 `ghcr.1ms.run/deeix-ai/deeix-chat:latest` 拉取 DEEIX 官方镜像。如果 `ghcr.1ms.run` 不可用，再让用户提供其它支持 GHCR 的代理、镜像同步地址、企业镜像仓库，或改用本地构建。

   如果临时手动测试 `docker.1ms.run` 加速域名，必须使用正确的 Docker Hub 命名空间：

   - Docker Hub 官方镜像才使用 `library/`，例如 `docker.1ms.run/library/redis:7`。
   - 非官方镜像必须保留命名空间，例如 `pgvector/pgvector:pg16` 对应 `docker.1ms.run/pgvector/pgvector:pg16`，不是 `docker.1ms.run/library/pgvector:pg16`。
   - 如果出现 `manifest unknown`、`not found`，通常是路径错误或代理源没有该镜像。
   - 如果已经开始拉取 layer，但出现 `failed to copy`、`could not fetch content descriptor ... from remote: not found`，通常是该加速域名对这个镜像的缓存或同步不完整，不是等待更久能解决。应判定当前加速域名对这个镜像不可用，换 Docker Hub 加速源、改用 daemon mirror 直连结果，或使用企业镜像仓库。

   修改 Docker daemon 配置后才需要重启 Docker 并验证。Ubuntu/Debian 等 systemd 系统使用：

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart docker
   docker info | sed -n '/Registry Mirrors:/,/Live Restore Enabled:/p'
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
   docker pull ghcr.1ms.run/deeix-ai/deeix-chat:latest
   ```

   如果宿主机不使用 systemd，再按系统服务管理方式处理：

   ```bash
   if command -v service >/dev/null 2>&1; then
     sudo service docker restart
   else
     echo "Cannot restart Docker automatically; ask the user to restart Docker manually."
   fi
   ```

   如果 Docker Hub 官方镜像可以拉取，但 `pgvector/pgvector:pg16` 或 `redis:7` 仍无法拉取，说明 `https://docker.1ms.run` 对实际依赖镜像不可用或同步不完整，需要改用华为云同步源、检查 daemon mirror 是否生效，或让用户提供企业镜像仓库/离线镜像。

   特别注意 `pgvector/pgvector:pg16`：它是全量部署的 PostgreSQL 服务镜像，不同服务器拉取结果可能差异很大。某台服务器能直连拉取，不代表另一台服务器也能拉取。安装前必须单独验证它，不能只验证 `hello-world` 或 `redis:7`。中国大陆默认优先验证华为云同步源。

   如果 `pgvector/pgvector:pg16` 卡住超过几分钟，先停止继续 compose，按以下顺序处理：

   1. 检查 Docker daemon mirror 是否真的生效，并确认 `Registry Mirrors` 包含可用加速源。
   2. 重新验证华为云同步源上的 `pgvector/pgvector:pg16`。
   3. 如果加速域名返回 `failed to copy` 或 `content descriptor ... not found`，判定该源对 pgvector 当前缓存不完整。
   4. 如果用户有企业镜像仓库或云厂商镜像同步服务，建议把 `pgvector/pgvector:pg16` 同步到自己的仓库。
   5. 如果已有另一台机器成功拉取，可以使用 `docker save` / `docker load` 离线迁移镜像。

   离线迁移示例：

   ```bash
   # 在已经成功拉取的机器上
   docker save pgvector/pgvector:pg16 | gzip > pgvector-pg16.tar.gz
   docker save redis:7 | gzip > redis-7.tar.gz

   # 传到目标服务器后
   gunzip -c pgvector-pg16.tar.gz | docker load
   gunzip -c redis-7.tar.gz | docker load
   ```

   如果 Docker Hub 镜像可以拉取但 `ghcr.1ms.run/deeix-ai/deeix-chat:latest` 仍无法拉取，说明当前 GHCR 加速源不可用。停止并让用户提供其它支持 GHCR 的代理、企业内网镜像源、云厂商镜像同步地址，或确认是否改成本地构建。


6. 识别首次启动或构建时的慢点。

   如果只是运行官方镜像，使用预构建镜像路径，不要本地构建。以下命令只适用于非中国大陆服务器：

   ```bash
   $COMPOSE -f docker-compose.full.yml pull app postgres redis
   $COMPOSE -f docker-compose.full.yml up -d --no-build
   ```

   主要耗时通常是拉取镜像层：

   - 非中国大陆默认：`ghcr.io/deeix-ai/deeix-chat:latest`
   - 中国大陆默认：`ghcr.1ms.run/deeix-ai/deeix-chat:latest`
   - 华为云同步源：`swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16`
   - 华为云同步源：`swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7`

   中国大陆服务器重点检查华为云同步源和 `ghcr.1ms.run` 可达性。官方预构建镜像来自 GHCR，GHCR 慢时即使不本地构建也会卡住；中国大陆默认用 `ghcr.1ms.run` 加速。Docker Hub mirror 只作为华为云同步源不可用时的备选方案。

   如果使用本地源码构建：

   ```bash
   DEEIX_CHAT_IMAGE=deeix-chat:local $COMPOSE -f docker-compose.full.yml up -d --build
   ```

   首次构建还会下载或执行：

   - Docker 基础镜像：`node:24-bookworm-slim`、`golang:1.25-bookworm`、`debian:bookworm-slim`。
   - 前端依赖：`corepack enable` 后执行 `pnpm install --frozen-lockfile`。
   - 后端依赖：`go mod download`。
   - 容器内 Debian 包：Dockerfile 的 `runtime-deps` 阶段基于 `debian:bookworm-slim`，会执行 `apt-get update` 并安装 `ca-certificates`、`tzdata`。这是容器内操作，不代表宿主机必须是 Debian。
   - 前端构建：`pnpm build` / Next.js build，这一步即使网络正常也可能耗时较长。

   判断卡在哪一步要看 Docker build 输出，不要只看 `running`。常见阶段含义：

   - `load metadata for docker.io/...`：正在访问 Docker Hub 镜像元数据。若代理域名返回 `not found`，先检查镜像命名空间是否正确。
   - `RUN pnpm install --frozen-lockfile`：正在下载前端 npm/pnpm 包。
   - `RUN go mod download`：正在下载 Go modules。
   - `RUN apt-get update`：正在访问构建容器内的 Debian apt 源。
   - `RUN pnpm build`：正在编译前端，可能是 CPU/内存耗时，不一定是网络问题。

   中国大陆服务器如果构建期下载很慢，需要在用户批准后选择合适的加速方式：

   - Docker Hub 镜像：配置 Docker daemon `registry-mirrors`，只使用可用的 Docker Hub 加速源，并验证实际镜像可以拉取；不要只验证 `hello-world`。中国大陆优先采用华为云同步源拉取 `pgvector/pgvector:pg16`、`redis:7`。
   - GHCR 镜像：中国大陆默认使用 `ghcr.1ms.run`；如果不可用，再使用用户提供的代理、镜像同步地址或企业镜像仓库；不要假设 Docker Hub mirror 会代理 GHCR。
   - pnpm/npm：配置 npm registry，例如 `https://registry.npmmirror.com`。
   - Go modules：配置 `GOPROXY`，例如 `https://goproxy.cn,direct`。
   - 容器内 apt：使用就近 Debian 镜像源；这是 Dockerfile 构建阶段的源，不是宿主机包管理器。
   - GitHub raw/clone：使用用户提供的代理或镜像。

   重要：Docker Hub/GHCR 加速只解决镜像拉取慢，不会自动加速 `pnpm install`、`go mod download`、`apt-get update` 或 `pnpm build`。如果用户说“构建时 running 很久”，必须先判断是否正在本地构建。

   当前 Dockerfile 没有内置 npm、Go、容器内 apt 镜像源切换参数。因此本文档目前能避免官方镜像拉取慢，也能识别构建期慢点，但不能在不改 Dockerfile 的情况下自动修复 pnpm、Go modules、apt 源慢的问题。

   处理优先级：

   - 如果只是部署使用，优先不要本地构建，直接使用官方预构建镜像。由于 `docker-compose.full.yml` 同时包含 `image` 和 `build`，为避免镜像不存在时 Compose 触发本地构建，建议先 `pull`，再用 `up --no-build`。

     非中国大陆服务器：

     ```bash
     $COMPOSE -f docker-compose.full.yml pull app postgres redis
     $COMPOSE -f docker-compose.full.yml up -d --no-build
     ```

     中国大陆服务器使用 GHCR 加速镜像：

     ```bash
     docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
     docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
     docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
     docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
     DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml pull app
     DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml up -d --no-build
     ```

   - 如果必须本地构建，并且用户有 HTTP/HTTPS 代理，可优先使用 Docker build 代理。代理方式不需要改 Dockerfile：

     ```bash
     HTTP_PROXY=http://<proxy-host>:<proxy-port> \
     HTTPS_PROXY=http://<proxy-host>:<proxy-port> \
     DEEIX_CHAT_IMAGE=deeix-chat:local $COMPOSE -f docker-compose.full.yml up -d --build
     ```

   - 如果必须通过 npm registry、GOPROXY、apt mirror 加速，需要修改 Dockerfile 或增加 build args 支持；不要假装 registry-mirrors 能解决这些下载。只有用户明确要求时，才在单独改动中增加可配置的 build args，例如 npm registry、GOPROXY、Debian apt mirror。

   - `RUN pnpm build` 是编译步骤，不是下载步骤；即使所有网络加速都配置正确，也可能因为 CPU/内存较慢而持续很久。

7. 确认仓库根目录。

   预检通过后，如果当前目录不是 DEEIX Chat 仓库根目录，才 clone 仓库。非中国大陆服务器使用官方 GitHub：

   ```bash
   git clone https://github.com/DEEIX-AI/DEEIX-Chat.git
   cd DEEIX-Chat
   ```

   中国大陆服务器直接使用 `gh-proxy.org` 加速地址拉取源码：

   ```bash
   git clone https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git
   cd DEEIX-Chat
   ```

   如果 GitHub clone 或 `gh-proxy.org` clone 失败，不要继续；回到 GitHub 加速步骤，让用户提供代理、镜像或源码包。

   用以下文件确认目录正确：

   ```bash
   test -f Dockerfile && test -f docker-compose.full.yml && test -f config.docker.example.yaml
   ```

   进入仓库后再验证 Compose 文件兼容性：

   ```bash
   if docker compose version >/dev/null 2>&1; then
     COMPOSE="docker compose"
   else
     COMPOSE="docker-compose"
   fi
   $COMPOSE -f docker-compose.full.yml config >/tmp/deeix-compose-config.yml
   ```

   如果 `$COMPOSE -f docker-compose.full.yml config` 失败，停止并输出错误。

8. 准备配置文件。

   如果 `config.yaml` 不存在，复制模板：

   ```bash
   cp config.docker.example.yaml config.yaml
   ```

   如果 `config.yaml` 已存在，不要覆盖。

   如果 `config.yaml` 是目录，停止并说明这是错误状态。只有在确认目录为空且用户同意后，才删除该目录并重新复制配置文件。

9. 检查关键配置。

   `docker-compose.full.yml` 会通过环境变量覆盖 PostgreSQL 和 Redis 连接：

   - `POSTGRES_DSN=postgres://deeix_chat:deeix_chat_2026@postgres:5432/deeix_chat?...`
   - `REDIS_ADDR=redis:6379`
   - `REDIS_PASSWORD=deeix_chat_2026`

   因此全量安装通常不需要修改 `database.postgres.dsn` 和 `database.redis.*`。

   本机试用可以保留 `config.docker.example.yaml` 默认值。生产或公网部署必须提醒用户修改：

   - `app.env`：生产环境应为 `prod`。
   - `security.jwt_secret`：必须替换为强随机密钥。
   - `security.data_encryption_key`：必须替换为强随机密钥，至少 32 字节。
   - `server.public_api_base_url`：公网 API 地址。
   - `server.public_web_base_url`：公网 Web 地址。
   - `server.cors_allow_origin`：允许访问的前端 origin。

   不要读取 `.env` 或其它可能包含真实密钥的文件。

10. 启动 Docker 全量环境。

   默认只绑定本机地址 `127.0.0.1:8080`。

   如果目标是安装部署，优先使用预构建镜像，避免本地构建耗时。由于 compose 文件包含 `build` 字段，建议先拉取镜像，再用 `--no-build` 启动。拉取阶段如果 `pgvector/pgvector:pg16` 卡住，回到第 5 步处理，不要继续等待 `up`。

   非中国大陆服务器：

   ```bash
   $COMPOSE -f docker-compose.full.yml pull app postgres redis
   $COMPOSE -f docker-compose.full.yml up -d --no-build
   ```

   中国大陆服务器如果使用 `ghcr.1ms.run` 拉取官方镜像：

   ```bash
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
   DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml pull app
   DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml up -d --no-build
   ```

   如果用户明确要求从当前源码构建，才使用 `--build`，见后面的本地构建步骤。

   这会启动 `app`、`postgres`、`redis`，并保留 Docker volumes：

   - `deeix-chat-app-storage`
   - `deeix-chat-postgres-data`
   - `deeix-chat-redis-data`

11. 读取首次初始化账户密码。

   DEEIX Chat 首次初始化会在 `app` 容器日志中输出管理员账户和密码，通常只输出一次。全量启动成功后必须立即读取，并在最终回复中原样告诉用户。

   先查看最近启动日志：

   ```bash
   docker logs deeix-chat-app --tail 200
   ```

   可以用关键词辅助定位：

   ```bash
   docker logs deeix-chat-app 2>&1 | grep -i "bootstrap superadmin\|password\|username\|admin" | tail -80
   ```

   如果使用 compose 服务名，也可以执行：

   ```bash
   $COMPOSE -f docker-compose.full.yml logs app --tail 200
   ```

   处理规则：

   - 找到账户和密码后，必须在最终回复中输出给用户。
   - 明确提醒用户首次登录后立即修改密码。
   - 不要把账户密码写入仓库文件、Markdown 文档、commit message、PR 评论或公开 issue。
   - 如果没有找到，说明可能原因：已经初始化过、日志被轮转、容器重建但数据库 volume 保留、应用没有输出初始化凭据。
   - 如果用户需要重置管理员密码，不要自行删除数据库 volume；先询问用户是否允许执行重置流程。

12. 如果用户明确要求公网 IP 访问。

   默认 compose 只暴露到 `127.0.0.1:8080`。不要直接修改已跟踪的 `docker-compose.full.yml`。

   可以创建一个本地 override 文件，例如 `docker-compose.ip.yml`：

   ```yaml
   services:
     app:
       ports: !override
         - "8080:8080"
   ```

   然后启动。仍然优先使用预构建镜像，避免触发本地构建：

   ```bash
   $COMPOSE -f docker-compose.full.yml -f docker-compose.ip.yml up -d --no-build
   ```

   中国大陆服务器使用 GHCR 加速镜像时：

   ```bash
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
   DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml -f docker-compose.ip.yml pull app
   DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml -f docker-compose.ip.yml up -d --no-build
   ```

   同时提醒用户：这个 override 文件通常是本地 demo 配置，不应提交到 PR 或公共仓库。

13. 如果需要用当前源码本地构建镜像。

   默认 compose 使用 `ghcr.io/deeix-ai/deeix-chat:latest`。中国大陆服务器可通过 `DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest` 使用 GHCR 加速镜像。

   注意：只有使用 `--build` 时才会进入本地构建。本地构建慢通常不是 GHCR 没加速，而是 Dockerfile 内部的 `pnpm install`、`go mod download`、`apt-get update` 或 `pnpm build` 慢。

   如果用户要求测试当前工作区代码，使用本地镜像名：

   ```bash
   DEEIX_CHAT_IMAGE=deeix-chat:local $COMPOSE -f docker-compose.full.yml up -d --build
   ```

   如果同时需要公网 IP 访问：

   ```bash
   DEEIX_CHAT_IMAGE=deeix-chat:local $COMPOSE -f docker-compose.full.yml -f docker-compose.ip.yml up -d --build
   ```

   如果构建期下载慢且用户提供了代理，可以临时加代理环境变量：

   ```bash
   HTTP_PROXY=http://<proxy-host>:<proxy-port> \
   HTTPS_PROXY=http://<proxy-host>:<proxy-port> \
   DEEIX_CHAT_IMAGE=deeix-chat:local $COMPOSE -f docker-compose.full.yml up -d --build
   ```

   如果慢在 `RUN pnpm build`，这通常是编译耗时，不是镜像加速问题。

14. 部署完成后确认访问方式。

   Docker 全量环境启动并验证通过后，必须询问用户选择哪种访问方式：

   - IP 访问：适合临时测试或内网访问。
   - 域名访问：适合正式使用，需要配置 DNS、反向代理、公开 URL 和 CORS。

   如果用户选择 IP 访问：

   - 默认 compose 只监听 `127.0.0.1:8080`，只能本机访问。
   - 如果需要从外部访问服务器 IP，使用本地 override 暴露 `8080:8080`，不要修改已跟踪的 `docker-compose.full.yml`。
   - 访问地址通常是 `http://<server-ip>:8080`。
   - 如果后端配置了 `server.public_api_base_url`、`server.public_web_base_url`、`server.cors_allow_origin`，确保它们包含该 IP 地址。

   如果用户选择域名访问，先收集：

   - Web 域名，例如 `chat.example.com`。
   - 是否单域名部署。如果前后端同源，Web 和 API 都走同一个域名。
   - 是否已有 Nginx、Caddy、Traefik、宝塔、1Panel、云厂商负载均衡或 CDN。
   - 是否已经有 TLS 证书，或是否需要反向代理自动申请 HTTPS。

   单域名反向代理推荐拓扑：

   ```text
   https://chat.example.com -> reverse proxy -> http://127.0.0.1:8080
   ```

   此时 `docker-compose.full.yml` 可以继续只绑定 `127.0.0.1:8080:8080`，反向代理与应用在同一台机器时不需要把应用端口暴露到公网。

   域名访问时需要修改 `config.yaml`：

   ```yaml
   server:
     public_api_base_url: "https://chat.example.com"
     public_web_base_url: "https://chat.example.com"
     cors_allow_origin: "https://chat.example.com"
     trusted_proxies: "127.0.0.1/32,::1/128"
   ```

   修改后重启 app。仍然优先使用预构建镜像，避免触发本地构建：

   ```bash
   $COMPOSE -f docker-compose.full.yml up -d --no-build
   ```

   中国大陆服务器使用 GHCR 加速镜像时：

   ```bash
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
   docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
   docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
   DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml pull app
   DEEIX_CHAT_IMAGE=ghcr.1ms.run/deeix-ai/deeix-chat:latest $COMPOSE -f docker-compose.full.yml up -d --no-build
   ```

   Nginx 反向代理示例：

   ```nginx
   server {
     listen 80;
     server_name chat.example.com;

     client_max_body_size 100m;

     location / {
       proxy_pass http://127.0.0.1:8080;
       proxy_http_version 1.1;
       proxy_set_header Host $host;
       proxy_set_header X-Real-IP $remote_addr;
       proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
       proxy_set_header X-Forwarded-Proto $scheme;
       proxy_set_header Upgrade $http_upgrade;
       proxy_set_header Connection "upgrade";
       proxy_read_timeout 300s;
       proxy_send_timeout 300s;
     }
   }
   ```

   Caddy 反向代理示例：

   ```caddy
   chat.example.com {
     reverse_proxy 127.0.0.1:8080
   }
   ```

   如果用户使用 CDN 或云负载均衡，提醒用户：

   - `/api/*`、`/healthz`、`/readyz`、`/swagger/*` 不应缓存。
   - WebSocket、流式响应和长连接需要完整转发。
   - 需要转发 `Host`、`X-Forwarded-Proto`、`X-Forwarded-For`。
   - 如果 CDN 开启 HTTPS 回源，源站协议和证书要匹配。

   域名访问验证：

   ```bash
   curl -I https://chat.example.com
   curl -I https://chat.example.com/healthz
   docker logs deeix-chat-app --tail 100
   ```

   如果域名访问异常，优先检查 DNS 解析、反向代理端口、TLS 证书、`public_api_base_url`、`public_web_base_url`、`cors_allow_origin` 和 `trusted_proxies`。

## 验证

执行任何 clone、pull、build、compose 前，先确认服务器网络区域、Docker Engine 版本、Compose v2/旧版 docker-compose 可用性、GitHub 连通性、依赖镜像和 GHCR 镜像拉取能力，以及构建期依赖下载是否可能需要加速：

```bash
docker --version || true
docker compose version || true
docker-compose version || true
curl -fsSL https://ipinfo.io/country || true
docker info | sed -n '/Registry Mirrors:/,/Live Restore Enabled:/p'
# 非中国大陆服务器：
git ls-remote https://github.com/DEEIX-AI/DEEIX-Chat.git HEAD
curl -I --connect-timeout 10 --max-time 20 https://raw.githubusercontent.com
docker pull hello-world
docker pull pgvector/pgvector:pg16
docker pull redis:7
docker pull ghcr.io/deeix-ai/deeix-chat:latest
# 中国大陆服务器默认使用：
git ls-remote https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git HEAD
curl -I --connect-timeout 10 --max-time 20 https://gh-proxy.org/https://github.com/DEEIX-AI/DEEIX-Chat.git
docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16
docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/pgvector/pgvector:pg16 pgvector/pgvector:pg16
docker pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7
docker tag swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/redis:7 redis:7
docker pull ghcr.1ms.run/deeix-ai/deeix-chat:latest
```

如果需要本地构建镜像，再按需验证构建期网络：

```bash
docker run --rm node:24-bookworm-slim node --version
docker run --rm golang:1.25-bookworm go env GOPROXY
docker run --rm debian:bookworm-slim sh -lc 'apt-get update'
```

然后执行应用验证。全量部署验证通过后，必须读取首次初始化账户密码日志，并确认用户选择 IP 访问还是域名访问；域名访问需要继续验证反向代理。若前面选择的是旧版 `docker-compose`，把下面的 `docker compose` 替换为 `docker-compose`：

```bash
docker compose -f docker-compose.full.yml ps
docker logs deeix-chat-app --tail 200
docker logs deeix-chat-app 2>&1 | grep -i "bootstrap superadmin\|password\|username\|admin" | tail -80
curl -I http://127.0.0.1:8080
```

如果使用了公网端口 override，也验证公网 IP 地址：

```bash
curl -I http://<server-ip>:8080
```

如果配置了域名和反向代理，也验证域名地址：

```bash
curl -I https://<domain>
curl -I https://<domain>/healthz
```

查看日志：

```bash
docker logs deeix-chat-app --tail 100
```

配置挂载检查。若前面选择的是旧版 `docker-compose`，把 `docker compose` 替换为 `docker-compose`：

```bash
docker compose -f docker-compose.full.yml exec app ls -l /app/config.yaml
```

如果 `app` 容器退出并提示 `JWT_SECRET must be explicitly set`，优先检查：

- `config.yaml` 是否存在。
- `config.yaml` 是否被误创建成目录。
- `config.yaml` 是否正确挂载到 `/app/config.yaml`。
- `security.jwt_secret` 是否为空或仍不满足当前环境要求。

## 最终回复格式

完成后给用户一个简短状态报告，包含：

1. 执行前预检：Docker Engine 版本、Compose 命令形式、GitHub 连通性、Docker daemon、依赖镜像和 GHCR 镜像拉取能力是否通过。
2. 安装路径：Docker 全量安装。
3. 服务器区域：中国大陆或非中国大陆；如果是中国大陆，说明 GitHub clone 加速、华为云依赖镜像源和 GHCR 加速是否可用。
4. GitHub 连通性：是否可直接访问，或使用了哪种用户批准的加速方式。
5. 首次启动/构建耗时判断：说明慢点是在 app/GHCR、pgvector、redis 镜像拉取，还是 pnpm、Go modules、容器内 apt、Next.js build；不要把容器内 Debian apt 误判为宿主机系统要求。
6. 启动状态：哪些容器已运行。
7. 初始化管理员凭据：从日志中读取到的账户和密码；如果未找到，说明原因和下一步。
8. 访问方式：IP 访问或域名访问；如果是域名访问，说明反向代理方案。
9. 访问地址：本机地址、公网 IP 地址或域名地址。
10. 创建或检测到的文件：例如 `config.yaml`，如有本地 override 也说明。
11. 剩余用户动作：首次登录后修改密码、生产密钥、域名/CORS、上游模型 API Key 等。
12. 如果失败：失败命令、核心错误、最小下一步。

## 立即执行边界

按本文档完成 Docker 全量安装和基础验证后停止。除非用户明确要求，不要继续改代码、提交 Git、删除数据卷、配置真实模型密钥或进行生产上线操作。
