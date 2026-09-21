package conversation

import (
	"context"

	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
)

// resolveUIComponents 把会话勾选的组件 ID 解析为可注入提示词的组件；不可见的 ID 静默忽略，
// 因为组件只影响渲染能力，不像技能那样改变模型行为，不值得让整条消息失败。
func (s *Service) resolveUIComponents(ctx context.Context, userID uint, ids []uint) ([]domainuicomponent.Component, error) {
	if len(ids) == 0 || s.uiComponentResolver == nil {
		return nil, nil
	}
	return s.uiComponentResolver.ResolveVisible(ctx, userID, ids)
}
