package admin

import (
	"context"

	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	systemeventapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
)

// ListAuditLogs 查询审计日志分页列表。
func (s *Service) ListAuditLogs(ctx context.Context, page int, pageSize int, filter auditapp.ListFilter) ([]domainaudit.Log, int64, error) {
	return s.auditService.List(ctx, page, pageSize, filter)
}

// ListUsageLogs 查询管理员调用日志。
func (s *Service) ListUsageLogs(ctx context.Context, page int, pageSize int, filter billing.UsageLogListFilter) ([]domainbilling.UsageLedger, int64, error) {
	if s.usageLogService == nil {
		return []domainbilling.UsageLedger{}, 0, nil
	}
	return s.usageLogService.ListUsageLogs(ctx, page, pageSize, filter)
}

// ListPaymentOrders 查询管理员支付订单记录。
func (s *Service) ListPaymentOrders(ctx context.Context, page int, pageSize int, filter billing.PaymentOrderListFilter) ([]domainbilling.PaymentOrder, int64, error) {
	if s.orderLogService == nil {
		return []domainbilling.PaymentOrder{}, 0, nil
	}
	return s.orderLogService.ListPaymentOrders(ctx, page, pageSize, filter)
}

// ListConversationEventLogs 查询管理员对话事件。
func (s *Service) ListConversationEventLogs(ctx context.Context, page int, pageSize int, filter appconversation.EventLogListFilter) ([]domainconversation.EventLog, int64, error) {
	if s.conversationEventSvc == nil {
		return []domainconversation.EventLog{}, 0, nil
	}
	return s.conversationEventSvc.ListConversationEventLogs(ctx, page, pageSize, filter)
}

// ListSystemEvents 查询系统事件分页列表。
func (s *Service) ListSystemEvents(ctx context.Context, page int, pageSize int, filter systemeventapp.ListFilter) ([]domainsystemevent.Event, int64, error) {
	if s.systemEventService == nil {
		return []domainsystemevent.Event{}, 0, nil
	}
	return s.systemEventService.List(ctx, page, pageSize, filter)
}
