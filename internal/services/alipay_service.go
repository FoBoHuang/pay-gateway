package services

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	alipay "github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// AlipayService 支付宝支付服务
type AlipayService struct {
	client *alipay.Client
	db     *gorm.DB
	config *config.AlipayConfig
}

// NewAlipayService 创建支付宝支付服务
func NewAlipayService(db *gorm.DB, cfg *config.AlipayConfig) (*AlipayService, error) {
	// 创建支付宝客户端（直接使用私钥字符串）
	client, err := alipay.New(cfg.AppID, cfg.PrivateKey, cfg.IsProduction)
	if err != nil {
		return nil, fmt.Errorf("创建支付宝客户端失败: %v", err)
	}

	// 加载证书（证书模式）
	if cfg.CertMode {
		if err := client.LoadAppCertPublicKeyFromFile(cfg.AppCertPath); err != nil {
			return nil, fmt.Errorf("加载应用公钥证书失败: %v", err)
		}
		if err := client.LoadAliPayRootCertFromFile(cfg.RootCertPath); err != nil {
			return nil, fmt.Errorf("加载支付宝根证书失败: %v", err)
		}
		if err := client.LoadAlipayCertPublicKeyFromFile(cfg.AlipayCertPath); err != nil {
			return nil, fmt.Errorf("加载支付宝公钥证书失败: %v", err)
		}
	}

	return &AlipayService{
		client: client,
		db:     db,
		config: cfg,
	}, nil
}

// CreateOrder 创建支付宝订单
func (s *AlipayService) CreateOrder(ctx context.Context, req *CreateAlipayOrderRequest) (*CreateAlipayOrderResponse, error) {
	// 生成系统订单号
	orderNo := generateOrderNo()

	// 创建订单
	order := &models.Order{
		OrderNo:       orderNo,
		UserID:        req.UserID,
		ProductID:     req.ProductID,
		Type:          models.OrderTypePurchase,
		Title:         req.Subject,
		Description:   req.Body,
		Quantity:      1,
		Currency:      "CNY",
		TotalAmount:   req.TotalAmount,
		Status:        models.OrderStatusCreated,
		PaymentMethod: models.PaymentMethodAlipay,
		PaymentStatus: models.PaymentStatusPending,
		ExpiredAt:     &[]time.Time{time.Now().Add(30 * time.Minute)}[0],
	}

	// 开启事务
	tx := s.db.Begin()
	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建订单失败: %v", err)
	}

	// 创建支付宝支付记录
	alipayPayment := &models.AlipayPayment{
		OrderID:        order.ID,
		OutTradeNo:     orderNo,
		TotalAmount:    formatAmount(req.TotalAmount),
		Subject:        req.Subject,
		Body:           req.Body,
		TradeStatus:    "WAIT_BUYER_PAY",
		AppID:          s.config.AppID,
		TimeoutExpress: "30m",
	}

	if err := tx.Create(alipayPayment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建支付宝支付记录失败: %v", err)
	}

	// 创建支付交易记录
	transaction := &models.PaymentTransaction{
		OrderID:       order.ID,
		TransactionID: orderNo,
		Provider:      models.PaymentProviderAlipay,
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

	return &CreateAlipayOrderResponse{
		OrderID:     order.ID,
		OrderNo:     orderNo,
		TotalAmount: req.TotalAmount,
		Subject:     req.Subject,
		Description: req.Body,
	}, nil
}

// CreateWapPayment 创建手机网站支付
func (s *AlipayService) CreateWapPayment(ctx context.Context, orderNo string) (string, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return "", fmt.Errorf("订单不存在: %v", err)
	}

	// 查询支付宝支付记录
	var alipayPayment models.AlipayPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&alipayPayment).Error; err != nil {
		return "", fmt.Errorf("支付宝支付记录不存在: %v", err)
	}

	// 构建支付请求
	p := alipay.TradeWapPay{}
	p.NotifyURL = s.config.NotifyURL
	p.ReturnURL = s.config.ReturnURL
	p.Subject = alipayPayment.Subject
	p.OutTradeNo = alipayPayment.OutTradeNo
	p.TotalAmount = alipayPayment.TotalAmount
	p.ProductCode = "QUICK_WAP_WAY"
	p.TimeoutExpress = alipayPayment.TimeoutExpress

	// 生成支付URL
	url, err := s.client.TradeWapPay(p)
	if err != nil {
		return "", fmt.Errorf("创建支付URL失败: %v", err)
	}

	return url.String(), nil
}

// CreatePagePayment 创建电脑网站支付
func (s *AlipayService) CreatePagePayment(ctx context.Context, orderNo string) (string, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return "", fmt.Errorf("订单不存在: %v", err)
	}

	// 查询支付宝支付记录
	var alipayPayment models.AlipayPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&alipayPayment).Error; err != nil {
		return "", fmt.Errorf("支付宝支付记录不存在: %v", err)
	}

	// 构建支付请求
	p := alipay.TradePagePay{}
	p.NotifyURL = s.config.NotifyURL
	p.ReturnURL = s.config.ReturnURL
	p.Subject = alipayPayment.Subject
	p.OutTradeNo = alipayPayment.OutTradeNo
	p.TotalAmount = alipayPayment.TotalAmount
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"
	p.TimeoutExpress = alipayPayment.TimeoutExpress

	// 生成支付URL
	url, err := s.client.TradePagePay(p)
	if err != nil {
		return "", fmt.Errorf("创建支付URL失败: %v", err)
	}

	return url.String(), nil
}

// HandleNotify 处理支付宝异步通知
func (s *AlipayService) HandleNotify(ctx context.Context, notifyData map[string]string) error {
	// 验证签名
	formData := url.Values{}
	for k, v := range notifyData {
		formData.Set(k, v)
	}
	err := s.client.VerifySign(formData)
	if err != nil {
		return errors.New("签名验证失败")
	}

	// 提取关键参数
	outTradeNo := notifyData["out_trade_no"]
	tradeNo := notifyData["trade_no"]
	tradeStatus := notifyData["trade_status"]
	_ = notifyData["total_amount"] // 总金额，暂时未使用

	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", outTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %v", err)
	}

	// 查询支付宝支付记录
	var alipayPayment models.AlipayPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&alipayPayment).Error; err != nil {
		return fmt.Errorf("支付宝支付记录不存在: %v", err)
	}

	// 开启事务
	tx := s.db.Begin()

	// 更新支付宝支付记录
	alipayPayment.TradeNo = tradeNo
	alipayPayment.TradeStatus = string(tradeStatus)
	if buyerUserID, ok := notifyData["buyer_user_id"]; ok {
		alipayPayment.BuyerUserID = buyerUserID
	}
	if buyerLogonID, ok := notifyData["buyer_logon_id"]; ok {
		alipayPayment.BuyerLogonID = buyerLogonID
	}
	if notifyTime, ok := notifyData["notify_time"]; ok {
		if t, err := time.Parse("2006-01-02 15:04:05", notifyTime); err == nil {
			alipayPayment.NotifyTime = &t
		}
	}
	if gmtPayment, ok := notifyData["gmt_payment"]; ok {
		if t, err := time.Parse("2006-01-02 15:04:05", gmtPayment); err == nil {
			alipayPayment.TimeEnd = &t
			alipayPayment.SendPayDate = &t
		}
	}

	// 保存完整的通知数据 - 暂时不保存，避免字段不存在的问题

	if err := tx.Save(&alipayPayment).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新支付宝支付记录失败: %v", err)
	}

	// 根据交易状态更新订单
	now := time.Now()
	switch string(tradeStatus) {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		order.Status = models.OrderStatusPaid
		order.PaymentStatus = models.PaymentStatusCompleted
		order.PaidAt = &now
	case "TRADE_CLOSED":
		order.PaymentStatus = models.PaymentStatusCancelled
	}

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新订单状态失败: %v", err)
	}

	// 更新交易记录
	var transaction models.PaymentTransaction
	if err := tx.Where("order_id = ? AND transaction_id = ?", order.ID, outTradeNo).First(&transaction).Error; err == nil {
		switch string(tradeStatus) {
		case "TRADE_SUCCESS", "TRADE_FINISHED":
			transaction.Status = models.PaymentStatusCompleted
			transaction.ProcessedAt = &now
			providerData := make(models.JSON)
			for k, v := range notifyData {
				providerData[k] = v
			}
			transaction.ProviderData = providerData
		case "TRADE_CLOSED":
			transaction.Status = models.PaymentStatusCancelled
			transaction.ProcessedAt = &now
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

	return nil
}

// QueryOrder 查询订单状态
func (s *AlipayService) QueryOrder(ctx context.Context, orderNo string) (*QueryAlipayOrderResponse, error) {
	// 查询本地订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 如果订单已完成，直接返回
	if order.Status == models.OrderStatusPaid {
		return &QueryAlipayOrderResponse{
			OrderNo:       orderNo,
			TradeStatus:   "TRADE_SUCCESS",
			TotalAmount:   order.TotalAmount,
			PaymentStatus: order.PaymentStatus,
			PaidAt:        order.PaidAt,
		}, nil
	}

	// 查询支付宝订单状态
	p := alipay.TradeQuery{}
	p.OutTradeNo = orderNo

	result, err := s.client.TradeQuery(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("查询支付宝订单失败: %v", err)
	}

	if result.Code != "10000" {
		return nil, fmt.Errorf("支付宝查询失败: %s", result.Msg)
	}

	// 解析响应数据
	tradeStatus := result.TradeStatus
	tradeNo := result.TradeNo

	// 如果状态有更新，同步更新本地数据
	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		if order.Status != models.OrderStatusPaid {
			now := time.Now()
			order.Status = models.OrderStatusPaid
			order.PaymentStatus = models.PaymentStatusCompleted
			order.PaidAt = &now
			s.db.Save(&order)
		}
	}

	return &QueryAlipayOrderResponse{
		OrderNo:       orderNo,
		TradeNo:       tradeNo,
		TradeStatus:   string(tradeStatus),
		TotalAmount:   order.TotalAmount,
		PaymentStatus: order.PaymentStatus,
		PaidAt:        order.PaidAt,
	}, nil
}

// Refund 退款
func (s *AlipayService) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 检查订单状态
	if order.Status != models.OrderStatusPaid {
		return nil, errors.New("订单未支付，无法退款")
	}

	// 查询支付宝支付记录
	var alipayPayment models.AlipayPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&alipayPayment).Error; err != nil {
		return nil, fmt.Errorf("支付宝支付记录不存在: %v", err)
	}

	// 生成退款请求号
	refundRequestNo := generateRefundRequestNo(req.OrderNo)

	// 构建退款请求
	p := alipay.TradeRefund{}
	p.OutTradeNo = req.OrderNo
	p.OutRequestNo = refundRequestNo
	p.RefundAmount = formatAmount(req.RefundAmount)
	p.RefundReason = req.RefundReason

	result, err := s.client.TradeRefund(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("退款请求失败: %v", err)
	}

	if result.Code != "10000" {
		return nil, fmt.Errorf("支付宝退款失败: %s", result.Msg)
	}

	// 开启事务
	tx := s.db.Begin()

	// 创建退款记录
	refund := &models.AlipayRefund{
		OrderID:         order.ID,
		AlipayPaymentID: alipayPayment.ID,
		OutRequestNo:    refundRequestNo,
		OutTradeNo:      req.OrderNo,
		TradeNo:         alipayPayment.TradeNo,
		RefundAmount:    formatAmount(req.RefundAmount),
		TotalAmount:     alipayPayment.TotalAmount,
		Currency:        "CNY",
		RefundReason:    req.RefundReason,
		RefundStatus:    "REFUND_SUCCESS",
		GmtRefundPay:    &[]time.Time{time.Now()}[0],
	}

	if err := tx.Create(refund).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建退款记录失败: %v", err)
	}

	// 更新订单状态
	now := time.Now()
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

	return &RefundResponse{
		RefundRequestNo: refundRequestNo,
		RefundAmount:    req.RefundAmount,
		RefundStatus:    "REFUND_SUCCESS",
		RefundAt:        &now,
	}, nil
}

// 辅助函数

func parsePrivateKey(privateKeyStr string) (*rsa.PrivateKey, error) {
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

func generateOrderNo() string {
	return fmt.Sprintf("ORD%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
}

func generateRefundRequestNo(orderNo string) string {
	return fmt.Sprintf("REF%s%s", orderNo, time.Now().Format("150405"))
}

func formatAmount(amount int64) string {
	return fmt.Sprintf("%.2f", float64(amount)/100)
}

// ==================== 周期扣款（订阅）功能 ====================

// CreateSubscription 创建支付宝周期扣款协议（签约）
func (s *AlipayService) CreateSubscription(ctx context.Context, req *CreateAlipaySubscriptionRequest) (*CreateAlipaySubscriptionResponse, error) {
	// 生成商户签约号
	outRequestNo := fmt.Sprintf("SUB%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])

	// 创建订单记录
	order := &models.Order{
		OrderNo:       outRequestNo,
		UserID:        req.UserID,
		ProductID:     req.ProductID,
		Type:          models.OrderTypeSubscription,
		Title:         req.ProductName,
		Description:   req.ProductDesc,
		Quantity:      1,
		Currency:      "CNY",
		TotalAmount:   req.SingleAmount,
		Status:        models.OrderStatusCreated,
		PaymentMethod: models.PaymentMethodAlipay,
		PaymentStatus: models.PaymentStatusPending,
	}

	// 计算首次执行时间（默认为明天）
	executionTime := time.Now().AddDate(0, 0, 1)
	if req.ExecutionTime != nil {
		executionTime = *req.ExecutionTime
	}

	// 开启事务
	tx := s.db.Begin()
	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建订单失败: %v", err)
	}

	// 创建周期扣款记录
	subscription := &models.AlipaySubscription{
		OrderID:             order.ID,
		OutRequestNo:        outRequestNo,
		PeriodType:          req.PeriodType,
		Period:              req.Period,
		ExecutionTime:       &executionTime,
		SingleAmount:        formatAmount(req.SingleAmount),
		TotalAmount:         formatAmount(req.TotalAmount),
		TotalPayments:       req.TotalPayments,
		CurrentPeriod:       0,
		Status:              "TEMP", // 临时状态，等待签约
		AppID:               s.config.AppID,
		PersonalProductCode: req.PersonalProductCode,
		SignScene:           req.SignScene,
	}

	if err := tx.Create(subscription).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建周期扣款记录失败: %v", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	// 构建签约请求
	// 注意：实际应用中需要调用支付宝签约API
	// 这里返回签约URL供客户端跳转
	signURL := fmt.Sprintf("https://openapi.alipay.com/gateway.do?method=alipay.user.agreement.page.sign&out_request_no=%s", outRequestNo)

	return &CreateAlipaySubscriptionResponse{
		OrderID:       order.ID,
		OutRequestNo:  outRequestNo,
		SignURL:       signURL,
		Status:        "TEMP",
		ExecutionTime: executionTime,
	}, nil
}

// QuerySubscription 查询周期扣款状态
func (s *AlipayService) QuerySubscription(ctx context.Context, outRequestNo string) (*QueryAlipaySubscriptionResponse, error) {
	var subscription models.AlipaySubscription
	if err := s.db.Where("out_request_no = ?", outRequestNo).First(&subscription).Error; err != nil {
		return nil, fmt.Errorf("周期扣款协议不存在: %v", err)
	}

	// 实际应用中应该调用支付宝API查询最新状态

	return &QueryAlipaySubscriptionResponse{
		OutRequestNo:        subscription.OutRequestNo,
		AgreementNo:         subscription.AgreementNo,
		ExternalAgreementNo: subscription.ExternalAgreementNo,
		Status:              subscription.Status,
		SignTime:            subscription.SignTime,
		ValidTime:           subscription.ValidTime,
		InvalidTime:         subscription.InvalidTime,
		PeriodType:          subscription.PeriodType,
		Period:              subscription.Period,
		ExecutionTime:       subscription.ExecutionTime,
		SingleAmount:        subscription.SingleAmount,
		TotalAmount:         subscription.TotalAmount,
		TotalPayments:       subscription.TotalPayments,
		CurrentPeriod:       subscription.CurrentPeriod,
		LastDeductTime:      subscription.LastDeductTime,
		NextDeductTime:      subscription.NextDeductTime,
		DeductSuccessCount:  subscription.DeductSuccessCount,
		DeductFailCount:     subscription.DeductFailCount,
	}, nil
}

// CancelSubscription 解约（取消周期扣款）
func (s *AlipayService) CancelSubscription(ctx context.Context, req *CancelAlipaySubscriptionRequest) error {
	var subscription models.AlipaySubscription
	if err := s.db.Where("out_request_no = ? OR agreement_no = ?", req.OutRequestNo, req.AgreementNo).First(&subscription).Error; err != nil {
		return fmt.Errorf("周期扣款协议不存在: %v", err)
	}

	// 检查状态
	if subscription.Status == "STOP" {
		return errors.New("协议已停止，无需重复解约")
	}

	// 实际应用中应该调用支付宝解约API
	// p := alipay.UserAgreementUnsign{}
	// p.AgreementNo = subscription.AgreementNo
	// result, err := s.client.UserAgreementUnsign(ctx, p)

	// 更新本地状态
	now := time.Now()
	subscription.Status = "STOP"
	subscription.CancelTime = &now
	subscription.CancelReason = req.CancelReason

	if err := s.db.Save(&subscription).Error; err != nil {
		return fmt.Errorf("更新周期扣款状态失败: %v", err)
	}

	return nil
}

// HandleSubscriptionNotify 处理支付宝周期扣款通知
func (s *AlipayService) HandleSubscriptionNotify(ctx context.Context, notifyData map[string]string) error {
	// 验证签名
	formData := url.Values{}
	for k, v := range notifyData {
		formData.Set(k, v)
	}
	err := s.client.VerifySign(formData)
	if err != nil {
		return errors.New("签名验证失败")
	}

	// 提取关键参数
	agreementNo := notifyData["agreement_no"]
	outRequestNo := notifyData["out_request_no"]
	status := notifyData["status"]

	// 查询周期扣款记录
	var subscription models.AlipaySubscription
	if err := s.db.Where("out_request_no = ? OR agreement_no = ?", outRequestNo, agreementNo).First(&subscription).Error; err != nil {
		return fmt.Errorf("周期扣款协议不存在: %v", err)
	}

	// 更新协议信息
	now := time.Now()
	subscription.AgreementNo = agreementNo
	subscription.Status = status

	if status == "NORMAL" {
		// 签约成功
		if signTime, ok := notifyData["sign_time"]; ok {
			if t, err := time.Parse("2006-01-02 15:04:05", signTime); err == nil {
				subscription.SignTime = &t
			}
		}
		if validTime, ok := notifyData["valid_time"]; ok {
			if t, err := time.Parse("2006-01-02 15:04:05", validTime); err == nil {
				subscription.ValidTime = &t
			}
		}
		if invalidTime, ok := notifyData["invalid_time"]; ok {
			if t, err := time.Parse("2006-01-02 15:04:05", invalidTime); err == nil {
				subscription.InvalidTime = &t
			}
		}
	} else if status == "STOP" {
		// 解约
		subscription.CancelTime = &now
	}

	if err := s.db.Save(&subscription).Error; err != nil {
		return fmt.Errorf("更新周期扣款状态失败: %v", err)
	}

	return nil
}

// HandleDeductNotify 处理周期扣款扣款通知
func (s *AlipayService) HandleDeductNotify(ctx context.Context, notifyData map[string]string) error {
	// 验证签名
	formData := url.Values{}
	for k, v := range notifyData {
		formData.Set(k, v)
	}
	err := s.client.VerifySign(formData)
	if err != nil {
		return errors.New("签名验证失败")
	}

	// 提取关键参数
	agreementNo := notifyData["agreement_no"]
	outTradeNo := notifyData["out_trade_no"]
	tradeNo := notifyData["trade_no"]
	amount := notifyData["amount"]
	status := notifyData["status"] // SUCCESS 或 FAIL

	// 查询周期扣款记录
	var subscription models.AlipaySubscription
	if err := s.db.Where("agreement_no = ?", agreementNo).First(&subscription).Error; err != nil {
		return fmt.Errorf("周期扣款协议不存在: %v", err)
	}

	// 更新扣款统计
	now := time.Now()
	subscription.LastDeductTime = &now
	subscription.LastDeductAmount = amount
	subscription.LastDeductStatus = status

	if status == "SUCCESS" {
		subscription.DeductSuccessCount++
		subscription.CurrentPeriod++
	} else {
		subscription.DeductFailCount++
	}

	// 计算下次扣款时间
	if subscription.ExecutionTime != nil {
		var nextTime time.Time
		if subscription.PeriodType == "MONTH" {
			nextTime = subscription.ExecutionTime.AddDate(0, subscription.Period*subscription.CurrentPeriod, 0)
		} else if subscription.PeriodType == "DAY" {
			nextTime = subscription.ExecutionTime.AddDate(0, 0, subscription.Period*subscription.CurrentPeriod)
		}
		subscription.NextDeductTime = &nextTime
	}

	if err := s.db.Save(&subscription).Error; err != nil {
		return fmt.Errorf("更新周期扣款记录失败: %v", err)
	}

	// 如果扣款成功，创建支付记录
	if status == "SUCCESS" {
		// 创建支付宝支付记录
		alipayPayment := &models.AlipayPayment{
			OrderID:     subscription.OrderID,
			OutTradeNo:  outTradeNo,
			TradeNo:     tradeNo,
			TotalAmount: amount,
			Subject:     fmt.Sprintf("周期扣款-%s", agreementNo),
			TradeStatus: "TRADE_SUCCESS",
			AppID:       s.config.AppID,
			TimeEnd:     &now,
		}
		s.db.Create(alipayPayment)
	}

	return nil
}

// ==================== 请求和响应结构体 ====================

type CreateAlipayOrderRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	Body        string `json:"body"`
	TotalAmount int64  `json:"total_amount" binding:"required,min=1"`
}

type CreateAlipayOrderResponse struct {
	OrderID     uint   `json:"order_id"`
	OrderNo     string `json:"order_no"`
	TotalAmount int64  `json:"total_amount"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

type QueryAlipayOrderResponse struct {
	OrderNo       string               `json:"order_no"`
	TradeNo       string               `json:"trade_no,omitempty"`
	TradeStatus   string               `json:"trade_status"`
	TotalAmount   int64                `json:"total_amount"`
	PaymentStatus models.PaymentStatus `json:"payment_status"`
	PaidAt        *time.Time           `json:"paid_at,omitempty"`
}

type RefundRequest struct {
	OrderNo      string `json:"order_no" binding:"required"`
	RefundAmount int64  `json:"refund_amount" binding:"required,min=1"`
	RefundReason string `json:"refund_reason" binding:"required"`
}

type RefundResponse struct {
	RefundRequestNo string     `json:"refund_request_no"`
	RefundAmount    int64      `json:"refund_amount"`
	RefundStatus    string     `json:"refund_status"`
	RefundAt        *time.Time `json:"refund_at,omitempty"`
}

// 周期扣款相关结构体

type CreateAlipaySubscriptionRequest struct {
	UserID              uint       `json:"user_id" binding:"required"`
	ProductID           string     `json:"product_id" binding:"required"`
	ProductName         string     `json:"product_name" binding:"required"`
	ProductDesc         string     `json:"product_desc"`
	PeriodType          string     `json:"period_type" binding:"required,oneof=DAY MONTH"` // 周期类型：DAY-日，MONTH-月
	Period              int        `json:"period" binding:"required,min=1"`                // 周期数
	ExecutionTime       *time.Time `json:"execution_time"`                                 // 首次执行时间
	SingleAmount        int64      `json:"single_amount" binding:"required,min=1"`         // 单次扣款金额（分）
	TotalAmount         int64      `json:"total_amount"`                                   // 总金额限制（分）
	TotalPayments       int        `json:"total_payments"`                                 // 总扣款次数
	PersonalProductCode string     `json:"personal_product_code" binding:"required"`       // 个人签约产品码
	SignScene           string     `json:"sign_scene" binding:"required"`                  // 签约场景
}

type CreateAlipaySubscriptionResponse struct {
	OrderID       uint      `json:"order_id"`
	OutRequestNo  string    `json:"out_request_no"`
	SignURL       string    `json:"sign_url"` // 签约URL
	Status        string    `json:"status"`
	ExecutionTime time.Time `json:"execution_time"`
}

type QueryAlipaySubscriptionResponse struct {
	OutRequestNo        string     `json:"out_request_no"`
	AgreementNo         string     `json:"agreement_no"`
	ExternalAgreementNo string     `json:"external_agreement_no,omitempty"`
	Status              string     `json:"status"`
	SignTime            *time.Time `json:"sign_time,omitempty"`
	ValidTime           *time.Time `json:"valid_time,omitempty"`
	InvalidTime         *time.Time `json:"invalid_time,omitempty"`
	PeriodType          string     `json:"period_type"`
	Period              int        `json:"period"`
	ExecutionTime       *time.Time `json:"execution_time,omitempty"`
	SingleAmount        string     `json:"single_amount"`
	TotalAmount         string     `json:"total_amount,omitempty"`
	TotalPayments       int        `json:"total_payments"`
	CurrentPeriod       int        `json:"current_period"`
	LastDeductTime      *time.Time `json:"last_deduct_time,omitempty"`
	NextDeductTime      *time.Time `json:"next_deduct_time,omitempty"`
	DeductSuccessCount  int        `json:"deduct_success_count"`
	DeductFailCount     int        `json:"deduct_fail_count"`
}

type CancelAlipaySubscriptionRequest struct {
	OutRequestNo string `json:"out_request_no"` // 商户签约号
	AgreementNo  string `json:"agreement_no"`   // 支付宝协议号
	CancelReason string `json:"cancel_reason" binding:"required"`
}
