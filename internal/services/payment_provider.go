package services

import (
	"context"
	"errors"
	"time"

	"pay-gateway/internal/models"
)

// PaymentProvider 统一的支付提供商接口
// 定义所有支付方式需要实现的基础功能
type PaymentProvider interface {
	// GetProviderName 获取支付提供商名称
	GetProviderName() string

	// CreateOrder 创建订单
	CreateOrder(ctx context.Context, req *UnifiedOrderRequest) (*UnifiedOrderResponse, error)

	// CreatePayment 创建支付
	CreatePayment(ctx context.Context, orderNo string, paymentReq interface{}) (interface{}, error)

	// QueryOrder 查询订单状态
	QueryOrder(ctx context.Context, orderNo string) (*UnifiedOrderQueryResponse, error)

	// Refund 退款
	Refund(ctx context.Context, req *UnifiedRefundRequest) (*UnifiedRefundResponse, error)

	// CloseOrder 关闭订单
	CloseOrder(ctx context.Context, orderNo string) error

	// HandleNotify 处理支付通知
	HandleNotify(ctx context.Context, notifyData interface{}) error

	// VerifyPayment 验证支付（用于客户端验证）
	VerifyPayment(ctx context.Context, verifyReq interface{}) (interface{}, error)
}

// UnifiedOrderRequest 统一的订单创建请求
type UnifiedOrderRequest struct {
	UserID      uint                   `json:"user_id"`
	ProductID   string                 `json:"product_id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Amount      int64                  `json:"amount"`     // 金额（分/微单位）
	Currency    string                 `json:"currency"`   // 货币代码
	OrderType   models.OrderType       `json:"order_type"` // 订单类型
	Extra       map[string]interface{} `json:"extra"`      // 额外参数
}

// UnifiedOrderResponse 统一的订单创建响应
type UnifiedOrderResponse struct {
	OrderID   uint                   `json:"order_id"`
	OrderNo   string                 `json:"order_no"`
	Amount    int64                  `json:"amount"`
	Currency  string                 `json:"currency"`
	Status    models.OrderStatus     `json:"status"`
	CreatedAt time.Time              `json:"created_at"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// UnifiedOrderQueryResponse 统一的订单查询响应
type UnifiedOrderQueryResponse struct {
	OrderNo       string                 `json:"order_no"`
	TransactionID string                 `json:"transaction_id"`
	Amount        int64                  `json:"amount"`
	Currency      string                 `json:"currency"`
	Status        models.OrderStatus     `json:"status"`
	PaymentStatus models.PaymentStatus   `json:"payment_status"`
	PaidAt        *time.Time             `json:"paid_at,omitempty"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// UnifiedRefundRequest 统一的退款请求
type UnifiedRefundRequest struct {
	OrderNo      string `json:"order_no"`
	RefundAmount int64  `json:"refund_amount"`
	RefundReason string `json:"refund_reason"`
	RefundNo     string `json:"refund_no,omitempty"` // 退款单号，可选
}

// UnifiedRefundResponse 统一的退款响应
type UnifiedRefundResponse struct {
	RefundNo     string                 `json:"refund_no"`
	OrderNo      string                 `json:"order_no"`
	RefundAmount int64                  `json:"refund_amount"`
	Status       string                 `json:"status"`
	RefundedAt   *time.Time             `json:"refunded_at,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// PaymentProviderRegistry 支付提供商注册表
type PaymentProviderRegistry struct {
	providers map[models.PaymentProvider]PaymentProvider
}

// NewPaymentProviderRegistry 创建支付提供商注册表
func NewPaymentProviderRegistry() *PaymentProviderRegistry {
	return &PaymentProviderRegistry{
		providers: make(map[models.PaymentProvider]PaymentProvider),
	}
}

// Register 注册支付提供商
func (r *PaymentProviderRegistry) Register(providerType models.PaymentProvider, provider PaymentProvider) {
	r.providers[providerType] = provider
}

// GetProvider 获取支付提供商
func (r *PaymentProviderRegistry) GetProvider(providerType models.PaymentProvider) (PaymentProvider, error) {
	provider, exists := r.providers[providerType]
	if !exists {
		return nil, errors.New("不支持的支付提供商: " + string(providerType))
	}
	return provider, nil
}

// GetAllProviders 获取所有支付提供商
func (r *PaymentProviderRegistry) GetAllProviders() map[models.PaymentProvider]PaymentProvider {
	return r.providers
}

// IsSupported 检查是否支持该支付提供商
func (r *PaymentProviderRegistry) IsSupported(providerType models.PaymentProvider) bool {
	_, exists := r.providers[providerType]
	return exists
}

// WechatPaymentAdapter 微信支付适配器
// 将WechatService适配为PaymentProvider接口
type WechatPaymentAdapter struct {
	service *WechatService
}

// NewWechatPaymentAdapter 创建微信支付适配器
func NewWechatPaymentAdapter(service *WechatService) *WechatPaymentAdapter {
	return &WechatPaymentAdapter{service: service}
}

// GetProviderName 实现PaymentProvider接口
func (a *WechatPaymentAdapter) GetProviderName() string {
	return "WeChat Pay"
}

// CreateOrder 实现PaymentProvider接口
func (a *WechatPaymentAdapter) CreateOrder(ctx context.Context, req *UnifiedOrderRequest) (*UnifiedOrderResponse, error) {
	tradeType := "JSAPI" // 默认类型
	if req.Extra != nil {
		if t, ok := req.Extra["trade_type"].(string); ok {
			tradeType = t
		}
	}

	wechatReq := &CreateWechatOrderRequest{
		UserID:      req.UserID,
		ProductID:   req.ProductID,
		Description: req.Title,
		Detail:      req.Description,
		TotalAmount: req.Amount,
		TradeType:   tradeType,
	}

	resp, err := a.service.CreateOrder(ctx, wechatReq)
	if err != nil {
		return nil, err
	}

	return &UnifiedOrderResponse{
		OrderID:   resp.OrderID,
		OrderNo:   resp.OrderNo,
		Amount:    resp.TotalAmount,
		Currency:  "CNY",
		Status:    models.OrderStatusCreated,
		CreatedAt: time.Now(),
	}, nil
}

// CreatePayment 实现PaymentProvider接口
func (a *WechatPaymentAdapter) CreatePayment(ctx context.Context, orderNo string, paymentReq interface{}) (interface{}, error) {
	// 根据paymentReq的类型，调用不同的支付方式
	if req, ok := paymentReq.(map[string]interface{}); ok {
		paymentType, _ := req["payment_type"].(string)
		switch paymentType {
		case "JSAPI":
			openID, _ := req["openid"].(string)
			return a.service.CreateJSAPIPayment(ctx, orderNo, openID)
		case "NATIVE":
			return a.service.CreateNativePayment(ctx, orderNo)
		case "APP":
			return a.service.CreateAPPPayment(ctx, orderNo)
		case "MWEB":
			sceneInfo, _ := req["scene_info"].(map[string]interface{})
			return a.service.CreateH5Payment(ctx, orderNo, sceneInfo)
		}
	}
	return nil, errors.New("不支持的支付类型")
}

// QueryOrder 实现PaymentProvider接口
func (a *WechatPaymentAdapter) QueryOrder(ctx context.Context, orderNo string) (*UnifiedOrderQueryResponse, error) {
	resp, err := a.service.QueryOrder(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	return &UnifiedOrderQueryResponse{
		OrderNo:       resp.OrderNo,
		TransactionID: resp.TransactionID,
		Amount:        resp.TotalAmount,
		Currency:      "CNY",
		PaymentStatus: resp.PaymentStatus,
		PaidAt:        resp.PaidAt,
		Extra: map[string]interface{}{
			"trade_state": resp.TradeState,
		},
	}, nil
}

// Refund 实现PaymentProvider接口
func (a *WechatPaymentAdapter) Refund(ctx context.Context, req *UnifiedRefundRequest) (*UnifiedRefundResponse, error) {
	wechatReq := &WechatRefundRequest{
		OrderNo:      req.OrderNo,
		RefundAmount: req.RefundAmount,
		RefundReason: req.RefundReason,
	}

	resp, err := a.service.Refund(ctx, wechatReq)
	if err != nil {
		return nil, err
	}

	return &UnifiedRefundResponse{
		RefundNo:     resp.OutRefundNo,
		OrderNo:      req.OrderNo,
		RefundAmount: resp.RefundAmount,
		Status:       resp.RefundStatus,
		RefundedAt:   resp.RefundAt,
		Extra: map[string]interface{}{
			"refund_id": resp.RefundID,
		},
	}, nil
}

// CloseOrder 实现PaymentProvider接口
func (a *WechatPaymentAdapter) CloseOrder(ctx context.Context, orderNo string) error {
	return a.service.CloseOrder(ctx, orderNo)
}

// HandleNotify 实现PaymentProvider接口
func (a *WechatPaymentAdapter) HandleNotify(ctx context.Context, notifyData interface{}) error {
	data, ok := notifyData.(map[string]interface{})
	if !ok {
		return errors.New("无效的通知数据格式")
	}
	return a.service.HandleNotify(ctx, data)
}

// VerifyPayment 实现PaymentProvider接口
func (a *WechatPaymentAdapter) VerifyPayment(ctx context.Context, verifyReq interface{}) (interface{}, error) {
	// 微信支付不需要客户端验证
	return nil, errors.New("微信支付不支持客户端验证")
}

// AlipayPaymentAdapter 支付宝支付适配器
type AlipayPaymentAdapter struct {
	service *AlipayService
}

// NewAlipayPaymentAdapter 创建支付宝支付适配器
func NewAlipayPaymentAdapter(service *AlipayService) *AlipayPaymentAdapter {
	return &AlipayPaymentAdapter{service: service}
}

// GetProviderName 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) GetProviderName() string {
	return "Alipay"
}

// CreateOrder 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) CreateOrder(ctx context.Context, req *UnifiedOrderRequest) (*UnifiedOrderResponse, error) {
	alipayReq := &CreateAlipayOrderRequest{
		UserID:      req.UserID,
		ProductID:   req.ProductID,
		Subject:     req.Title,
		Body:        req.Description,
		TotalAmount: req.Amount,
	}

	resp, err := a.service.CreateOrder(ctx, alipayReq)
	if err != nil {
		return nil, err
	}

	return &UnifiedOrderResponse{
		OrderID:   resp.OrderID,
		OrderNo:   resp.OrderNo,
		Amount:    resp.TotalAmount,
		Currency:  "CNY",
		Status:    models.OrderStatusCreated,
		CreatedAt: time.Now(),
	}, nil
}

// CreatePayment 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) CreatePayment(ctx context.Context, orderNo string, paymentReq interface{}) (interface{}, error) {
	// 根据paymentReq的类型，调用不同的支付方式
	if req, ok := paymentReq.(map[string]interface{}); ok {
		paymentType, _ := req["payment_type"].(string)
		switch paymentType {
		case "wap":
			return a.service.CreateWapPayment(ctx, orderNo)
		case "page":
			return a.service.CreatePagePayment(ctx, orderNo)
		default:
			// 默认使用wap支付
			return a.service.CreateWapPayment(ctx, orderNo)
		}
	}
	return a.service.CreateWapPayment(ctx, orderNo)
}

// QueryOrder 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) QueryOrder(ctx context.Context, orderNo string) (*UnifiedOrderQueryResponse, error) {
	resp, err := a.service.QueryOrder(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	return &UnifiedOrderQueryResponse{
		OrderNo:       resp.OrderNo,
		TransactionID: resp.TradeNo,
		Amount:        resp.TotalAmount,
		Currency:      "CNY",
		PaymentStatus: resp.PaymentStatus,
		PaidAt:        resp.PaidAt,
		Extra: map[string]interface{}{
			"trade_status": resp.TradeStatus,
		},
	}, nil
}

// Refund 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) Refund(ctx context.Context, req *UnifiedRefundRequest) (*UnifiedRefundResponse, error) {
	alipayReq := &RefundRequest{
		OrderNo:      req.OrderNo,
		RefundAmount: req.RefundAmount,
		RefundReason: req.RefundReason,
	}

	resp, err := a.service.Refund(ctx, alipayReq)
	if err != nil {
		return nil, err
	}

	return &UnifiedRefundResponse{
		RefundNo:     resp.RefundRequestNo,
		OrderNo:      req.OrderNo,
		RefundAmount: resp.RefundAmount,
		Status:       resp.RefundStatus,
		RefundedAt:   resp.RefundAt,
	}, nil
}

// CloseOrder 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) CloseOrder(ctx context.Context, orderNo string) error {
	// 支付宝不需要主动关闭订单，超时自动关闭
	return nil
}

// HandleNotify 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) HandleNotify(ctx context.Context, notifyData interface{}) error {
	data, ok := notifyData.(map[string]string)
	if !ok {
		return errors.New("无效的通知数据格式")
	}
	return a.service.HandleNotify(ctx, data)
}

// VerifyPayment 实现PaymentProvider接口
func (a *AlipayPaymentAdapter) VerifyPayment(ctx context.Context, verifyReq interface{}) (interface{}, error) {
	// 支付宝不需要客户端验证
	return nil, errors.New("支付宝不支持客户端验证")
}

// ApplePaymentAdapter Apple支付适配器
type ApplePaymentAdapter struct {
	service *AppleService
}

// NewApplePaymentAdapter 创建Apple支付适配器
func NewApplePaymentAdapter(service *AppleService) *ApplePaymentAdapter {
	return &ApplePaymentAdapter{service: service}
}

// GetProviderName 实现PaymentProvider接口
func (a *ApplePaymentAdapter) GetProviderName() string {
	return "Apple In-App Purchase"
}

// CreateOrder 实现PaymentProvider接口
func (a *ApplePaymentAdapter) CreateOrder(ctx context.Context, req *UnifiedOrderRequest) (*UnifiedOrderResponse, error) {
	// Apple IAP不需要服务端创建订单，订单由客户端发起
	return nil, errors.New("Apple IAP订单由客户端创建")
}

// CreatePayment 实现PaymentProvider接口
func (a *ApplePaymentAdapter) CreatePayment(ctx context.Context, orderNo string, paymentReq interface{}) (interface{}, error) {
	// Apple IAP不需要服务端创建支付
	return nil, errors.New("Apple IAP支付由客户端处理")
}

// QueryOrder 实现PaymentProvider接口
func (a *ApplePaymentAdapter) QueryOrder(ctx context.Context, orderNo string) (*UnifiedOrderQueryResponse, error) {
	// Apple IAP查询通过交易ID进行
	return nil, errors.New("Apple IAP查询需要使用交易ID")
}

// Refund 实现PaymentProvider接口
func (a *ApplePaymentAdapter) Refund(ctx context.Context, req *UnifiedRefundRequest) (*UnifiedRefundResponse, error) {
	// Apple IAP退款需要通过App Store Connect处理
	return nil, errors.New("Apple IAP退款需要通过App Store Connect处理")
}

// CloseOrder 实现PaymentProvider接口
func (a *ApplePaymentAdapter) CloseOrder(ctx context.Context, orderNo string) error {
	// Apple IAP不需要关闭订单
	return nil
}

// HandleNotify 实现PaymentProvider接口
func (a *ApplePaymentAdapter) HandleNotify(ctx context.Context, notifyData interface{}) error {
	// Apple Server Notification处理
	return errors.New("Apple通知处理需要使用专门的webhook handler")
}

// VerifyPayment 实现PaymentProvider接口
func (a *ApplePaymentAdapter) VerifyPayment(ctx context.Context, verifyReq interface{}) (interface{}, error) {
	// 验证Apple收据或交易
	if req, ok := verifyReq.(map[string]interface{}); ok {
		if receiptData, exists := req["receipt_data"].(string); exists {
			orderID, _ := req["order_id"].(uint)
			return a.service.VerifyPurchase(ctx, receiptData, orderID)
		}
		if transactionID, exists := req["transaction_id"].(string); exists {
			return a.service.VerifyTransaction(ctx, transactionID)
		}
	}
	return nil, errors.New("无效的验证请求")
}

// GooglePlayPaymentAdapter Google Play支付适配器
type GooglePlayPaymentAdapter struct {
	service *GooglePlayService
}

// NewGooglePlayPaymentAdapter 创建Google Play支付适配器
func NewGooglePlayPaymentAdapter(service *GooglePlayService) *GooglePlayPaymentAdapter {
	return &GooglePlayPaymentAdapter{service: service}
}

// GetProviderName 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) GetProviderName() string {
	return "Google Play"
}

// CreateOrder 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) CreateOrder(ctx context.Context, req *UnifiedOrderRequest) (*UnifiedOrderResponse, error) {
	// Google Play IAP不需要服务端创建订单
	return nil, errors.New("Google Play IAP订单由客户端创建")
}

// CreatePayment 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) CreatePayment(ctx context.Context, orderNo string, paymentReq interface{}) (interface{}, error) {
	// Google Play IAP不需要服务端创建支付
	return nil, errors.New("Google Play IAP支付由客户端处理")
}

// QueryOrder 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) QueryOrder(ctx context.Context, orderNo string) (*UnifiedOrderQueryResponse, error) {
	// Google Play查询通过购买令牌进行
	return nil, errors.New("Google Play查询需要使用购买令牌")
}

// Refund 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) Refund(ctx context.Context, req *UnifiedRefundRequest) (*UnifiedRefundResponse, error) {
	// Google Play退款需要通过Google Play Console处理
	return nil, errors.New("Google Play退款需要通过Google Play Console处理")
}

// CloseOrder 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) CloseOrder(ctx context.Context, orderNo string) error {
	// Google Play不需要关闭订单
	return nil
}

// HandleNotify 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) HandleNotify(ctx context.Context, notifyData interface{}) error {
	// Google Play Real-time Developer Notifications处理
	return errors.New("Google Play通知处理需要使用专门的webhook handler")
}

// VerifyPayment 实现PaymentProvider接口
func (g *GooglePlayPaymentAdapter) VerifyPayment(ctx context.Context, verifyReq interface{}) (interface{}, error) {
	// 验证Google Play购买或订阅
	if req, ok := verifyReq.(map[string]interface{}); ok {
		productID, _ := req["product_id"].(string)
		purchaseToken, _ := req["purchase_token"].(string)
		purchaseType, _ := req["purchase_type"].(string)

		if purchaseType == "subscription" {
			return g.service.VerifySubscription(ctx, productID, purchaseToken)
		}
		return g.service.VerifyPurchase(ctx, productID, purchaseToken)
	}
	return nil, errors.New("无效的验证请求")
}
