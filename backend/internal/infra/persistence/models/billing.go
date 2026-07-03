package model

import "time"

const (
	// BillingIntervalMonth 表示按月计费。
	BillingIntervalMonth = "month"
	// BillingIntervalYear 表示按年计费。
	BillingIntervalYear = "year"
	// BillingIntervalLifetime 表示永久价格，如默认 free。
	BillingIntervalLifetime = "lifetime"
)

// BillingPlan 定义订阅等级本身，不直接承载价格周期。
type BillingPlan struct {
	BaseModel
	Code                string `gorm:"size:32;not null;uniqueIndex:idx_billing_plans_code;comment:套餐编码"`
	Name                string `gorm:"size:64;not null;default:'';comment:套餐名称"`
	Description         string `gorm:"size:255;not null;default:'';comment:套餐说明"`
	FeatureJSON         string `gorm:"type:text;not null;default:'';comment:能力配置JSON"`
	PeriodCreditNanousd int64  `gorm:"not null;default:0;comment:周期用量额度(纳美元)"`
	DiscountPercent     int    `gorm:"not null;default:0;comment:默认折扣百分比"`
	SortOrder           int    `gorm:"not null;default:0;comment:排序权重"`
	IsActive            bool   `gorm:"not null;default:false;index:idx_billing_plans_active;comment:是否启用"`
	PermissionGroupID   *uint  `gorm:"index:idx_billing_plans_perm_group;comment:绑定的权限组ID"`
}

// TableName 指定表名。
func (BillingPlan) TableName() string {
	return "billing_plans"
}

// BillingPrice 定义套餐在不同周期下的价格版本。
type BillingPrice struct {
	BaseModel
	PlanID           uint   `gorm:"not null;index:idx_billing_prices_plan_id;comment:套餐ID"`
	Code             string `gorm:"size:48;not null;uniqueIndex:idx_billing_prices_code;comment:价格编码"`
	BillingInterval  string `gorm:"size:16;not null;default:'month';comment:计费周期(month/year/lifetime)"`
	Currency         string `gorm:"size:16;not null;default:'USD';comment:币种"`
	AmountCents      int64  `gorm:"not null;default:0;comment:金额(分)"`
	IsActive         bool   `gorm:"not null;default:false;index:idx_billing_prices_active;comment:是否启用"`
	IsDefault        bool   `gorm:"not null;default:false;index:idx_billing_prices_default;comment:是否为默认价格"`
	ExternalPriceRef string `gorm:"size:128;not null;default:'';comment:外部支付价格ID"`
}

// TableName 指定表名。
func (BillingPrice) TableName() string {
	return "billing_prices"
}

// Subscription 记录用户订阅到哪个套餐价格。
type Subscription struct {
	BaseModel
	UserID               uint       `gorm:"not null;index:idx_billing_subscriptions_user_id;comment:用户ID"`
	PlanID               uint       `gorm:"not null;index:idx_billing_subscriptions_plan_id;comment:套餐ID"`
	PriceID              uint       `gorm:"not null;index:idx_billing_subscriptions_price_id;comment:价格ID"`
	Status               string     `gorm:"size:32;not null;default:'active';index:idx_billing_subscriptions_status;comment:订阅状态"`
	StartAt              time.Time  `gorm:"not null;comment:订阅开始时间"`
	CurrentPeriodStartAt time.Time  `gorm:"not null;comment:当前计费周期开始时间"`
	CurrentPeriodEndAt   *time.Time `gorm:"comment:当前计费周期结束时间"`
	CancelAtPeriodEnd    bool       `gorm:"not null;default:false;comment:是否到期取消"`
	CanceledAt           *time.Time `gorm:"comment:取消时间"`
	AutoRenew            bool       `gorm:"not null;default:false;comment:是否自动续费"`
}

// TableName 指定表名。
func (Subscription) TableName() string {
	return "billing_subscriptions"
}

// PaymentOrder 记录用户购买套餐时创建的支付单。
type PaymentOrder struct {
	BaseModel
	OrderNo            string     `gorm:"size:64;not null;uniqueIndex:idx_billing_payment_orders_order_no;comment:内部支付单号"`
	OrderType          string     `gorm:"size:32;not null;default:'subscription';index:idx_billing_payment_orders_order_type;comment:支付单类型(subscription/topup)"`
	UserID             uint       `gorm:"not null;index:idx_billing_payment_orders_user_id;comment:用户ID"`
	PlanID             uint       `gorm:"not null;index:idx_billing_payment_orders_plan_id;comment:套餐ID"`
	PriceID            uint       `gorm:"not null;index:idx_billing_payment_orders_price_id;comment:价格ID"`
	Provider           string     `gorm:"size:32;not null;default:'';index:idx_billing_payment_orders_provider;comment:支付渠道(stripe/epay)"`
	Status             string     `gorm:"size:32;not null;default:'pending';index:idx_billing_payment_orders_status;comment:支付状态"`
	BaseCurrency       string     `gorm:"size:16;not null;default:'USD';comment:基准币种"`
	BaseAmountCents    int64      `gorm:"not null;default:0;comment:基准金额(分)"`
	PayCurrency        string     `gorm:"size:16;not null;default:'CNY';comment:支付币种"`
	PayAmountCents     int64      `gorm:"not null;default:0;comment:支付金额(分)"`
	FXRate             string     `gorm:"size:32;not null;default:'';comment:汇率快照"`
	CreditNanousd      int64      `gorm:"not null;default:0;comment:充值入账金额(纳美元)"`
	BillingInterval    string     `gorm:"size:16;not null;default:'month';comment:计费周期"`
	Cycles             int        `gorm:"not null;default:1;comment:购买周期数"`
	ExternalPaymentID  string     `gorm:"size:128;not null;default:'';index:idx_billing_payment_orders_external_payment_id;comment:外部支付ID"`
	ExternalCheckoutID string     `gorm:"size:128;not null;default:'';index:idx_billing_payment_orders_external_checkout_id;comment:外部收银台ID"`
	CheckoutURL        string     `gorm:"type:text;not null;default:'';comment:收银台跳转地址"`
	PaidAt             *time.Time `gorm:"comment:支付完成时间"`
	ExpiredAt          *time.Time `gorm:"comment:支付单过期时间"`
	SnapshotJSON       string     `gorm:"type:text;not null;default:'';comment:支付单快照JSON"`
}

// TableName 指定表名。
func (PaymentOrder) TableName() string {
	return "billing_payment_orders"
}

// BillingAccount 记录用户按量计费余额账户。
type BillingAccount struct {
	BaseModel
	UserID         uint   `gorm:"not null;uniqueIndex:idx_billing_accounts_user_id;comment:用户ID"`
	Currency       string `gorm:"size:16;not null;default:'USD';comment:余额币种"`
	BalanceNanousd int64  `gorm:"not null;default:0;comment:余额(纳美元)"`
	Status         string `gorm:"size:32;not null;default:'active';index:idx_billing_accounts_status;comment:账户状态"`
}

// TableName 指定表名。
func (BillingAccount) TableName() string {
	return "billing_accounts"
}

// BalanceTransaction 记录余额变动流水。
type BalanceTransaction struct {
	BaseModel
	AccountID           uint   `gorm:"not null;index:idx_billing_balance_transactions_account_id;comment:余额账户ID"`
	UserID              uint   `gorm:"not null;index:idx_billing_balance_transactions_user_id;comment:用户ID"`
	Type                string `gorm:"size:32;not null;index:idx_billing_balance_transactions_type;comment:流水类型"`
	AmountNanousd       int64  `gorm:"not null;default:0;comment:变动金额(纳美元)"`
	BalanceAfterNanousd int64  `gorm:"not null;default:0;comment:变动后余额(纳美元)"`
	RefType             string `gorm:"size:64;not null;default:'';index:idx_billing_balance_transactions_ref;comment:关联对象类型"`
	RefID               uint   `gorm:"not null;default:0;index:idx_billing_balance_transactions_ref;comment:关联对象ID"`
	RefNo               string `gorm:"size:128;not null;default:'';index:idx_billing_balance_transactions_ref_no;comment:关联对象编号"`
	Description         string `gorm:"size:255;not null;default:'';comment:流水说明"`
}

// TableName 指定表名。
func (BalanceTransaction) TableName() string {
	return "billing_balance_transactions"
}

// RedemptionCode 记录管理员创建的兑换码定义。
type RedemptionCode struct {
	BaseModel
	CodeHash        string     `gorm:"size:64;not null;uniqueIndex:idx_billing_redemption_codes_hash;comment:兑换码HMAC-SHA256哈希"`
	CodeEncrypted   string     `gorm:"type:text;not null;default:'';comment:AES-GCM加密后的兑换码明文"`
	CodeHint        string     `gorm:"size:32;not null;default:'';comment:兑换码展示提示，不包含完整明文"`
	Mode            string     `gorm:"size:16;not null;default:'usage';index:idx_billing_redemption_codes_mode;comment:适用计费模式(usage/period)"`
	RewardType      string     `gorm:"size:32;not null;default:'balance';index:idx_billing_redemption_codes_reward_type;comment:奖励类型(balance/subscription)"`
	CreditNanousd   int64      `gorm:"not null;default:0;comment:余额奖励金额(纳美元)"`
	PlanID          uint       `gorm:"not null;default:0;index:idx_billing_redemption_codes_plan_id;comment:订阅套餐ID"`
	DurationDays    int        `gorm:"not null;default:0;comment:订阅有效天数"`
	MaxRedemptions  *int       `gorm:"comment:总兑换次数上限，空表示不限"`
	PerUserLimit    int        `gorm:"not null;default:1;comment:单用户兑换次数上限"`
	RedeemedCount   int        `gorm:"not null;default:0;comment:已兑换次数"`
	Status          string     `gorm:"size:32;not null;default:'active';index:idx_billing_redemption_codes_status;comment:状态(active/inactive/deleted)"`
	ExpiresAt       *time.Time `gorm:"index:idx_billing_redemption_codes_expires_at;comment:过期时间"`
	Description     string     `gorm:"size:255;not null;default:'';comment:兑换码说明"`
	CreatedByUserID uint       `gorm:"not null;default:0;index:idx_billing_redemption_codes_created_by;comment:创建管理员ID"`
}

// TableName 指定表名。
func (RedemptionCode) TableName() string {
	return "billing_redemption_codes"
}

// Redemption 记录用户的一次兑换行为。
type Redemption struct {
	BaseModel
	CodeID               uint   `gorm:"not null;index:idx_billing_redemptions_code_id;index:idx_billing_redemptions_code_user,priority:1;comment:兑换码ID"`
	UserID               uint   `gorm:"not null;index:idx_billing_redemptions_user_id;index:idx_billing_redemptions_code_user,priority:2;comment:兑换用户ID"`
	Mode                 string `gorm:"size:16;not null;default:'usage';index:idx_billing_redemptions_mode;comment:兑换时计费模式"`
	RewardType           string `gorm:"size:32;not null;default:'balance';comment:奖励类型"`
	CreditNanousd        int64  `gorm:"not null;default:0;comment:余额奖励金额(纳美元)"`
	PlanID               uint   `gorm:"not null;default:0;index:idx_billing_redemptions_plan_id;comment:订阅套餐ID"`
	SubscriptionID       uint   `gorm:"not null;default:0;index:idx_billing_redemptions_subscription_id;comment:创建的订阅ID"`
	BalanceTransactionID uint   `gorm:"not null;default:0;index:idx_billing_redemptions_balance_tx_id;comment:余额流水ID"`
	RefNo                string `gorm:"size:128;not null;default:'';index:idx_billing_redemptions_ref_no;comment:兑换流水号"`
	SnapshotJSON         string `gorm:"type:text;not null;default:'{}';comment:兑换奖励快照JSON"`
}

// TableName 指定表名。
func (Redemption) TableName() string {
	return "billing_redemptions"
}

// ModelPricing 定义平台模型名对应的统一计费单价。
type ModelPricing struct {
	BaseModel
	PlatformModelName           string `gorm:"size:128;not null;uniqueIndex:idx_billing_model_prices_name;comment:平台模型名计费key"`
	Currency                    string `gorm:"size:16;not null;default:'USD';comment:计费币种"`
	IsFree                      bool   `gorm:"not null;default:false;index:idx_billing_model_prices_is_free;comment:是否免费模型"`
	PricingMode                 string `gorm:"size:16;not null;default:'token';comment:计费模式(token/call/duration/tiered)"`
	InputNanousdPerMTokens      int64  `gorm:"not null;default:0;comment:输入token单价(每百万token,纳美元)"`
	CacheReadNanousdPerMTokens  int64  `gorm:"not null;default:0;comment:缓存读取token单价(每百万token,纳美元)"`
	CacheWriteNanousdPerMTokens int64  `gorm:"not null;default:0;comment:缓存写入token单价(每百万token,纳美元)"`
	OutputNanousdPerMTokens     int64  `gorm:"not null;default:0;comment:输出token单价(每百万token,纳美元)"`
	CallNanousdPerCall          int64  `gorm:"not null;default:0;comment:按次单价(每次,纳美元)"`
	DurationNanousdPerSecond    int64  `gorm:"not null;default:0;comment:按秒单价(每秒,纳美元)"`
	TieredPricingJSON           string `gorm:"type:text;not null;default:'{}';comment:阶梯计费配置JSON"`
}

// TableName 指定表名。
func (ModelPricing) TableName() string {
	return "billing_model_prices"
}

// UsageLedger 记录每日消费与Token使用量，保留计费快照用于审计追溯。
type UsageLedger struct {
	BaseModel
	UserID              uint      `gorm:"not null;index:idx_billing_usage_ledgers_user_id;index:idx_billing_usage_ledgers_user_date,priority:1;index:idx_billing_usage_ledgers_user_billing_at,priority:1;comment:用户ID"`
	ConversationID      uint      `gorm:"not null;index:idx_billing_usage_ledgers_conversation_id;comment:会话ID"`
	ProviderProtocol    string    `gorm:"size:64;not null;default:'';index:idx_billing_usage_ledgers_provider_protocol;comment:协议适配器快照"`
	UpstreamName        string    `gorm:"size:128;not null;default:'';comment:上游名称"`
	PlatformModelName   string    `gorm:"size:128;not null;default:'';index:idx_billing_usage_ledgers_platform_model_name;comment:平台模型名"`
	RoutedBindingCode   string    `gorm:"size:64;not null;default:'';index:idx_billing_usage_ledgers_routed_binding_code;comment:实际路由上游模型绑定编码"`
	UpstreamModelName   string    `gorm:"size:256;not null;default:'';comment:上游真实模型名"`
	IsFreeModel         bool      `gorm:"not null;default:false;index:idx_billing_usage_ledgers_is_free_model;comment:是否免费模型用量"`
	BillingAt           time.Time `gorm:"index:idx_billing_usage_ledgers_billing_at;index:idx_billing_usage_ledgers_user_billing_at,priority:2;comment:计费归属时间"`
	UsageDate           time.Time `gorm:"type:date;not null;index:idx_billing_usage_ledgers_usage_date;index:idx_billing_usage_ledgers_user_date,priority:2;comment:消费日期"`
	InputTokens         int64     `gorm:"not null;default:0;comment:输入Token"`
	CacheReadTokens     int64     `gorm:"not null;default:0;comment:缓存读取Token"`
	CacheWriteTokens    int64     `gorm:"not null;default:0;comment:缓存写入Token"`
	CacheWrite5mTokens  int64     `gorm:"column:cache_write_5m_tokens;not null;default:0;comment:5分钟缓存写入Token"`
	CacheWrite1hTokens  int64     `gorm:"column:cache_write_1h_tokens;not null;default:0;comment:1小时缓存写入Token"`
	OutputTokens        int64     `gorm:"not null;default:0;comment:输出Token"`
	ReasoningTokens     int64     `gorm:"not null;default:0;comment:推理Token"`
	CallCount           int64     `gorm:"not null;default:0;comment:调用次数"`
	DurationSeconds     int64     `gorm:"not null;default:0;comment:计费时长秒数"`
	LatencyMS           int64     `gorm:"not null;default:0;comment:调用耗时毫秒"`
	UsageSpeed          string    `gorm:"size:32;not null;default:'';comment:计费速度档位"`
	ServiceTier         string    `gorm:"size:32;not null;default:'';comment:计费服务等级"`
	BilledCurrency      string    `gorm:"size:16;not null;default:'USD';comment:计费币种"`
	BilledNanousd       int64     `gorm:"not null;default:0;comment:账单金额(纳美元)"`
	PricingSnapshotJSON string    `gorm:"type:text;not null;default:'';comment:计费快照JSON"`
}

// TableName 指定表名。
func (UsageLedger) TableName() string {
	return "billing_usage_ledgers"
}
