package repository

import (
	"context"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

// BillingRepository 定义计费流程依赖的持久化能力。
type BillingRepository interface {
	ListActivePlans(ctx context.Context) ([]domainbilling.Plan, error)
	ListActivePricesByPlanIDs(ctx context.Context, planIDs []uint) ([]domainbilling.Price, error)
	GetPriceByID(ctx context.Context, priceID uint) (*domainbilling.Price, error)
	GetPlanByID(ctx context.Context, planID uint) (*domainbilling.Plan, error)
	ListPlansByIDs(ctx context.Context, planIDs []uint) ([]domainbilling.Plan, error)
	GetActivePlanByCode(ctx context.Context, code string) (*domainbilling.Plan, error)
	UpdatePlanWithDefaultPrice(ctx context.Context, plan *domainbilling.Plan, price *domainbilling.Price) error
	ListCurrentSubscriptionsByUserIDs(ctx context.Context, userIDs []uint, now time.Time) ([]domainbilling.Subscription, error)
	ReplaceSubscription(ctx context.Context, item *domainbilling.Subscription) error
	CreatePaymentOrder(ctx context.Context, item *domainbilling.PaymentOrder) (*domainbilling.PaymentOrder, error)
	UpdatePaymentOrderCheckout(ctx context.Context, orderNo string, externalCheckoutID string, checkoutURL string) error
	GetPaymentOrderByOrderNo(ctx context.Context, orderNo string) (*domainbilling.PaymentOrder, error)
	MarkPaymentOrderPaidAndReplaceSubscription(ctx context.Context, orderNo string, externalPaymentID string, paidAt time.Time, subscription *domainbilling.Subscription) (*domainbilling.PaymentOrder, bool, error)
	AddUsage(ctx context.Context, usage *domainbilling.UsageLedger) error
	AddUsageAndDebitBalance(ctx context.Context, usage *domainbilling.UsageLedger) error
	AddUsageAndSettleBalance(ctx context.Context, usage *domainbilling.UsageLedger, reservation *domainbilling.UsageBalanceReservation) error
	ReserveUsageBalance(ctx context.Context, userID uint, amountNanousd int64, refNo string) (*domainbilling.UsageBalanceReservation, error)
	ReleaseUsageBalanceReservation(ctx context.Context, userID uint, refNo string, description string) error
	GetOrCreateBillingAccount(ctx context.Context, userID uint) (*domainbilling.BillingAccount, error)
	ListBillingAccountsByUserIDs(ctx context.Context, userIDs []uint) ([]domainbilling.BillingAccount, error)
	SetBillingAccountBalance(ctx context.Context, userID uint, balanceNanousd int64, refNo string, description string) (*domainbilling.BillingAccount, error)
	MarkPaymentOrderPaidAndCreditBalance(ctx context.Context, orderNo string, externalPaymentID string, paidAt time.Time) (*domainbilling.PaymentOrder, bool, error)
	GetBillingMode(ctx context.Context) (string, error)
	GetBillingPrepaidAmountNanousd(ctx context.Context) (int64, error)
	GetModelPricing(ctx context.Context, platformModelName string) (*domainbilling.ModelPricing, error)
	ListModelPricing(ctx context.Context, query string, offset int, limit int) ([]domainbilling.ModelPricing, int64, error)
	UpsertModelPricing(ctx context.Context, item *domainbilling.ModelPricing) (*domainbilling.ModelPricing, error)
	ListUsageByUser(ctx context.Context, userID uint, filter UsageListFilter, offset int, limit int) ([]domainbilling.UsageLedger, int64, error)
	ListUsageLogs(ctx context.Context, filter UsageLogListFilter, offset int, limit int) ([]domainbilling.UsageLedger, int64, error)
	GetUserCreatedAt(ctx context.Context, userID uint) (time.Time, error)
	ListMonthlyUsageByUser(ctx context.Context, userID uint, limit int) ([]domainbilling.UsageMonthlySummary, error)
	ListDailyUsageByUser(ctx context.Context, userID uint, startDate time.Time, endDate time.Time) ([]domainbilling.UsageDailySummary, error)
	SumBillableNanousd(ctx context.Context, userID uint, startAt time.Time, endAt time.Time) (int64, error)
}

// UsageListFilter 描述用户用量账本的筛选和排序条件。
type UsageListFilter struct {
	Query  string
	Status string
	Sort   string
}

// UsageLogListFilter 描述管理员调用日志筛选和排序条件。
type UsageLogListFilter struct {
	Query             string
	PlatformModelName string
	BillingMode       string
	UserID            uint
	CreatedFrom       *time.Time
	CreatedTo         *time.Time
	Sort              string
}
