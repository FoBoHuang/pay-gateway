package services

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	alipay "github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"pay-gateway/internal/cache"
	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

const (
	lockKeyPrefixNotify = "alipay:notify:lock:"
	lockKeyPrefixDeduct = "alipay:deduct:lock:"
	lockKeyExpiration   = 30 * time.Second
)

// AlipayService 支付宝支付服务
type AlipayService struct {
	client *alipay.Client
	db     *gorm.DB
	config *config.AlipayConfig
	redis  *cache.Redis // 可选，用于分布式锁
}

// NewAlipayService 创建支付宝支付服务
// redis 可选，传入 nil 时不使用分布式锁
func NewAlipayService(db *gorm.DB, cfg *config.AlipayConfig, redis *cache.Redis) (*AlipayService, error) {
	// 创建支付宝客户端（直接使用私钥字符串）
	client, err := alipay.New(cfg.AppID, cfg.PrivateKey, cfg.IsProduction)
	if err != nil {
		return nil, fmt.Errorf("创建支付宝客户端失败: %v", err)
	}

	// 加载验签密钥：证书模式与公钥模式二选一（回调验签必需）
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
	} else {
		if cfg.AlipayPublicKey == "" {
			return nil, errors.New("公钥模式(cert_mode=false)下需配置 alipay_public_key 以验签回调")
		}
		if err := client.LoadAliPayPublicKey(cfg.AlipayPublicKey); err != nil {
			return nil, fmt.Errorf("加载支付宝公钥失败: %v", err)
		}
	}

	return &AlipayService{
		client: client,
		db:     db,
		config: cfg,
		redis:  redis,
	}, nil
}

// CreateOrder 创建支付宝订单
func (s *AlipayService) CreateOrder(ctx context.Context, req *CreateAlipayOrderRequest) (*CreateAlipayOrderResponse, error) {
	// 1. 防重复下单：检查是否存在同用户、同商品、未支付的待支付订单（且未过期）
	if !req.AllowDuplicate {
		var existingOrder models.Order
		err := s.db.Where("user_id = ? AND product_id = ? AND payment_method = ? AND payment_status = ?",
			req.UserID, req.ProductID, models.PaymentMethodAlipay, models.PaymentStatusPending).
			Where("(expired_at IS NULL OR expired_at > ?)", time.Now()).
			Order("created_at DESC").
			First(&existingOrder).Error
		if err == nil {
			// 存在可复用的待支付订单，直接返回
			var ap models.AlipayPayment
			if s.db.Where("order_id = ?", existingOrder.ID).First(&ap).Error == nil {
				return &CreateAlipayOrderResponse{
					OrderID:     existingOrder.ID,
					OrderNo:     existingOrder.OrderNo,
					TotalAmount: existingOrder.TotalAmount,
					Subject:     existingOrder.Title,
					Description: existingOrder.Description,
				}, nil
			}
		}
	}

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

// validateOrderForPayment 校验订单是否可创建支付（未支付、未过期）
func (s *AlipayService) validateOrderForPayment(order *models.Order) error {
	if order.PaymentStatus != models.PaymentStatusPending {
		return fmt.Errorf("订单已支付，无法重复创建支付")
	}
	if order.ExpiredAt != nil && order.ExpiredAt.Before(time.Now()) {
		return fmt.Errorf("订单已过期，请重新下单")
	}
	return nil
}

// CreateWapPayment 创建手机网站支付
func (s *AlipayService) CreateWapPayment(ctx context.Context, orderNo string) (string, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return "", fmt.Errorf("订单不存在: %v", err)
	}

	// 创建支付前校验：未支付、未过期
	if err := s.validateOrderForPayment(&order); err != nil {
		return "", err
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

	// 创建支付前校验：未支付、未过期
	if err := s.validateOrderForPayment(&order); err != nil {
		return "", err
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

// CreateAppPayment 创建App支付
func (s *AlipayService) CreateAppPayment(ctx context.Context, orderNo string) (string, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return "", fmt.Errorf("订单不存在: %v", err)
	}

	// 创建支付前校验：未支付、未过期
	if err := s.validateOrderForPayment(&order); err != nil {
		return "", err
	}

	// 查询支付宝支付记录
	var alipayPayment models.AlipayPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&alipayPayment).Error; err != nil {
		return "", fmt.Errorf("支付宝支付记录不存在: %v", err)
	}

	// 构建支付请求
	p := alipay.TradeAppPay{}
	p.NotifyURL = s.config.NotifyURL
	p.Subject = alipayPayment.Subject
	p.OutTradeNo = alipayPayment.OutTradeNo
	p.TotalAmount = alipayPayment.TotalAmount
	p.ProductCode = "QUICK_MSECURITY_PAY"
	p.TimeoutExpress = alipayPayment.TimeoutExpress

	// 生成支付参数字符串
	payParam, err := s.client.TradeAppPay(p)
	if err != nil {
		return "", fmt.Errorf("创建App支付参数失败: %v", err)
	}

	return payParam, nil
}

// HandleNotify 处理支付宝异步通知
func (s *AlipayService) HandleNotify(ctx context.Context, notifyData map[string]string) error {
	// 1. 验证签名
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
	totalAmountStr := notifyData["total_amount"]

	// 2. 分布式锁：保证同一订单的并发通知串行处理
	if s.redis != nil {
		lockKey := lockKeyPrefixNotify + outTradeNo
		ok, lockErr := s.redis.SetNX(ctx, lockKey, "1", lockKeyExpiration)
		if lockErr != nil {
			return fmt.Errorf("获取分布式锁失败: %w", lockErr)
		}
		if !ok {
			return errors.New("订单正在处理中，请稍后重试")
		}
		// 处理完成后主动释放锁，避免锁占用过久
		defer func() { _ = s.redis.Del(context.Background(), lockKey) }()
	}

	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", outTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %v", err)
	}

	// 3. 幂等性：订单已支付完成直接返回成功
	if order.PaymentStatus == models.PaymentStatusCompleted {
		return nil
	}

	// 4. 金额校验：防止通知中的金额与订单金额不一致
	if totalAmountStr != "" {
		notifyAmountFen, parseErr := parseAmountFromYuan(totalAmountStr)
		if parseErr != nil {
			return fmt.Errorf("解析通知金额失败: %w", parseErr)
		}
		if notifyAmountFen != order.TotalAmount {
			return fmt.Errorf("金额校验失败: 通知金额=%d(分) 与订单金额=%d(分) 不一致", notifyAmountFen, order.TotalAmount)
		}
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

// SyncPendingOrders 服务端主动查询兜底：轮询待支付支付宝订单，向支付宝查询并同步状态
func (s *AlipayService) SyncPendingOrders(ctx context.Context) (syncedCount int, err error) {
	now := time.Now()
	var orders []models.Order
	err = s.db.WithContext(ctx).
		Where("payment_method = ? AND payment_status = ? AND status = ?",
			models.PaymentMethodAlipay, models.PaymentStatusPending, models.OrderStatusCreated).
		Where("(expired_at IS NULL OR expired_at > ?)", now).
		Order("created_at ASC").
		Limit(50).
		Find(&orders).Error
	if err != nil {
		return 0, fmt.Errorf("查询待支付订单失败: %w", err)
	}
	for _, order := range orders {
		if _, qErr := s.QueryOrder(ctx, order.OrderNo); qErr != nil {
			continue
		}
		syncedCount++
	}
	return syncedCount, nil
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

	// 退款幂等：确定 out_request_no，支持调用方传入以实现重试幂等
	refundRequestNo := req.OutRequestNo
	if refundRequestNo == "" {
		refundRequestNo = generateRefundRequestNo(req.OrderNo)
	}

	// 幂等检查：若该退款请求号已处理成功，直接返回
	var existingRefund models.AlipayRefund
	if err := s.db.Where("out_request_no = ?", refundRequestNo).First(&existingRefund).Error; err == nil {
		if existingRefund.RefundStatus == "REFUND_SUCCESS" {
			return &RefundResponse{
				RefundRequestNo: existingRefund.OutRequestNo,
				RefundAmount:    parseRefundAmount(existingRefund.RefundAmount),
				RefundStatus:    existingRefund.RefundStatus,
				RefundAt:        existingRefund.GmtRefundPay,
			}, nil
		}
	}

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

// parseRefundAmount 将退款金额字符串（元）解析为分，解析失败返回 0
func parseRefundAmount(s string) int64 {
	fen, _ := parseAmountFromYuan(s)
	return fen
}

// parseAmountFromYuan 将支付宝金额字符串（元，如 "29.99"）解析为分（int64）
func parseAmountFromYuan(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("金额字符串为空")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("解析金额失败: %w", err)
	}
	// 转为分，四舍五入避免浮点误差
	return int64(f*100 + 0.5), nil
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

	// 构建签约请求参数
	p := alipay.AgreementPageSign{}
	p.NotifyURL = s.config.NotifyURL
	p.ReturnURL = s.config.ReturnURL
	p.PersonalProductCode = req.PersonalProductCode
	p.SignScene = req.SignScene
	p.ExternalAgreementNo = outRequestNo

	// 设置周期扣款规则
	p.PeriodRuleParams = &alipay.PeriodRuleParams{
		PeriodType:    req.PeriodType,
		Period:        fmt.Sprintf("%d", req.Period),
		ExecuteTime:   executionTime.Format("2006-01-02"),
		SingleAmount:  formatAmount(req.SingleAmount),
		TotalAmount:   formatAmount(req.TotalAmount),
		TotalPayments: req.TotalPayments,
	}

	// 设置产品信息
	p.AccessParams = &alipay.AccessParams{
		Channel: "ALIPAYAPP", // 或 PCWEB、QRCODE 根据场景选择
	}

	// 生成签约URL
	signURL, err := s.client.AgreementPageSign(p)
	if err != nil {
		return nil, fmt.Errorf("生成签约URL失败: %v", err)
	}

	return &CreateAlipaySubscriptionResponse{
		OrderID:       order.ID,
		OutRequestNo:  outRequestNo,
		SignURL:       signURL.String(),
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

	// 如果已有协议号，调用支付宝API查询最新状态
	if subscription.AgreementNo != "" {
		p := alipay.AgreementQuery{}
		p.AgreementNo = subscription.AgreementNo

		result, err := s.client.AgreementQuery(ctx, p)
		if err != nil {
			// API调用失败，返回本地数据
			return s.buildSubscriptionResponse(&subscription), nil
		}

		// 更新本地数据
		if result.Code.IsSuccess() {
			subscription.Status = result.Status
			if result.SignTime != "" {
				if t, err := time.Parse("2006-01-02 15:04:05", result.SignTime); err == nil {
					subscription.SignTime = &t
				}
			}
			if result.ValidTime != "" {
				if t, err := time.Parse("2006-01-02 15:04:05", result.ValidTime); err == nil {
					subscription.ValidTime = &t
				}
			}
			if result.InvalidTime != "" {
				if t, err := time.Parse("2006-01-02 15:04:05", result.InvalidTime); err == nil {
					subscription.InvalidTime = &t
				}
			}
			// 保存更新
			s.db.Save(&subscription)
		}
	}

	return s.buildSubscriptionResponse(&subscription), nil
}

// ==================== 免密支付（商户代扣）功能 ====================

const (
	withholdOutRequestNoPrefix = "WH"
	defaultWithholdProductCode = "GENERAL_WITHHOLDING_P"
	defaultWithholdSignScene   = "DEFAULT|DEFAULT"
)

// getWithholdNotifyURL 获取免密签约通知URL
func (s *AlipayService) getWithholdNotifyURL() string {
	if s.config.WithholdNotifyURL != "" {
		return s.config.WithholdNotifyURL
	}
	// 从 NotifyURL 派生：将 /notify 替换为 /withhold，若无则追加 /withhold
	if strings.HasSuffix(s.config.NotifyURL, "/notify") {
		return strings.TrimSuffix(s.config.NotifyURL, "/notify") + "/withhold"
	}
	return strings.TrimSuffix(s.config.NotifyURL, "/") + "/withhold"
}

// CreateWithholdAgreement 创建免密签约（商户代扣）
func (s *AlipayService) CreateWithholdAgreement(ctx context.Context, req *CreateWithholdAgreementRequest) (*CreateWithholdAgreementResponse, error) {
	outRequestNo := fmt.Sprintf("%s%s%s", withholdOutRequestNoPrefix, time.Now().Format("20060102150405"), uuid.New().String()[:8])

	productCode := defaultWithholdProductCode
	if req.PersonalProductCode != "" {
		productCode = req.PersonalProductCode
	}
	signScene := defaultWithholdSignScene
	if req.SignScene != "" {
		signScene = req.SignScene
	}

	agreement := &models.AlipayWithholdAgreement{
		UserID:              req.UserID,
		OutRequestNo:        outRequestNo,
		Status:              "TEMP",
		AppID:               s.config.AppID,
		PersonalProductCode: productCode,
		SignScene:           signScene,
	}

	if err := s.db.Create(agreement).Error; err != nil {
		return nil, fmt.Errorf("创建免密签约记录失败: %v", err)
	}

	p := alipay.AgreementPageSign{}
	p.NotifyURL = s.getWithholdNotifyURL()
	p.ReturnURL = s.config.ReturnURL
	p.PersonalProductCode = productCode
	p.SignScene = signScene
	p.ExternalAgreementNo = outRequestNo
	p.AccessParams = &alipay.AccessParams{Channel: "ALIPAYAPP"}

	signURL, err := s.client.AgreementPageSign(p)
	if err != nil {
		return nil, fmt.Errorf("生成签约URL失败: %v", err)
	}

	return &CreateWithholdAgreementResponse{
		OutRequestNo: outRequestNo,
		SignURL:      signURL.String(),
		Status:       "TEMP",
	}, nil
}

// HandleWithholdNotify 处理免密签约通知
func (s *AlipayService) HandleWithholdNotify(ctx context.Context, notifyData map[string]string) error {
	formData := url.Values{}
	for k, v := range notifyData {
		formData.Set(k, v)
	}
	if err := s.client.VerifySign(formData); err != nil {
		return errors.New("签名验证失败")
	}

	outRequestNo := notifyData["out_request_no"]
	agreementNo := notifyData["agreement_no"]
	status := notifyData["status"]

	var agreement models.AlipayWithholdAgreement
	if err := s.db.Where("out_request_no = ?", outRequestNo).First(&agreement).Error; err != nil {
		return fmt.Errorf("免密签约记录不存在: %v", err)
	}

	agreement.AgreementNo = agreementNo
	agreement.Status = status

	if status == "NORMAL" {
		if signTime, ok := notifyData["sign_time"]; ok {
			if t, err := time.Parse("2006-01-02 15:04:05", signTime); err == nil {
				agreement.SignTime = &t
			}
		}
		if validTime, ok := notifyData["valid_time"]; ok {
			if t, err := time.Parse("2006-01-02 15:04:05", validTime); err == nil {
				agreement.ValidTime = &t
			}
		}
		if invalidTime, ok := notifyData["invalid_time"]; ok {
			if t, err := time.Parse("2006-01-02 15:04:05", invalidTime); err == nil {
				agreement.InvalidTime = &t
			}
		}
	} else if status == "STOP" {
		now := time.Now()
		agreement.CancelTime = &now
	}

	return s.db.Save(&agreement).Error
}

// ExecuteWithhold 执行单次代扣（免密支付）
func (s *AlipayService) ExecuteWithhold(ctx context.Context, req *ExecuteWithholdRequest) (*ExecuteWithholdResponse, error) {
	var agreement models.AlipayWithholdAgreement
	if err := s.db.Where("agreement_no = ? AND user_id = ? AND status = ?", req.AgreementNo, req.UserID, "NORMAL").First(&agreement).Error; err != nil {
		return nil, fmt.Errorf("免密协议不存在或已失效: %v", err)
	}

	orderNo := generateOrderNo()

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
	}

	tx := s.db.Begin()
	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建订单失败: %v", err)
	}

	alipayPayment := &models.AlipayPayment{
		OrderID:     order.ID,
		OutTradeNo:  orderNo,
		TotalAmount: formatAmount(req.TotalAmount),
		Subject:     req.Subject,
		Body:        req.Body,
		TradeStatus: "WAIT_BUYER_PAY",
		AppID:       s.config.AppID,
		AgreementNo: agreement.AgreementNo,
	}
	if err := tx.Create(alipayPayment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建支付记录失败: %v", err)
	}

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

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	// 调用支付宝代扣接口 alipay.trade.pay
	payParam := alipay.TradePay{}
	payParam.OutTradeNo = orderNo
	payParam.Subject = req.Subject
	payParam.TotalAmount = formatAmount(req.TotalAmount)
	payParam.Body = req.Body
	payParam.ProductCode = "AGREEMENT_PAYMENT"
	payParam.NotifyURL = s.config.NotifyURL
	payParam.AgreementParams = &alipay.AgreementParams{AgreementNo: req.AgreementNo}
	payParam.Scene = "bar_code"
	payParam.AuthCode = ""

	result, err := s.client.TradePay(ctx, payParam)
	if err != nil {
		return nil, fmt.Errorf("代扣请求失败: %v", err)
	}

	if !result.Code.IsSuccess() {
		return nil, fmt.Errorf("代扣失败: %s - %s", result.Code, result.Msg)
	}

	now := time.Now()
	order.Status = models.OrderStatusPaid
	order.PaymentStatus = models.PaymentStatusCompleted
	order.PaidAt = &now
	s.db.Model(&order).Updates(map[string]interface{}{
		"status":         order.Status,
		"payment_status": order.PaymentStatus,
		"paid_at":        order.PaidAt,
	})

	alipayPayment.TradeNo = result.TradeNo
	alipayPayment.TradeStatus = "TRADE_SUCCESS"
	alipayPayment.BuyerUserID = result.BuyerUserId
	alipayPayment.BuyerLogonID = result.BuyerLogonId
	if t, err := time.Parse("2006-01-02 15:04:05", result.GmtPayment); err == nil {
		alipayPayment.TimeEnd = &t
	}
	s.db.Save(alipayPayment)

	transaction.Status = models.PaymentStatusCompleted
	transaction.ProcessedAt = &now
	s.db.Save(transaction)

	return &ExecuteWithholdResponse{
		OrderNo:     orderNo,
		TradeNo:     result.TradeNo,
		TotalAmount: req.TotalAmount,
		TradeStatus: "TRADE_SUCCESS",
		PaidAt:      &now,
	}, nil
}

// QueryWithholdAgreement 查询免密签约状态
func (s *AlipayService) QueryWithholdAgreement(ctx context.Context, outRequestNo string) (*QueryWithholdAgreementResponse, error) {
	var agreement models.AlipayWithholdAgreement
	if err := s.db.Where("out_request_no = ?", outRequestNo).First(&agreement).Error; err != nil {
		return nil, fmt.Errorf("免密签约记录不存在: %v", err)
	}

	if agreement.AgreementNo != "" {
		p := alipay.AgreementQuery{}
		p.AgreementNo = agreement.AgreementNo
		if result, err := s.client.AgreementQuery(ctx, p); err == nil && result.Code.IsSuccess() {
			agreement.Status = result.Status
			if result.SignTime != "" {
				if t, err := time.Parse("2006-01-02 15:04:05", result.SignTime); err == nil {
					agreement.SignTime = &t
				}
			}
			s.db.Save(&agreement)
		}
	}

	return &QueryWithholdAgreementResponse{
		OutRequestNo: agreement.OutRequestNo,
		AgreementNo:  agreement.AgreementNo,
		Status:       agreement.Status,
		SignTime:     agreement.SignTime,
	}, nil
}

// buildSubscriptionResponse 构建订阅查询响应
func (s *AlipayService) buildSubscriptionResponse(subscription *models.AlipaySubscription) *QueryAlipaySubscriptionResponse {
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
	}
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

	// 必须有协议号才能解约
	if subscription.AgreementNo == "" {
		return errors.New("协议尚未签约完成，无法解约")
	}

	// 调用支付宝解约API
	p := alipay.AgreementUnsign{}
	p.AgreementNo = subscription.AgreementNo
	p.ExternalAgreementNo = subscription.OutRequestNo

	result, err := s.client.AgreementUnsign(ctx, p)
	if err != nil {
		return fmt.Errorf("调用支付宝解约接口失败: %v", err)
	}

	// 检查返回结果
	if !result.Code.IsSuccess() {
		return fmt.Errorf("支付宝解约失败: %s - %s", result.Code, result.Msg)
	}

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

	// 2. 分布式锁：以 out_trade_no 为键，保证同一扣款通知串行处理
	lockKey := lockKeyPrefixDeduct + outTradeNo
	if s.redis != nil {
		ok, lockErr := s.redis.SetNX(ctx, lockKey, "1", lockKeyExpiration)
		if lockErr != nil {
			return fmt.Errorf("获取分布式锁失败: %w", lockErr)
		}
		if !ok {
			return errors.New("扣款正在处理中，请稍后重试")
		}
		defer func() { _ = s.redis.Del(context.Background(), lockKey) }()
	}

	// 3. 幂等性：检查 trade_no 或 out_trade_no 是否已处理
	var existingRecord models.AlipayDeductRecord
	if err := s.db.Where("trade_no = ? OR out_trade_no = ?", tradeNo, outTradeNo).First(&existingRecord).Error; err == nil {
		return nil // 已处理过，直接返回成功
	}

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

	// 开启事务
	tx := s.db.Begin()

	if err := tx.Save(&subscription).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新周期扣款状态失败: %v", err)
	}

	// 创建扣款记录（用于幂等去重，替代原 AlipayPayment 避免 OrderID 唯一约束冲突）
	deductRecord := &models.AlipayDeductRecord{
		SubscriptionID: subscription.ID,
		OrderID:        subscription.OrderID,
		AgreementNo:    agreementNo,
		OutTradeNo:     outTradeNo,
		TradeNo:        tradeNo,
		Amount:         amount,
		Status:         status,
		DeductTime:     &now,
	}
	if err := tx.Create(deductRecord).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("创建扣款记录失败: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}

	return nil
}

// ==================== 请求和响应结构体 ====================

type CreateAlipayOrderRequest struct {
	UserID         uint   `json:"user_id" binding:"required"`
	ProductID      string `json:"product_id" binding:"required"`
	Subject        string `json:"subject" binding:"required"`
	Body           string `json:"body"`
	TotalAmount    int64  `json:"total_amount" binding:"required,min=1"`
	AllowDuplicate bool   `json:"allow_duplicate"` // 是否允许重复下单，默认 false 时复用已有待支付订单
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
	OutRequestNo string `json:"out_request_no"` // 可选，退款请求号，传入相同值可实现重试幂等
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

// 免密支付（商户代扣）相关结构体

type CreateWithholdAgreementRequest struct {
	UserID              uint   `json:"user_id" binding:"required"`
	PersonalProductCode string `json:"personal_product_code"` // 默认 GENERAL_WITHHOLDING_P
	SignScene           string `json:"sign_scene"`            // 默认 DEFAULT|DEFAULT
}

type CreateWithholdAgreementResponse struct {
	OutRequestNo string `json:"out_request_no"`
	SignURL      string `json:"sign_url"`
	Status       string `json:"status"`
}

type ExecuteWithholdRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	AgreementNo string `json:"agreement_no" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	Body        string `json:"body"`
	TotalAmount int64  `json:"total_amount" binding:"required,min=1"`
}

type ExecuteWithholdResponse struct {
	OrderNo     string     `json:"order_no"`
	TradeNo     string     `json:"trade_no"`
	TotalAmount int64      `json:"total_amount"`
	TradeStatus string     `json:"trade_status"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

type QueryWithholdAgreementResponse struct {
	OutRequestNo string     `json:"out_request_no"`
	AgreementNo  string     `json:"agreement_no"`
	Status       string     `json:"status"`
	SignTime     *time.Time `json:"sign_time,omitempty"`
}
