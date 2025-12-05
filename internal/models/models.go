package models

import (
	"time"

	"gorm.io/gorm"
)

// SubscriptionState 订阅状态枚举类型（用于Google Play订阅）
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

// User 用户模型
// 简化的用户模型，用于关联订单
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UUID      string         `gorm:"uniqueIndex;not null" json:"uuid"`
	Email     string         `gorm:"uniqueIndex" json:"email"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
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
