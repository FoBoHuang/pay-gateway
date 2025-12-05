package services

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// WechatService 微信支付服务
type WechatService struct {
	db         *gorm.DB
	config     *config.WechatConfig
	logger     *zap.Logger
	privateKey *rsa.PrivateKey
}

// NewWechatService 创建微信支付服务实例
func NewWechatService(db *gorm.DB, cfg *config.WechatConfig, logger *zap.Logger) (*WechatService, error) {
	if cfg.MchID == "" || cfg.AppID == "" {
		return nil, errors.New("微信支付配置不完整")
	}

	// 解析私钥
	privateKey, err := parseWechatPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("解析微信私钥失败: %v", err)
	}

	return &WechatService{
		db:         db,
		config:     cfg,
		logger:     logger,
		privateKey: privateKey,
	}, nil
}

// CreateOrder 创建微信支付订单
func (s *WechatService) CreateOrder(ctx context.Context, req *CreateWechatOrderRequest) (*CreateWechatOrderResponse, error) {
	// 生成系统订单号
	orderNo := generateWechatOrderNo()

	// 创建订单
	order := &models.Order{
		OrderNo:       orderNo,
		UserID:        req.UserID,
		ProductID:     req.ProductID,
		Type:          models.OrderTypePurchase,
		Title:         req.Description,
		Description:   req.Detail,
		Quantity:      1,
		Currency:      "CNY",
		TotalAmount:   req.TotalAmount,
		Status:        models.OrderStatusCreated,
		PaymentMethod: models.PaymentMethodWeChat,
		PaymentStatus: models.PaymentStatusPending,
		ExpiredAt:     &[]time.Time{time.Now().Add(30 * time.Minute)}[0],
	}

	// 开启事务
	tx := s.db.Begin()
	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建订单失败: %v", err)
	}

	// 创建微信支付记录
	wechatPayment := &models.WechatPayment{
		OrderID:    order.ID,
		OutTradeNo: orderNo,
		TradeType:  req.TradeType,
		AppID:      s.config.AppID,
		MchID:      s.config.MchID,
	}

	if err := tx.Create(wechatPayment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建微信支付记录失败: %v", err)
	}

	// 创建支付交易记录
	transaction := &models.PaymentTransaction{
		OrderID:       order.ID,
		TransactionID: orderNo,
		Provider:      models.PaymentProviderWeChat,
		Type:          "PAYMENT",
		Amount:        req.TotalAmount,
		Currency:      "CNY",
		Status:        models.PaymentStatusPending,
		ProviderData:  models.JSON{},
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建交易记录失败: %v", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	s.logger.Info("微信订单创建成功",
		zap.String("order_no", orderNo),
		zap.Uint("order_id", order.ID),
		zap.String("trade_type", req.TradeType),
	)

	return &CreateWechatOrderResponse{
		OrderID:     order.ID,
		OrderNo:     orderNo,
		TotalAmount: req.TotalAmount,
		Description: req.Description,
	}, nil
}

// CreateJSAPIPayment 创建JSAPI支付（小程序、公众号）
func (s *WechatService) CreateJSAPIPayment(ctx context.Context, orderNo, openID string) (*JSAPIPaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 构建预支付交易单请求
	// 注意：实际应用中应该调用微信支付API创建预支付单
	_ = map[string]interface{}{
		"appid":        s.config.AppID,
		"mchid":        s.config.MchID,
		"description":  order.Title,
		"out_trade_no": orderNo,
		"time_expire":  order.ExpiredAt.Format(time.RFC3339),
		"notify_url":   s.config.NotifyURL,
		"amount": map[string]interface{}{
			"total":    order.TotalAmount,
			"currency": "CNY",
		},
		"payer": map[string]interface{}{
			"openid": openID,
		},
	}

	// 这里应该调用微信支付API，为了演示，我们模拟生成prepay_id
	prepayID := fmt.Sprintf("wx%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])

	// 更新微信支付记录
	wechatPayment.PrepayID = prepayID
	if payerJSON, _ := json.Marshal(map[string]string{"openid": openID}); payerJSON != nil {
		wechatPayment.Payer = models.JSON{"openid": openID}
	}
	if amountJSON, _ := json.Marshal(map[string]interface{}{"total": order.TotalAmount, "currency": "CNY"}); amountJSON != nil {
		wechatPayment.Amount = models.JSON{"total": order.TotalAmount, "currency": "CNY"}
	}

	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("JSAPI支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("prepay_id", prepayID),
		zap.String("openid", openID),
	)

	// 生成小程序调起支付所需参数
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()
	packageStr := fmt.Sprintf("prepay_id=%s", prepayID)

	return &JSAPIPaymentResponse{
		PrepayID:  prepayID,
		AppID:     s.config.AppID,
		TimeStamp: timestamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  "RSA",
	}, nil
}

// CreateNativePayment 创建Native支付（扫码支付）
func (s *WechatService) CreateNativePayment(ctx context.Context, orderNo string) (*NativePaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 生成二维码链接（模拟）
	codeURL := fmt.Sprintf("weixin://wxpay/bizpayurl?pr=%s", generateCodeURL())

	// 更新微信支付记录
	wechatPayment.CodeURL = codeURL
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("Native支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("code_url", codeURL),
	)

	return &NativePaymentResponse{
		CodeURL: codeURL,
	}, nil
}

// CreateAPPPayment 创建APP支付
func (s *WechatService) CreateAPPPayment(ctx context.Context, orderNo string) (*APPPaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 生成prepay_id（模拟）
	prepayID := fmt.Sprintf("wx%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])

	// 更新微信支付记录
	wechatPayment.PrepayID = prepayID
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("APP支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("prepay_id", prepayID),
	)

	// 生成APP调起支付所需参数
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()
	packageStr := "Sign=WXPay"

	return &APPPaymentResponse{
		PrepayID:  prepayID,
		PartnerID: s.config.MchID,
		AppID:     s.config.AppID,
		TimeStamp: timestamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  "RSA",
	}, nil
}

// CreateH5Payment 创建H5支付
func (s *WechatService) CreateH5Payment(ctx context.Context, orderNo string, sceneInfo map[string]interface{}) (*H5PaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 生成H5支付链接（模拟）
	h5URL := fmt.Sprintf("https://wx.tenpay.com/cgi-bin/mmpayweb-bin/checkmweb?prepay_id=%s&package=XXX", uuid.New().String())

	// 更新微信支付记录
	wechatPayment.H5URL = h5URL
	if sceneInfo != nil {
		wechatPayment.SceneInfo = models.JSON(sceneInfo)
	}
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("H5支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("h5_url", h5URL),
	)

	return &H5PaymentResponse{
		H5URL: h5URL,
	}, nil
}

// HandleNotify 处理微信支付异步通知
func (s *WechatService) HandleNotify(ctx context.Context, notifyData map[string]interface{}) error {
	// 提取关键参数
	outTradeNo, _ := notifyData["out_trade_no"].(string)
	transactionID, _ := notifyData["transaction_id"].(string)
	tradeState, _ := notifyData["trade_state"].(string)

	if outTradeNo == "" {
		return errors.New("缺少商户订单号")
	}

	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", outTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 开启事务
	tx := s.db.Begin()

	// 更新微信支付记录
	wechatPayment.TransactionID = transactionID
	wechatPayment.TradeState = tradeState
	if tradeStateDesc, ok := notifyData["trade_state_desc"].(string); ok {
		wechatPayment.TradeStateDesc = tradeStateDesc
	}
	if bankType, ok := notifyData["bank_type"].(string); ok {
		wechatPayment.BankType = bankType
	}

	// 解析支付完成时间
	if successTimeStr, ok := notifyData["success_time"].(string); ok {
		if successTime, err := time.Parse(time.RFC3339, successTimeStr); err == nil {
			wechatPayment.SuccessTime = &successTime
		}
	}

	// 保存原始通知数据
	wechatPayment.RawNotifyData = models.JSON(notifyData)
	now := time.Now()
	wechatPayment.NotifyTime = &now

	if err := tx.Save(&wechatPayment).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新微信支付记录失败: %v", err)
	}

	// 根据交易状态更新订单
	switch tradeState {
	case "SUCCESS":
		order.Status = models.OrderStatusPaid
		order.PaymentStatus = models.PaymentStatusCompleted
		order.PaidAt = &now
	case "CLOSED", "REVOKED", "PAYERROR":
		order.PaymentStatus = models.PaymentStatusFailed
	}

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新订单状态失败: %v", err)
	}

	// 更新交易记录
	var transaction models.PaymentTransaction
	if err := tx.Where("order_id = ? AND transaction_id = ?", order.ID, outTradeNo).First(&transaction).Error; err == nil {
		switch tradeState {
		case "SUCCESS":
			transaction.Status = models.PaymentStatusCompleted
			transaction.ProcessedAt = &now
			transaction.ProviderData = models.JSON(notifyData)
		case "CLOSED", "REVOKED", "PAYERROR":
			transaction.Status = models.PaymentStatusFailed
			transaction.ProcessedAt = &now
			if errorMsg, ok := notifyData["error_message"].(string); ok {
				transaction.ErrorMessage = &errorMsg
			}
		}
		if err := tx.Save(&transaction).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("更新交易记录失败: %v", err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}

	s.logger.Info("微信支付通知处理成功",
		zap.String("out_trade_no", outTradeNo),
		zap.String("transaction_id", transactionID),
		zap.String("trade_state", tradeState),
	)

	return nil
}

// QueryOrder 查询订单状态
func (s *WechatService) QueryOrder(ctx context.Context, orderNo string) (*QueryWechatOrderResponse, error) {
	// 查询本地订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 如果订单已完成，直接返回
	if order.Status == models.OrderStatusPaid {
		return &QueryWechatOrderResponse{
			OrderNo:       orderNo,
			TransactionID: wechatPayment.TransactionID,
			TradeState:    wechatPayment.TradeState,
			TotalAmount:   order.TotalAmount,
			PaymentStatus: order.PaymentStatus,
			PaidAt:        order.PaidAt,
		}, nil
	}

	// 实际应用中，这里应该调用微信支付查询接口
	// 为演示目的，直接返回本地数据

	return &QueryWechatOrderResponse{
		OrderNo:       orderNo,
		TransactionID: wechatPayment.TransactionID,
		TradeState:    wechatPayment.TradeState,
		TotalAmount:   order.TotalAmount,
		PaymentStatus: order.PaymentStatus,
		PaidAt:        order.PaidAt,
	}, nil
}

// Refund 退款
func (s *WechatService) Refund(ctx context.Context, req *WechatRefundRequest) (*WechatRefundResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 检查订单状态
	if order.Status != models.OrderStatusPaid {
		return nil, errors.New("订单未支付，无法退款")
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 生成退款单号
	outRefundNo := generateWechatRefundNo(req.OrderNo)
	refundID := fmt.Sprintf("wx%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])

	// 开启事务
	tx := s.db.Begin()

	// 创建退款记录
	refund := &models.WechatRefund{
		OrderID:         order.ID,
		WechatPaymentID: wechatPayment.ID,
		OutRefundNo:     outRefundNo,
		RefundID:        refundID,
		OutTradeNo:      req.OrderNo,
		TransactionID:   wechatPayment.TransactionID,
		RefundAmount:    req.RefundAmount,
		TotalAmount:     order.TotalAmount,
		Currency:        "CNY",
		RefundReason:    req.RefundReason,
		RefundStatus:    "SUCCESS",
	}

	now := time.Now()
	refund.SuccessTime = &now

	if err := tx.Create(refund).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建退款记录失败: %v", err)
	}

	// 更新订单状态
	order.Status = models.OrderStatusRefunded
	order.RefundAt = &now
	order.RefundReason = req.RefundReason
	order.RefundAmount = req.RefundAmount

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("更新订单状态失败: %v", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	s.logger.Info("微信退款成功",
		zap.String("out_trade_no", req.OrderNo),
		zap.String("out_refund_no", outRefundNo),
		zap.String("refund_id", refundID),
		zap.Int64("refund_amount", req.RefundAmount),
	)

	return &WechatRefundResponse{
		OutRefundNo:  outRefundNo,
		RefundID:     refundID,
		RefundAmount: req.RefundAmount,
		RefundStatus: "SUCCESS",
		RefundAt:     &now,
	}, nil
}

// CloseOrder 关闭订单
func (s *WechatService) CloseOrder(ctx context.Context, orderNo string) error {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %v", err)
	}

	// 检查订单状态
	if order.Status == models.OrderStatusPaid {
		return errors.New("订单已支付，无法关闭")
	}

	// 更新订单状态
	order.Status = models.OrderStatusCancelled
	order.PaymentStatus = models.PaymentStatusCancelled

	if err := s.db.Save(&order).Error; err != nil {
		return fmt.Errorf("关闭订单失败: %v", err)
	}

	s.logger.Info("订单关闭成功",
		zap.String("order_no", orderNo),
		zap.Uint("order_id", order.ID),
	)

	return nil
}

// 辅助函数

func parseWechatPrivateKey(privateKeyStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		return nil, errors.New("无法解析私钥")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// 尝试PKCS1格式
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("不是RSA私钥")
	}

	return privateKey, nil
}

func generateWechatOrderNo() string {
	return fmt.Sprintf("WX%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
}

func generateWechatRefundNo(orderNo string) string {
	return fmt.Sprintf("REF%s%s", orderNo, time.Now().Format("150405"))
}

func generateNonceStr() string {
	return uuid.New().String()[:32]
}

func generateCodeURL() string {
	return uuid.New().String()[:16]
}

// 请求和响应结构体

// CreateWechatOrderRequest 创建微信订单请求
type CreateWechatOrderRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
	Description string `json:"description" binding:"required"`
	Detail      string `json:"detail"`
	TotalAmount int64  `json:"total_amount" binding:"required,min=1"`
	TradeType   string `json:"trade_type" binding:"required,oneof=JSAPI NATIVE APP MWEB"`
}

// CreateWechatOrderResponse 创建微信订单响应
type CreateWechatOrderResponse struct {
	OrderID     uint   `json:"order_id"`
	OrderNo     string `json:"order_no"`
	TotalAmount int64  `json:"total_amount"`
	Description string `json:"description"`
}

// JSAPIPaymentResponse JSAPI支付响应
type JSAPIPaymentResponse struct {
	PrepayID  string `json:"prepay_id"`
	AppID     string `json:"app_id"`
	TimeStamp string `json:"time_stamp"`
	NonceStr  string `json:"nonce_str"`
	Package   string `json:"package"`
	SignType  string `json:"sign_type"`
	PaySign   string `json:"pay_sign,omitempty"` // 需要客户端计算
}

// NativePaymentResponse Native支付响应
type NativePaymentResponse struct {
	CodeURL string `json:"code_url"`
}

// APPPaymentResponse APP支付响应
type APPPaymentResponse struct {
	PrepayID  string `json:"prepay_id"`
	PartnerID string `json:"partner_id"`
	AppID     string `json:"app_id"`
	TimeStamp string `json:"time_stamp"`
	NonceStr  string `json:"nonce_str"`
	Package   string `json:"package"`
	SignType  string `json:"sign_type"`
	Sign      string `json:"sign,omitempty"` // 需要客户端计算
}

// H5PaymentResponse H5支付响应
type H5PaymentResponse struct {
	H5URL string `json:"h5_url"`
}

// QueryWechatOrderResponse 查询微信订单响应
type QueryWechatOrderResponse struct {
	OrderNo       string               `json:"order_no"`
	TransactionID string               `json:"transaction_id"`
	TradeState    string               `json:"trade_state"`
	TotalAmount   int64                `json:"total_amount"`
	PaymentStatus models.PaymentStatus `json:"payment_status"`
	PaidAt        *time.Time           `json:"paid_at,omitempty"`
}

// WechatRefundRequest 微信退款请求
type WechatRefundRequest struct {
	OrderNo      string `json:"order_no" binding:"required"`
	RefundAmount int64  `json:"refund_amount" binding:"required,min=1"`
	RefundReason string `json:"refund_reason" binding:"required"`
}

// WechatRefundResponse 微信退款响应
type WechatRefundResponse struct {
	OutRefundNo  string     `json:"out_refund_no"`
	RefundID     string     `json:"refund_id"`
	RefundAmount int64      `json:"refund_amount"`
	RefundStatus string     `json:"refund_status"`
	RefundAt     *time.Time `json:"refund_at,omitempty"`
}
