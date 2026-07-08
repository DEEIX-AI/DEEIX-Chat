# 🚀 部署到线上指南

## 📋 部署前检查清单

### 1. 代码提交状态
- [x] 代码已提交到 dev 分支
- [ ] 代码已合并到 main/master 分支（如果需要）
- [ ] 已推送到 GitHub

### 2. 功能完整性
- [x] 修复上游模型空行问题
- [x] 一键快速配置模型价格
- [x] 品牌设置页面（前端）
- [ ] 品牌设置后端 API（需要后端开发）

---

## 🔧 部署步骤

### 方法 1: Docker Compose 部署（推荐）

#### 步骤 1: 推送代码到 GitHub

```bash
# 推送到远程仓库
git push origin dev

# 如果需要合并到主分支
git checkout main
git merge dev
git push origin main
```

#### 步骤 2: 在服务器上拉取最新代码

```bash
# SSH 登录到服务器
ssh user@your-server.com

# 进入项目目录
cd /path/to/DEEIX-Chat

# 拉取最新代码
git pull origin dev  # 或 main

# 如果有子模块
git submodule update --init --recursive
```

#### 步骤 3: 构建 Docker 镜像

```bash
# 构建镜像
docker build -t deeix-chat:latest .

# 或者使用 docker-compose 构建
docker-compose build
```

#### 步骤 4: 停止旧容器并启动新容器

```bash
# 停止旧容器
docker-compose down

# 启动新容器
docker-compose up -d

# 查看日志
docker-compose logs -f app
```

#### 步骤 5: 验证部署

```bash
# 检查容器状态
docker-compose ps

# 检查应用健康状态
curl http://localhost:8080/health

# 或访问浏览器
# http://your-domain.com
```

---

### 方法 2: 使用 GitHub Actions 自动部署

#### 步骤 1: 创建 GitHub Actions 工作流

创建文件 `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Production

on:
  push:
    branches:
      - main  # 或 dev，根据你的主分支

jobs:
  deploy:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Build and push Docker image
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: |
            ghcr.io/deeix-ai/deeix-chat:latest
            ghcr.io/deeix-ai/deeix-chat:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
      
      - name: Deploy to server
        uses: appleboy/ssh-action@master
        with:
          host: ${{ secrets.SERVER_HOST }}
          username: ${{ secrets.SERVER_USER }}
          key: ${{ secrets.SERVER_SSH_KEY }}
          script: |
            cd /path/to/DEEIX-Chat
            docker-compose pull
            docker-compose up -d
            docker-compose logs --tail=50
```

#### 步骤 2: 配置 GitHub Secrets

在 GitHub 仓库设置中添加：
- `SERVER_HOST`: 服务器 IP 或域名
- `SERVER_USER`: SSH 用户名
- `SERVER_SSH_KEY`: SSH 私钥

#### 步骤 3: 推送代码触发部署

```bash
git push origin main
```

---

### 方法 3: 手动构建部署

#### 步骤 1: 构建前端

```bash
cd frontend
pnpm install
pnpm build
```

#### 步骤 2: 构建后端

```bash
cd backend
go mod download
go build -o deeix-chat ./cmd/server
```

#### 步骤 3: 部署到服务器

```bash
# 打包文件
tar -czf deploy.tar.gz \
  backend/deeix-chat \
  frontend/.next \
  frontend/public \
  config.yaml

# 上传到服务器
scp deploy.tar.gz user@server:/path/to/deploy/

# SSH 到服务器解压
ssh user@server
cd /path/to/deploy
tar -xzf deploy.tar.gz

# 重启服务
systemctl restart deeix-chat
```

---

## 🔄 零停机部署（推荐生产环境）

### 使用 Docker Compose 滚动更新

```bash
# 创建 docker-compose.override.yml
services:
  app:
    deploy:
      replicas: 2
      update_config:
        parallelism: 1
        delay: 10s
      restart_policy:
        condition: on-failure

# 滚动更新
docker-compose up -d --no-deps --scale app=2 app
```

### 使用 Nginx 作为反向代理

```nginx
upstream deeix-chat {
    least_conn;
    server 127.0.0.1:8080;
    server 127.0.0.1:8081 backup;
}

server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://deeix-chat;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 📊 部署后验证

### 1. 检查功能

```bash
# 检查模型价格快速配置
# 1. 登录管理后台
# 2. 进入计费设置 -> 模型定价
# 3. 点击闪电图标 ⚡ 按钮
# 4. 验证批量配置功能

# 检查上游模型同步
# 1. 进入管理后台 -> 上游管理
# 2. 点击获取模型
# 3. 验证不再显示空行

# 检查品牌设置（如果后端已实现）
# 1. 进入管理后台 -> 品牌设置
# 2. 上传图标和 Logo
# 3. 保存并验证生效
```

### 2. 检查日志

```bash
# Docker 日志
docker-compose logs -f app

# 查找错误
docker-compose logs app | grep -i error

# 查找警告
docker-compose logs app | grep -i warning
```

### 3. 性能检查

```bash
# 检查响应时间
curl -w "@curl-format.txt" -o /dev/null -s http://your-domain.com

# 检查内存使用
docker stats deeix-chat-app

# 检查数据库连接
docker-compose exec app /app/deeix-chat health
```

---

## 🔧 故障排查

### 问题 1: 容器启动失败

```bash
# 查看详细日志
docker-compose logs app

# 检查配置文件
docker-compose config

# 验证端口未被占用
netstat -tulpn | grep 8080
```

### 问题 2: 前端资源加载失败

```bash
# 清除前端缓存
docker-compose exec app rm -rf /app/frontend/.next/cache

# 重新构建前端
docker-compose build --no-cache app
```

### 问题 3: 数据库连接失败

```bash
# 检查数据库连接
docker-compose exec app psql -h db -U postgres -c "SELECT 1"

# 检查网络连接
docker network inspect deeix-chat-network
```

---

## 📝 回滚步骤

如果部署出现问题，可以快速回滚：

### Docker 回滚

```bash
# 查看历史镜像
docker images deeix-chat

# 回滚到上一个版本
docker tag deeix-chat:previous deeix-chat:latest
docker-compose up -d

# 或使用 Git 回滚
git checkout <previous-commit>
docker-compose build
docker-compose up -d
```

### Git 回滚

```bash
# 回滚到上一个提交
git revert HEAD
git push origin dev

# 或硬重置（谨慎使用）
git reset --hard HEAD~1
git push -f origin dev
```

---

## 🚦 环境配置建议

### 生产环境 config.yaml

```yaml
server:
  port: 8080
  mode: production  # 生产模式
  
database:
  type: postgres
  host: db
  port: 5432
  database: deeix_chat
  
cache:
  type: redis
  host: redis
  port: 6379
  
storage:
  type: s3  # 生产环境建议使用 S3
  bucket: your-bucket
  
logging:
  level: info  # 生产环境使用 info
  output: /app/logs/app.log
```

---

## 📊 监控和告警

### 1. 添加健康检查

```yaml
# docker-compose.yml
services:
  app:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

### 2. 配置日志收集

```bash
# 使用 rsyslog
docker-compose logs app | logger -t deeix-chat

# 或使用 ELK Stack
# 配置 Filebeat 收集日志
```

### 3. 配置监控告警

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'deeix-chat'
    static_configs:
      - targets: ['app:8080']
```

---

## ✅ 部署完成检查清单

- [ ] 代码已推送到远程仓库
- [ ] Docker 镜像构建成功
- [ ] 容器启动成功
- [ ] 应用可正常访问
- [ ] 管理后台可登录
- [ ] 新功能正常工作
  - [ ] 模型价格快速配置
  - [ ] 上游模型同步无空行
  - [ ] 品牌设置（如果已实现）
- [ ] 日志无错误
- [ ] 性能正常
- [ ] 数据库连接正常
- [ ] 备份已完成

---

## 📞 需要帮助？

如果部署过程中遇到问题：

1. 查看日志: `docker-compose logs -f app`
2. 检查 GitHub Issues
3. 联系团队技术支持
4. 查阅 [DEEIX Chat 文档](https://deeix.com/docs)

---

**最后更新**: 2026-07-09  
**版本**: v1.0.0  
**作者**: Claude Opus 4.8
