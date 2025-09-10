package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentProvider 支付提供商类型
type PaymentProvider string

const (
	PaymentProviderGooglePlay PaymentProvider = "GOOGLE_PLAY"
	PaymentProviderAppleStore PaymentProvider = "APPLE_STORE"
	PaymentProviderAlipay     PaymentProvider = "ALIPAY"
	PaymentProviderWeChat     PaymentProvider = "WECHAT"
)

// PaymentStatus 支付状态
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusCompleted PaymentStatus = "COMPLETED"
	PaymentStatusFailed    PaymentStatus = "FAILED"
	PaymentStatusCancelled PaymentStatus = "CANCELLED"
	PaymentStatusRefunded  PaymentStatus = "REFUNDED"
	PaymentStatusExpired   PaymentStatus = "EXPIRED"
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "CREATED"
	OrderStatusPaid      OrderStatus = "PAID"
	OrderStatusDelivered OrderStatus = "DELIVERED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusRefunded  OrderStatus = "REFUNDED"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypePurchase     OrderType = "PURCHASE"
	OrderTypeSubscription OrderType = "SUBSCRIPTION"
)

// PaymentMethod 支付方式
type PaymentMethod string

const (
	PaymentMethodGooglePlay PaymentMethod = "GOOGLE_PLAY"
)

// Order 订单主表
type Order struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	OrderNo          string         `gorm:"uniqueIndex;not null;size:32" json:"order_no"` // 系统订单号
	UserID           uint           `gorm:"not null;index" json:"user_id"`                // 用户ID
	ProductID        string         `gorm:"not null;index;size:100" json:"product_id"`    // 商品ID
	Type             OrderType      `gorm:"not null;index" json:"type"`                   // 订单类型
	Title            string         `gorm:"not null;size:200" json:"title"`               // 商品标题
	Description      string         `gorm:"size:500" json:"description"`                  // 商品描述
	Quantity         int            `gorm:"not null;default:1" json:"quantity"`           // 数量
	Currency         string         `gorm:"not null;size:3" json:"currency"`              // 货币代码
	TotalAmount      int64          `gorm:"not null" json:"total_amount"`                 // 总金额（微单位）
	Status           OrderStatus    `gorm:"not null;index" json:"status"`                 // 订单状态
	PaymentMethod    PaymentMethod  `gorm:"not null;index" json:"payment_method"`         // 支付方式
	PaymentStatus    PaymentStatus  `gorm:"not null;index" json:"payment_status"`         // 支付状态
	PaidAt           *time.Time     `json:"paid_at,omitempty"`                            // 支付时间
	ExpiredAt        *time.Time     `json:"expired_at,omitempty"`                         // 过期时间
	RefundAt         *time.Time     `json:"refund_at,omitempty"`                          // 退款时间
	RefundReason     string         `gorm:"size:500" json:"refund_reason,omitempty"`      // 退款原因
	RefundAmount     int64          `json:"refund_amount,omitempty"`                      // 退款金额
	DeveloperPayload string         `gorm:"size:500" json:"developer_payload,omitempty"`  // 开发者透传数据
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	User          User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	GooglePayment *GooglePayment `gorm:"foreignKey:OrderID" json:"google_payment,omitempty"`
}

// GooglePayment Google支付详情
type GooglePayment struct {
	ID                      uint       `gorm:"primarykey" json:"id"`
	OrderID                 uint       `gorm:"not null;uniqueIndex" json:"order_id"`                // 订单ID
	PurchaseToken           string     `gorm:"not null;uniqueIndex;size:255" json:"purchase_token"` // Google购买令牌
	OrderIDGoogle           string     `gorm:"not null;index;size:100" json:"order_id_google"`      // Google订单号
	ProductIDGoogle         string     `gorm:"not null;index;size:100" json:"product_id_google"`    // Google商品ID
	PurchaseState           int        `json:"purchase_state"`                                      // 购买状态
	ConsumptionState        int        `json:"consumption_state"`                                   // 消费状态
	AcknowledgementState    int        `json:"acknowledgement_state"`                               // 确认状态
	PurchaseTimeMillis      string     `gorm:"size:20" json:"purchase_time_millis"`                 // 购买时间（毫秒）
	ObfuscatedAccountID     string     `gorm:"size:100" json:"obfuscated_account_id,omitempty"`     // 混淆账户ID
	ObfuscatedProfileID     string     `gorm:"size:100" json:"obfuscated_profile_id,omitempty"`     // 混淆档案ID
	RegionCode              string     `gorm:"size:10" json:"region_code,omitempty"`                // 地区代码
	CountryCode             string     `gorm:"size:10" json:"country_code,omitempty"`               // 国家代码
	PriceAmountMicros       string     `gorm:"size:20" json:"price_amount_micros"`                  // 价格（微单位）
	AutoRenewing            *bool      `json:"auto_renewing,omitempty"`                             // 自动续订
	CancelReason            *int       `json:"cancel_reason,omitempty"`                             // 取消原因
	UserCancellationTime    *time.Time `json:"user_cancellation_time,omitempty"`                    // 用户取消时间
	ExpiryTimeMillis        string     `gorm:"size:20" json:"expiry_time_millis,omitempty"`         // 到期时间（毫秒）
	GracePeriodExpiryTime   *time.Time `json:"grace_period_expiry_time,omitempty"`                  // 宽限期到期时间
	AutoResumeTimeMillis    string     `gorm:"size:20" json:"auto_resume_time_millis,omitempty"`    // 自动恢复时间
	IntroductoryPrice       *int64     `json:"introductory_price,omitempty"`                        // 介绍价格
	IntroductoryPricePeriod *string    `gorm:"size:50" json:"introductory_price_period,omitempty"`  // 介绍价格周期
	IntroductoryPriceCycles *int       `json:"introductory_price_cycles,omitempty"`                 // 介绍价格周期数
	PromoCode               *string    `gorm:"size:50" json:"promo_code,omitempty"`                 // 促销代码
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`

	// 关联
	Order Order `gorm:"foreignKey:OrderID" json:"order,omitempty"`
}

// PaymentTransaction 支付交易记录
type PaymentTransaction struct {
	ID            uint            `gorm:"primarykey" json:"id"`
	OrderID       uint            `gorm:"not null;index" json:"order_id"`                      // 订单ID
	TransactionID string          `gorm:"uniqueIndex;not null;size:100" json:"transaction_id"` // 交易ID
	Provider      PaymentProvider `gorm:"not null;index" json:"provider"`                      // 支付提供商
	Type          string          `gorm:"not null;index;size:50" json:"type"`                  // 交易类型
	Amount        int64           `gorm:"not null" json:"amount"`                              // 交易金额
	Currency      string          `gorm:"not null;size:3" json:"currency"`                     // 货币代码
	Status        PaymentStatus   `gorm:"not null;index" json:"status"`                        // 交易状态
	ProviderData  JSON            `gorm:"type:jsonb" json:"provider_data,omitempty"`           // 提供商数据
	ErrorCode     *string         `gorm:"size:50" json:"error_code,omitempty"`                 // 错误代码
	ErrorMessage  *string         `gorm:"size:500" json:"error_message,omitempty"`             // 错误信息
	ProcessedAt   *time.Time      `json:"processed_at,omitempty"`                              // 处理时间
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`

	// 关联
	Order Order `gorm:"foreignKey:OrderID" json:"order,omitempty"`
}

// UserBalance 用户余额（可选，用于存储用户余额信息）
type UserBalance struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null;uniqueIndex" json:"user_id"` // 用户ID
	Balance       int64     `gorm:"not null;default:0" json:"balance"`   // 余额（微单位）
	Currency      string    `gorm:"not null;size:3" json:"currency"`     // 货币代码
	LastUpdatedAt time.Time `json:"last_updated_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 关联
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// JSON 自定义JSON类型
type JSON map[string]interface{}

// TableName 设置表名
func (JSON) GormDataType() string {
	return "jsonb"
}
