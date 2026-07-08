# 🚀 快速部署指南

## ⚠️ Git 权限问题解决

如果遇到 `Permission denied` 错误，请选择以下方法之一：

### 方法 1: 使用 SSH (推荐)

```bash
# 配置 SSH URL
git remote set-url origin git@github.com:DEEIX-AI/DEEIX-Chat.git

# 推送代码
git push origin dev
```

### 方法 2: 使用个人访问令牌 (Personal Access Token)

1. 访问 GitHub: https://github.com/settings/tokens
2. 生成新的 token (需要 `repo` 权限)
3. 使用 token 推送:

```bash
# 使用 token 推送 (一次性)
git push https://YOUR_TOKEN@github.com/DEEIX-AI/DEEIX-Chat.git dev

# 或者配置 credential helper
git config credential.helper store
git push origin dev  # 然后输入 username 和 token
```

### 方法 3: 联系仓库管理员

如果您不是仓库的协作者，需要：
1. 联系 DEEIX-AI 组织管理员
2. 请求添加为协作者
3. 或者创建 Pull Request

---

## 📦 本地部署步骤（无需推送）

如果暂时无法推送代码，可以在本地或服务器直接部署：

### 1. 打包代码

```bash
# 创建部署包（排除不必要的文件）
git archive -o deeix-chat-$(date +%Y%m%d).tar.gz HEAD

# 或使用 zip
git archive -o deeix-chat-$(date +%Y%m%d).zip HEAD
```

### 2. 上传到服务器

```bash
# 使用 SCP 上传
scp deeix-chat-*.tar.gz user@your-server:/path/to/deploy/

# 或使用 rsync（更快）
rsync -avz --exclude 'node_modules' --exclude '.git' \
  ./ user@your-server:/path/to/DEEIX-Chat/
```

### 3. 在服务器上部署

```bash
# SSH 登录服务器
ssh user@your-server

# 进入项目目录
cd /path/to/DEEIX-Chat

# 解压（如果使用了打包）
tar -xzf deeix-chat-*.tar.gz

# 运行部署脚本
chmod +x deploy.sh
./deploy.sh production dev
```

---

## 🐳 Docker 快速部署

### 一键部署命令

```bash
# 方法 1: 使用部署脚本
./deploy.sh production dev

# 方法 2: 手动命令
docker-compose down
docker-compose build --no-cache
docker-compose up -d
docker-compose logs -f app
```

### 验证部署

```bash
# 检查容器状态
docker-compose ps

# 查看日志
docker-compose logs -f app

# 测试访问
curl http://localhost:8080/health
```

---

## ✅ 功能验证清单

部署完成后，请验证以下新功能：

### 1. 上游模型空行修复 ✅

1. 登录管理后台
2. 进入: 上游管理 → 选择一个上游
3. 点击"获取模型"按钮
4. **验证**: 模型列表不再有空行，总数统计正确

### 2. 模型价格快速配置 ⚡

1. 进入: 计费设置 → 模型定价
2. 点击工具栏的闪电图标 ⚡
3. 设置价格倍率（如 1.0）
4. 点击"应用配置"
5. **验证**: 未配置的模型自动设置了价格

### 3. 品牌设置功能 🎨

> ⚠️ 注意: 此功能需要后端 API 支持，前端代码已完成

1. 进入: 管理后台 → 品牌设置（需要先添加菜单）
2. 上传图标和 Logo
3. 选择主题色
4. **验证**: 保存后页面刷新，应用新品牌

---

## 🔧 故障排查

### 问题 1: 容器无法启动

```bash
# 查看详细错误
docker-compose logs app

# 检查端口占用
netstat -tulpn | grep 8080

# 重新构建
docker-compose build --no-cache
```

### 问题 2: 前端资源 404

```bash
# 清除缓存重新构建
docker-compose down -v
docker-compose build --no-cache
docker-compose up -d
```

### 问题 3: 数据库连接失败

```bash
# 检查配置文件
cat config.yaml

# 检查网络
docker network ls
docker network inspect deeix-chat-network
```

---

## 📊 监控部署

### 实时日志

```bash
# 查看所有日志
docker-compose logs -f

# 只看应用日志
docker-compose logs -f app

# 过滤错误
docker-compose logs app | grep -i error
```

### 资源使用

```bash
# 查看容器资源
docker stats deeix-chat-app

# 查看磁盘使用
docker system df
```

---

## 🔄 回滚操作

如果部署出现问题：

```bash
# 查看备份镜像
docker images | grep deeix-chat

# 回滚到备份版本
docker tag deeix-chat:backup-YYYYMMDD-HHMMSS deeix-chat:latest
docker-compose up -d

# 或使用 Git 回滚
git reset --hard HEAD~1
docker-compose build
docker-compose up -d
```

---

## 📝 部署后任务

### 1. 更新文档

- [ ] 更新 CHANGELOG.md
- [ ] 通知团队新功能
- [ ] 更新用户手册

### 2. 监控和告警

- [ ] 配置日志监控
- [ ] 设置错误告警
- [ ] 检查性能指标

### 3. 备份

- [ ] 备份数据库
- [ ] 备份配置文件
- [ ] 备份 Docker 镜像

---

## 🎯 下一步

### 完成品牌设置功能

品牌设置功能前端已完成，需要后端支持：

1. **添加后端 API**:
   ```
   GET /api/v1/public/branding
   ```

2. **返回格式**:
   ```json
   {
     "site_name": "DEEIX Chat",
     "site_title": "DEEIX Chat",
     "favicon_url": "",
     "logo_light_url": "",
     "logo_dark_url": "",
     "theme_color": "#0f172a"
   }
   ```

3. **集成到前端**:
   - 在 `app/layout.tsx` 添加 `BrandingProvider`
   - 更新 `app-logo.tsx` 使用动态 Logo
   - 添加管理后台菜单

详见: [BRANDING_FEATURE.md](./BRANDING_FEATURE.md)

---

## 📞 获取帮助

- 📖 完整部署文档: [DEPLOYMENT.md](./DEPLOYMENT.md)
- 🔧 功能更新说明: [CHANGES.md](./CHANGES.md)
- 🎨 品牌设置文档: [BRANDING_FEATURE.md](./BRANDING_FEATURE.md)
- 💬 技术支持: Telegram [@deeix_chat](https://t.me/deeix_chat)

---

**更新日期**: 2026-07-09  
**提交哈希**: 7f9edf8  
**分支**: dev
