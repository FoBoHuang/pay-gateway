package models

import (
	"time"

	"gorm.io/gorm"
)

// PurchaseState 购买状态枚举类型
type PurchaseState string

const (
	PurchaseStatePending   PurchaseState = "PENDING"
	PurchaseStatePurchased PurchaseState = "PURCHASED"
	PurchaseStateCancelled PurchaseState = "CANCELLED"
	PurchaseStateRefunded  PurchaseState = "REFUNDED"
	PurchaseStateExpired   PurchaseState = "EXPIRED"
)

// SubscriptionState 订阅状态枚举类型
type SubscriptionState string

const (
	SubscriptionStateActive        SubscriptionState = "ACTIVE"
	SubscriptionStateCancelled     SubscriptionState = "CANCELLED"
	SubscriptionStateExpired       SubscriptionState = "EXPIRED"
	SubscriptionStateOnHold        SubscriptionState = "ON_HOLD"
	SubscriptionStatePaused        SubscriptionState = "PAUSED"
	SubscriptionStatePending       SubscriptionState = "PENDING"
	SubscriptionStateInGracePeriod SubscriptionState = "IN_GRACE_PERIOD"
)

// ProductType 产品类型枚举类型
type ProductType string

const (
	ProductTypeInApp        ProductType = "INAPP"
	ProductTypeSubscription ProductType = "SUBSCRIPTION"
)

// User 用户模型
// 使用UUID作为主键，支持软删除
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UUID      string         `gorm:"uniqueIndex;not null" json:"uuid"`
	Email     string         `gorm:"uniqueIndex" json:"email"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Purchases     []Purchase     `gorm:"foreignKey:UserID" json:"purchases,omitempty"`
	Subscriptions []Subscription `gorm:"foreignKey:UserID" json:"subscriptions,omitempty"`
}

// Product 商品模型
// 对应Google Play Console中配置的商品信息
type Product struct {
	ID          uint        `gorm:"primarykey" json:"id"`
	ProductID   string      `gorm:"uniqueIndex;not null" json:"product_id"` // Google Play product ID
	Type        ProductType `gorm:"not null" json:"type"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Price       int64       `json:"price"` // Price in micros (1,000,000 = 1 USD)
	Currency    string      `json:"currency"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Purchase 购买记录模型
// 记录用户的单次购买行为，包含购买状态和详细信息
type Purchase struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	UserID            uint           `gorm:"not null;index" json:"user_id"`
	ProductID         string         `gorm:"not null;index" json:"product_id"`
	PurchaseToken     string         `gorm:"uniqueIndex;not null" json:"purchase_token"`
	OrderID           string         `gorm:"index" json:"order_id"`
	State             PurchaseState  `gorm:"not null;index" json:"state"`
	PurchaseTime      time.Time      `json:"purchase_time"`
	ConsumptionState  int            `json:"consumption_state"`
	DeveloperPayload  string         `json:"developer_payload"`
	ObfuscatedAccount string         `json:"obfuscated_account"`
	ObfuscatedProfile string         `json:"obfuscated_profile"`
	RegionCode        string         `json:"region_code"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Product Product `gorm:"foreignKey:ProductID;references:ProductID" json:"product,omitempty"`
}

// Subscription 订阅记录模型
// 记录用户的订阅购买行为，包含订阅状态和续费信息
type Subscription struct {
	ID                      uint              `gorm:"primarykey" json:"id"`
	UserID                  uint              `gorm:"not null;index" json:"user_id"`
	ProductID               string            `gorm:"not null;index" json:"product_id"`
	PurchaseToken           string            `gorm:"uniqueIndex;not null" json:"purchase_token"`
	OrderID                 string            `gorm:"index" json:"order_id"`
	State                   SubscriptionState `gorm:"not null;index" json:"state"`
	AutoRenewing            bool              `json:"auto_renewing"`
	Price                   int64             `json:"price"` // Price in micros
	Currency                string            `json:"currency"`
	Country                 string            `json:"country"`
	StartTime               time.Time         `json:"start_time"`
	ExpiryTime              time.Time         `json:"expiry_time"`
	GracePeriodExpiryTime   *time.Time        `json:"grace_period_expiry_time,omitempty"`
	CancelReason            *int              `json:"cancel_reason,omitempty"`
	UserCancellationTime    *time.Time        `json:"user_cancellation_time,omitempty"`
	AutoResumeTime          *time.Time        `json:"auto_resume_time,omitempty"`
	PromoCode               *string           `json:"promo_code,omitempty"`
	IntroductoryPrice       *int64            `json:"introductory_price,omitempty"`
	IntroductoryPricePeriod *string           `json:"introductory_price_period,omitempty"`
	IntroductoryPriceCycles *int              `json:"introductory_price_cycles,omitempty"`
	ObfuscatedAccount       string            `json:"obfuscated_account"`
	ObfuscatedProfile       string            `json:"obfuscated_profile"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
	DeletedAt               gorm.DeletedAt    `gorm:"index" json:"-"`

	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Product Product `gorm:"foreignKey:ProductID;references:ProductID" json:"product,omitempty"`
}

// WebhookType Webhook类型
type WebhookType string

const (
	WebhookTypeTest           WebhookType = "TEST"
	WebhookTypeOneTimeProduct WebhookType = "ONE_TIME_PRODUCT"
	WebhookTypeSubscription   WebhookType = "SUBSCRIPTION"
	WebhookTypeUnknown        WebhookType = "UNKNOWN"
)

// WebhookStatus Webhook处理状态
type WebhookStatus string

const (
	WebhookStatusPending   WebhookStatus = "PENDING"
	WebhookStatusProcessed WebhookStatus = "PROCESSED"
	WebhookStatusFailed    WebhookStatus = "FAILED"
	WebhookStatusSkipped   WebhookStatus = "SKIPPED"
)

// WebhookEvent Webhook事件模型
// 记录Google Play发送的Webhook通知事件
type WebhookEvent struct {
	ID                         uint                        `gorm:"primarykey" json:"id"`
	EventID                    string                      `gorm:"uniqueIndex;not null;size:100" json:"event_id"` // 事件ID
	Type                       WebhookType                 `gorm:"not null;index" json:"type"`                    // Webhook类型
	Version                    string                      `json:"version"`
	PackageName                string                      `gorm:"not null;index;size:100" json:"package_name"`
	EventTime                  int64                       `gorm:"not null;index" json:"event_time_millis"`
	Status                     WebhookStatus               `gorm:"not null;index;default:'PENDING'" json:"status"`
	RetryCount                 int                         `gorm:"not null;default:0" json:"retry_count"`
	MaxRetries                 int                         `gorm:"not null;default:3" json:"max_retries"`
	NextRetryAt                *time.Time                  `json:"next_retry_at,omitempty"`
	ProcessedAt                *time.Time                  `json:"processed_at,omitempty"`
	ErrorMessage               *string                     `gorm:"size:1000" json:"error_message,omitempty"`
	RawPayload                 JSON                        `gorm:"type:jsonb" json:"raw_payload,omitempty"`    // 原始数据
	ProcessedData              JSON                        `gorm:"type:jsonb" json:"processed_data,omitempty"` // 处理后的数据
	Processed                  bool                        `gorm:"index;default:false" json:"processed"`
	OneTimeProductNotification *OneTimeProductNotification `gorm:"embedded" json:"one_time_product_notification,omitempty"`
	SubscriptionNotification   *SubscriptionNotification   `gorm:"embedded" json:"subscription_notification,omitempty"`
	TestNotification           *TestNotification           `gorm:"embedded" json:"test_notification,omitempty"`
	CreatedAt                  time.Time                   `json:"created_at"`
	UpdatedAt                  time.Time                   `json:"updated_at"`
}

// OneTimeProductNotification 一次性商品通知结构
// 包含单次购买相关的通知信息
type OneTimeProductNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notification_type"`
	PurchaseToken    string `json:"purchase_token"`
	SKU              string `json:"sku"`
}

// SubscriptionNotification 订阅通知结构
// 包含订阅相关的通知信息
type SubscriptionNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notification_type"`
	PurchaseToken    string `json:"purchase_token"`
	SubscriptionID   string `json:"subscription_id"`
}

// TestNotification 测试通知结构
// 用于Google Play的Webhook测试
type TestNotification struct {
	Version string `json:"version"`
}

// Notification types for subscriptions
const (
	SubscriptionNotificationTypeRecovered            = 1
	SubscriptionNotificationTypeRenewed              = 2
	SubscriptionNotificationTypeCanceled             = 3
	SubscriptionNotificationTypePurchased            = 4
	SubscriptionNotificationTypeAccountHold          = 5
	SubscriptionNotificationTypeGracePeriod          = 6
	SubscriptionNotificationTypeRestarted            = 7
	SubscriptionNotificationTypePriceChangeConfirmed = 8
	SubscriptionNotificationTypeDeferred             = 9
	SubscriptionNotificationTypePaused               = 10
	SubscriptionNotificationTypePauseScheduleChanged = 11
	SubscriptionNotificationTypeRevoked              = 12
	SubscriptionNotificationTypeExpired              = 13
	SubscriptionNotificationTypeInGracePeriod        = 6
)

// Notification types for one-time purchases
const (
	OneTimeProductNotificationTypePurchased = 1
	OneTimeProductNotificationTypeCanceled  = 2
)

// WebhookEvent methods
// ShouldRetry 判断是否应该重试
func (w *WebhookEvent) ShouldRetry() bool {
	return w.Status == WebhookStatusFailed && w.RetryCount < w.MaxRetries
}

// WebhookRetryStrategy Webhook重试策略
type WebhookRetryStrategy struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	MaxRetries    int
}

// CalculateNextRetry 计算下次重试时间
func (w *WebhookEvent) CalculateNextRetry(strategy WebhookRetryStrategy) time.Time {
	if w.RetryCount >= strategy.MaxRetries {
		return time.Time{}
	}

	delay := strategy.InitialDelay
	for i := 0; i < w.RetryCount; i++ {
		delay = time.Duration(float64(delay) * strategy.BackoffFactor)
		if delay > strategy.MaxDelay {
			delay = strategy.MaxDelay
			break
		}
	}

	return time.Now().Add(delay)
}

// MarkAsProcessed 标记为已处理
func (w *WebhookEvent) MarkAsProcessed() {
	w.Status = WebhookStatusProcessed
	w.ProcessedAt = &time.Time{}
	*w.ProcessedAt = time.Now()
	w.Processed = true
}

// MarkAsFailed 标记为失败
func (w *WebhookEvent) MarkAsFailed(errorMsg string) {
	w.Status = WebhookStatusFailed
	w.ErrorMessage = &errorMsg
	w.RetryCount++
}

// IsSubscriptionEvent 判断是否为订阅事件
func (w *WebhookEvent) IsSubscriptionEvent() bool {
	return w.SubscriptionNotification != nil
}

// IsOneTimeProductEvent 判断是否为一次性产品事件
func (w *WebhookEvent) IsOneTimeProductEvent() bool {
	return w.OneTimeProductNotification != nil
}

// IsTestEvent 判断是否为测试事件
func (w *WebhookEvent) IsTestEvent() bool {
	return w.TestNotification != nil
}

// GetNotificationType 获取通知类型
func (w *WebhookEvent) GetNotificationType() int {
	if w.SubscriptionNotification != nil {
		return w.SubscriptionNotification.NotificationType
	}
	if w.OneTimeProductNotification != nil {
		return w.OneTimeProductNotification.NotificationType
	}
	return 0
}

// GetPurchaseToken 获取购买令牌
func (w *WebhookEvent) GetPurchaseToken() string {
	if w.SubscriptionNotification != nil {
		return w.SubscriptionNotification.PurchaseToken
	}
	if w.OneTimeProductNotification != nil {
		return w.OneTimeProductNotification.PurchaseToken
	}
	return ""
}

// GetProductID 获取产品ID
func (w *WebhookEvent) GetProductID() string {
	if w.SubscriptionNotification != nil {
		return w.SubscriptionNotification.SubscriptionID
	}
	if w.OneTimeProductNotification != nil {
		return w.OneTimeProductNotification.SKU
	}
	return ""
}
