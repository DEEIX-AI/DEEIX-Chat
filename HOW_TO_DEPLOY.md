# 🚀 最终部署指南

## 📊 当前状态

### ✅ 已完成的工作

- ✅ **3 个提交已创建并保存在本地**
  - `7f9edf8` - 核心功能实现（修复空行、快速配置、品牌设置）
  - `08240a8` - 部署文档和脚本
  - `7e8cf51` - 部署总结文档

- ✅ **功能开发完成**
  - 上游模型空行修复
  - 模型价格一键快速配置
  - 品牌自定义功能（前端）

- ✅ **文档已完成**
  - CHANGES.md - 功能更新说明
  - BRANDING_FEATURE.md - 品牌设置文档
  - DEPLOYMENT.md - 详细部署指南
  - QUICK_DEPLOY.md - 快速部署指南
  - DEPLOY_SUMMARY.md - 部署总结
  - deploy.sh - 自动化部署脚本

### ⚠️ 当前问题

**Git 推送权限被拒绝**
- GitHub 账号: `qianye60`
- 仓库: `DEEIX-AI/DEEIX-Chat`
- 错误: `Permission denied`
- 原因: 您的账号不是 DEEIX-AI 组织的协作者

---

## 🎯 部署方案（3 种方法）

### 方案 1: 创建 Pull Request（推荐）⭐

这是最标准的开源协作流程：

#### 步骤 1: Fork 仓库并推送

```bash
# 1. 访问 GitHub 上 Fork 仓库
# https://github.com/DEEIX-AI/DEEIX-Chat
# 点击右上角 "Fork" 按钮

# 2. 添加您的 Fork 为远程仓库
git remote add my-fork git@github.com:qianye60/DEEIX-Chat.git

# 3. 推送到您的 Fork
git push my-fork dev

# 或者推送到新分支
git push my-fork dev:feature/pricing-and-branding
```

#### 步骤 2: 创建 Pull Request

1. 访问您的 Fork: https://github.com/qianye60/DEEIX-Chat
2. 点击 "Contribute" → "Open pull request"
3. 填写 PR 信息:
   ```
   标题: feat: add model pricing quick config and branding customization
   
   描述:
   ## 🎯 功能更新
   
   ### 1. 修复上游模型空行问题
   - 在获取上游模型时过滤空的模型名称
   - 修复总数统计不准确的问题（如 56/57 → 56/56）
   
   ### 2. 添加模型价格一键快速配置
   - 新增闪电图标按钮在模型定价页面
   - 支持批量配置未定价的模型
   - 智能匹配 OpenRouter 官方价格
   - 支持价格倍率调整
   
   ### 3. 添加品牌自定义功能（前端）
   - 管理页面支持上传网站图标和 Logo
   - 支持配置网站名称和标题
   - 支持主题色自定义
   - 需要后端 API 支持（待实现）
   
   ## 📝 变更文件
   
   - frontend/features/admin/components/sections/upstreams/upstreams-models-dialog.tsx
   - frontend/features/admin/components/sections/billing/billing-prices.tsx
   - frontend/features/admin/components/sections/branding/admin-branding.tsx
   - frontend/shared/components/branding-provider.tsx
   - frontend/i18n/messages/zh-CN/admin-billing.json
   - frontend/i18n/messages/zh-CN/admin-branding.json
   - frontend/i18n/messages/en-US/admin-branding.json
   
   ## ✅ 测试
   
   - [x] 上游模型同步无空行
   - [x] 快速配置功能正常
   - [x] 品牌设置页面可访问（需后端支持）
   
   ## 📖 文档
   
   - CHANGES.md - 功能更新说明
   - BRANDING_FEATURE.md - 品牌设置文档
   - DEPLOYMENT.md - 部署指南
   ```
4. 点击 "Create pull request"

#### 步骤 3: 等待审核合并

PR 被合并后，您可以在服务器上部署：

```bash
# 在服务器上
cd /path/to/DEEIX-Chat
git pull origin dev
./deploy.sh
```

---

### 方案 2: 直接上传到服务器部署（最快）⚡

如果您有服务器访问权限，可以直接上传代码部署：

#### 选项 A: 使用 rsync（推荐）

```bash
# 在本地 Windows 上，使用 Git Bash 或 WSL
rsync -avz --exclude 'node_modules' --exclude '.git' --exclude 'frontend/.next' \
  --exclude 'backend/tmp' --delete \
  /c/Users/15385/Documents/CodeProject/DEEIX-Chat/ \
  user@your-server:/path/to/DEEIX-Chat/

# 然后 SSH 到服务器部署
ssh user@your-server
cd /path/to/DEEIX-Chat
chmod +x deploy.sh
./deploy.sh production dev
```

#### 选项 B: 使用 SCP

```bash
# 创建压缩包
cd /c/Users/15385/Documents/CodeProject/DEEIX-Chat
tar -czf deeix-chat-update-$(date +%Y%m%d).tar.gz \
  --exclude='node_modules' \
  --exclude='.git' \
  --exclude='frontend/.next' \
  .

# 上传到服务器
scp deeix-chat-update-*.tar.gz user@your-server:/tmp/

# SSH 到服务器
ssh user@your-server

# 解压并部署
cd /path/to/DEEIX-Chat
tar -xzf /tmp/deeix-chat-update-*.tar.gz
chmod +x deploy.sh
./deploy.sh production dev
```

#### 选项 C: 使用 Git Bundle

```bash
# 创建 Git Bundle（包含最近 3 个提交）
git bundle create deeix-chat-update.bundle HEAD~3..HEAD

# 上传 Bundle
scp deeix-chat-update.bundle user@your-server:/tmp/

# SSH 到服务器
ssh user@your-server
cd /path/to/DEEIX-Chat

# 应用 Bundle
git pull /tmp/deeix-chat-update.bundle dev

# 部署
chmod +x deploy.sh
./deploy.sh production dev
```

---

### 方案 3: 联系管理员添加权限

如果您需要长期协作，建议联系 DEEIX-AI 组织管理员：

1. **联系方式**:
   - Telegram: [@deeix_chat](https://t.me/deeix_chat)
   - GitHub: 在仓库创建 Issue 说明情况
   - Email: 通过官网联系

2. **请求内容**:
   ```
   您好，我是开发者 qianye60，
   
   我为 DEEIX-Chat 项目贡献了以下功能：
   - 修复上游模型空行问题
   - 添加模型价格快速配置功能
   - 添加品牌自定义功能
   
   希望能被添加为仓库协作者，以便直接推送代码。
   
   GitHub: https://github.com/qianye60
   ```

3. **被添加后**:
   ```bash
   # 切换回 SSH URL（如果还没切换）
   git remote set-url origin git@github.com:DEEIX-AI/DEEIX-Chat.git
   
   # 推送代码
   git push origin dev
   ```

---

## 🎯 推荐执行顺序

### 立即行动（10分钟内）

**如果您有服务器访问权限** → 使用**方案 2**（直接上传部署）

```bash
# 1. 使用 rsync 上传（最简单）
rsync -avz --exclude 'node_modules' --exclude '.git' \
  /c/Users/15385/Documents/CodeProject/DEEIX-Chat/ \
  user@your-server:/path/to/DEEIX-Chat/

# 2. SSH 到服务器部署
ssh user@your-server
cd /path/to/DEEIX-Chat
chmod +x deploy.sh
./deploy.sh production dev

# 3. 验证功能
# - 测试上游模型同步
# - 测试快速配置价格
```

### 长期协作（推荐）

**创建 Pull Request** → 使用**方案 1**

```bash
# 1. Fork 仓库（在 GitHub 网页上操作）

# 2. 添加 Fork 并推送
git remote add my-fork git@github.com:qianye60/DEEIX-Chat.git
git push my-fork dev:feature/pricing-and-branding

# 3. 在 GitHub 上创建 PR

# 4. 等待合并后，在服务器部署
ssh user@your-server
cd /path/to/DEEIX-Chat
git pull origin dev
./deploy.sh
```

---

## 📋 部署脚本使用说明

### 自动化部署脚本

```bash
# 基本用法
./deploy.sh [environment] [branch]

# 示例
./deploy.sh production dev     # 生产环境，dev 分支
./deploy.sh staging main       # 预发布环境，main 分支
./deploy.sh                    # 默认：production dev
```

### 脚本执行流程

1. ✅ 拉取最新代码
2. 💾 创建备份镜像
3. 🔨 构建新 Docker 镜像
4. ⏸️ 停止旧容器
5. 🚀 启动新容器
6. 🏥 健康检查

### 回滚操作

如果部署失败，脚本会自动回滚。手动回滚：

```bash
# 查看备份镜像
docker images | grep backup

# 回滚到备份版本
docker tag deeix-chat:backup-YYYYMMDD-HHMMSS deeix-chat:latest
docker-compose up -d
```

---

## ✅ 部署后验证清单

### 1. 基础检查

```bash
# 容器状态
docker-compose ps

# 应用日志
docker-compose logs -f app

# 健康检查
curl http://localhost:8080/health
```

### 2. 功能验证

#### ✅ 上游模型空行修复

- [ ] 登录管理后台
- [ ] 进入上游管理
- [ ] 点击获取模型
- [ ] 确认无空行，总数正确

#### ⚡ 模型价格快速配置

- [ ] 进入计费设置 → 模型定价
- [ ] 点击闪电图标 ⚡
- [ ] 设置倍率并应用
- [ ] 确认未配置模型已设置价格

#### 🎨 品牌设置（待后端实现）

- [ ] 后端 API 已实现
- [ ] 前端集成完成
- [ ] 可上传图标和 Logo
- [ ] 可自定义主题色

---

## 📞 获取帮助

### 文档资源

- 📖 [DEPLOYMENT.md](./DEPLOYMENT.md) - 详细部署指南
- 🚀 [QUICK_DEPLOY.md](./QUICK_DEPLOY.md) - 快速部署
- 📝 [CHANGES.md](./CHANGES.md) - 功能更新
- 🎨 [BRANDING_FEATURE.md](./BRANDING_FEATURE.md) - 品牌设置

### 联系支持

- 💬 Telegram: [@deeix_chat](https://t.me/deeix_chat)
- 🐛 GitHub Issues
- 📧 技术支持邮箱

---

## 🎉 总结

您已经完成了三个重要功能的开发：

1. ✅ **上游模型空行修复** - 提升用户体验
2. ✅ **模型价格快速配置** - 节省管理时间
3. ✅ **品牌自定义功能** - 支持平台定制（前端完成）

**现在只需要选择一个部署方案执行即可！**

推荐执行顺序：
1. **立即**: 使用方案 2 直接上传到服务器部署
2. **同时**: 创建方案 1 的 Pull Request 供审核
3. **可选**: 联系管理员获取推送权限

---

**更新时间**: 2026-07-09 03:10  
**本地提交**: 3 个（待推送）  
**状态**: ✅ 开发完成，⏳ 等待部署
