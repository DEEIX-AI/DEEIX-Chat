#!/bin/bash

# DEEIX Chat 快速部署脚本
# 使用方法: ./deploy.sh [production|staging]

set -e

ENVIRONMENT=${1:-production}
BRANCH=${2:-dev}

echo "🚀 开始部署 DEEIX Chat"
echo "📦 环境: $ENVIRONMENT"
echo "🌿 分支: $BRANCH"
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查是否在正确的目录
if [ ! -f "docker-compose.yml" ]; then
    echo -e "${RED}❌ 错误: 未找到 docker-compose.yml 文件${NC}"
    echo "请在项目根目录运行此脚本"
    exit 1
fi

# 步骤 1: 拉取最新代码
echo -e "${YELLOW}📥 步骤 1/6: 拉取最新代码...${NC}"
git fetch origin
git checkout $BRANCH
git pull origin $BRANCH

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Git pull 失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 代码更新完成${NC}"
echo ""

# 步骤 2: 备份当前容器（可选）
echo -e "${YELLOW}💾 步骤 2/6: 创建备份...${NC}"
BACKUP_TAG="backup-$(date +%Y%m%d-%H%M%S)"

if docker images | grep -q "deeix-chat"; then
    docker tag deeix-chat:latest deeix-chat:$BACKUP_TAG || true
    echo -e "${GREEN}✅ 已创建备份镜像: deeix-chat:$BACKUP_TAG${NC}"
else
    echo -e "${YELLOW}⚠️  未找到现有镜像，跳过备份${NC}"
fi
echo ""

# 步骤 3: 构建新镜像
echo -e "${YELLOW}🔨 步骤 3/6: 构建 Docker 镜像...${NC}"
docker-compose build --no-cache

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Docker 构建失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 镜像构建完成${NC}"
echo ""

# 步骤 4: 停止旧容器
echo -e "${YELLOW}⏸️  步骤 4/6: 停止旧容器...${NC}"
docker-compose down

if [ $? -ne 0 ]; then
    echo -e "${YELLOW}⚠️  停止容器时出现警告，继续部署...${NC}"
fi
echo -e "${GREEN}✅ 旧容器已停止${NC}"
echo ""

# 步骤 5: 启动新容器
echo -e "${YELLOW}🚀 步骤 5/6: 启动新容器...${NC}"
docker-compose up -d

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ 容器启动失败${NC}"
    echo -e "${YELLOW}尝试回滚到备份版本...${NC}"
    docker tag deeix-chat:$BACKUP_TAG deeix-chat:latest
    docker-compose up -d
    exit 1
fi
echo -e "${GREEN}✅ 新容器已启动${NC}"
echo ""

# 步骤 6: 健康检查
echo -e "${YELLOW}🏥 步骤 6/6: 健康检查...${NC}"
echo "等待服务启动..."
sleep 10

# 检查容器状态
CONTAINER_STATUS=$(docker-compose ps -q app | xargs docker inspect -f '{{.State.Status}}')

if [ "$CONTAINER_STATUS" = "running" ]; then
    echo -e "${GREEN}✅ 容器运行正常${NC}"
else
    echo -e "${RED}❌ 容器状态异常: $CONTAINER_STATUS${NC}"
    docker-compose logs --tail=50 app
    exit 1
fi

# 检查应用健康状态（如果有健康检查端点）
if curl -f -s http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 应用健康检查通过${NC}"
else
    echo -e "${YELLOW}⚠️  健康检查失败或未配置${NC}"
    echo "请手动验证应用是否正常运行"
fi
echo ""

# 部署完成
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}🎉 部署完成！${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo "📊 部署信息:"
echo "  - 环境: $ENVIRONMENT"
echo "  - 分支: $BRANCH"
echo "  - 备份: deeix-chat:$BACKUP_TAG"
echo "  - 时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo ""
echo "🔍 查看日志:"
echo "  docker-compose logs -f app"
echo ""
echo "📋 验证功能:"
echo "  1. 访问: http://localhost:8080"
echo "  2. 登录管理后台"
echo "  3. 测试新功能:"
echo "     - 模型定价 -> 快速配置 ⚡"
echo "     - 上游管理 -> 模型同步（无空行）"
echo "     - 品牌设置（如果后端已实现）"
echo ""
echo "🔄 回滚命令（如需要）:"
echo "  docker tag deeix-chat:$BACKUP_TAG deeix-chat:latest"
echo "  docker-compose up -d"
echo ""
