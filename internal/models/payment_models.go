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
	PaymentMethodAlipay     PaymentMethod = "ALIPAY"
	PaymentMethodWeChat     PaymentMethod = "WECHAT"
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
	AlipayPayment *AlipayPayment `gorm:"foreignKey:OrderID" json:"alipay_payment,omitempty"`
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

// AlipayPayment 支付宝支付详情
type AlipayPayment struct {
	ID                    uint       `gorm:"primarykey" json:"id"`
	OrderID               uint       `gorm:"not null;uniqueIndex" json:"order_id"`                           // 订单ID
	OutTradeNo            string     `gorm:"not null;uniqueIndex;size:64" json:"out_trade_no"`              // 商户订单号
	TradeNo               string     `gorm:"size:64;index" json:"trade_no,omitempty"`                       // 支付宝交易号
	BuyerUserID           string     `gorm:"size:64;index" json:"buyer_user_id,omitempty"`                  // 买家支付宝用户ID
	BuyerLogonID          string     `gorm:"size:100" json:"buyer_logon_id,omitempty"`                      // 买家支付宝账号
	TotalAmount           string     `gorm:"size:20" json:"total_amount"`                                   // 订单金额
	ReceiptAmount         string     `gorm:"size:20" json:"receipt_amount,omitempty"`                       // 实收金额
	InvoiceAmount         string     `gorm:"size:20" json:"invoice_amount,omitempty"`                       // 开票金额
	BuyerPayAmount        string     `gorm:"size:20" json:"buyer_pay_amount,omitempty"`                     // 买家付款金额
	PointAmount           string     `gorm:"size:20" json:"point_amount,omitempty"`                         // 积分宝金额
	RefundFee             string     `gorm:"size:20" json:"refund_fee,omitempty"`                           // 总退款金额
	Subject               string     `gorm:"size:256" json:"subject"`                                       // 订单标题
	Body                  string     `gorm:"size:400" json:"body,omitempty"`                                // 订单描述
	TradeStatus           string     `gorm:"size:32;index" json:"trade_status,omitempty"`                   // 交易状态
	PaymentMethod         string     `gorm:"size:32" json:"payment_method,omitempty"`                       // 付款方式
	FundBillList          JSON       `gorm:"type:jsonb" json:"fund_bill_list,omitempty"`                    // 资金明细信息
	VoucherDetailList     JSON       `gorm:"type:jsonb" json:"voucher_detail_list,omitempty"`               // 优惠券信息
	AuthTradePayMode      string     `gorm:"size:32" json:"auth_trade_pay_mode,omitempty"`                  // 预授权支付模式
	CreditBizOrderID      string     `gorm:"size:64" json:"credit_biz_order_id,omitempty"`                  // 信用业务订单号
	CreditPayMode         string     `gorm:"size:32" json:"credit_pay_mode,omitempty"`                      // 信用支付模式
	CreditPhaseInfo       JSON       `gorm:"type:jsonb" json:"credit_phase_info,omitempty"`                 // 信用支付阶段信息
	StoreID               string     `gorm:"size:32" json:"store_id,omitempty"`                             // 商户门店编号
	TerminalID            string     `gorm:"size:32" json:"terminal_id,omitempty"`                          // 商户机具终端编号
	MerchantOrderNo       string     `gorm:"size:32" json:"merchant_order_no,omitempty"`                    // 商户原始订单号
	BusinessParams        JSON       `gorm:"type:jsonb" json:"business_params,omitempty"`                   // 商户业务参数
	PromoParams             string     `gorm:"size:512" json:"promo_params,omitempty"`                        // 优惠参数
	SendPayDate           *time.Time `json:"send_pay_date,omitempty"`                                        // 付款时间
	TimeoutExpress        string     `gorm:"size:32" json:"timeout_express,omitempty"`                      // 超时时间
	TimeEnd               *time.Time `json:"time_end,omitempty"`                                             // 交易付款时间
	NotifyTime            *time.Time `json:"notify_time,omitempty"`                                          // 通知时间
	NotifyType            string     `gorm:"size:64" json:"notify_type,omitempty"`                          // 通知类型
	NotifyID              string     `gorm:"size:128" json:"notify_id,omitempty"`                           // 通知校验ID
	AppID                 string     `gorm:"size:32;index" json:"app_id,omitempty"`                         // 应用ID
	Charset               string     `gorm:"size:10" json:"charset,omitempty"`                              // 编码格式
	Version               string     `gorm:"size:10" json:"version,omitempty"`                              // 接口版本
	SignType              string     `gorm:"size:10" json:"sign_type,omitempty"`                            // 签名类型
	Sign                  string     `gorm:"size:256" json:"sign,omitempty"`                                // 签名
	PassbackParams        string     `gorm:"size:512" json:"passback_params,omitempty"`                     // 回传参数
	ExtraCommonParam      string     `gorm:"size:256" json:"extra_common_param,omitempty"`                  // 公用回传参数
	AgreementNo           string     `gorm:"size:64" json:"agreement_no,omitempty"`                         // 协议号
	OutRequestNo          string     `gorm:"size:64" json:"out_request_no,omitempty"`                       // 外部请求号
	OperationID           string     `gorm:"size:64" json:"operation_id,omitempty"`                         // 操作ID
	RetryFlag             string     `gorm:"size:1" json:"retry_flag,omitempty"`                            // 重试标志
	ErrorCode             string     `gorm:"size:32" json:"error_code,omitempty"`                           // 错误码
	ErrorMsg              string     `gorm:"size:256" json:"error_msg,omitempty"`                           // 错误描述
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`

	// 关联
	Order Order `gorm:"foreignKey:OrderID" json:"order,omitempty"`
}

// AlipayRefund 支付宝退款记录
type AlipayRefund struct {
	ID               uint       `gorm:"primarykey" json:"id"`
	OrderID          uint       `gorm:"not null;index" json:"order_id"`                      // 订单ID
	AlipayPaymentID  uint       `gorm:"not null;index" json:"alipay_payment_id"`             // 支付宝支付记录ID
	OutRequestNo     string     `gorm:"not null;uniqueIndex;size:64" json:"out_request_no"` // 退款请求号
	OutTradeNo       string     `gorm:"not null;index;size:64" json:"out_trade_no"`         // 商户订单号
	TradeNo          string     `gorm:"size:64;index" json:"trade_no,omitempty"`            // 支付宝交易号
	RefundAmount     string     `gorm:"not null;size:20" json:"refund_amount"`              // 退款金额
	TotalAmount      string     `gorm:"size:20" json:"total_amount,omitempty"`              // 订单金额
	Currency         string     `gorm:"size:3" json:"currency,omitempty"`                   // 币种
	RefundReason     string     `gorm:"size:256" json:"refund_reason,omitempty"`            // 退款原因
	RefundStatus     string     `gorm:"size:32;index" json:"refund_status,omitempty"`       // 退款状态
	RefundCurrency   string     `gorm:"size:3" json:"refund_currency,omitempty"`            // 退款币种
	GmtRefundPay     *time.Time `json:"gmt_refund_pay,omitempty"`                            // 退款支付时间
	PresentRefundBuyerAmount string `gorm:"size:20" json:"present_refund_buyer_amount,omitempty"` // 退回买家的金额
	PresentRefundDiscountAmount string `gorm:"size:20" json:"present_refund_discount_amount,omitempty"` // 退回优惠金额
	PresentRefundMdiscountAmount string `gorm:"size:20" json:"present_refund_mdiscount_amount,omitempty"` // 退回商家优惠金额
	HasDepositBack   string     `gorm:"size:1" json:"has_deposit_back,omitempty"`           // 是否有银行卡冲退
	DepositBackInfo  JSON       `gorm:"type:jsonb" json:"deposit_back_info,omitempty"`      // 银行卡冲退信息
	RefundChargeInfo JSON      `gorm:"type:jsonb" json:"refund_charge_info,omitempty"`     // 退款手续费信息
	ErrorCode        string     `gorm:"size:32" json:"error_code,omitempty"`                // 错误码
	ErrorMsg         string     `gorm:"size:256" json:"error_msg,omitempty"`                // 错误描述
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// 关联
	Order        Order        `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	AlipayPayment AlipayPayment `gorm:"foreignKey:AlipayPaymentID" json:"alipay_payment,omitempty"`
}

// JSON 自定义JSON类型
type JSON map[string]interface{}

// TableName 设置表名
func (JSON) GormDataType() string {
	return "jsonb"
}
