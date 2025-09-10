package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"google-play-billing/internal/config"
	"google-play-billing/internal/models"
)

// PaymentService 支付服务接口
type PaymentService interface {
	// 订单相关
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*models.Order, error)
	GetOrder(ctx context.Context, orderID uint) (*models.Order, error)
	GetOrderByOrderNo(ctx context.Context, orderNo string) (*models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uint, status models.OrderStatus) error
	CancelOrder(ctx context.Context, orderID uint, reason string) error

	// 支付相关
	ProcessPayment(ctx context.Context, req *ProcessPaymentRequest) (*models.PaymentTransaction, error)
	ConfirmPayment(ctx context.Context, orderID uint, transactionID string, providerData models.JSON) error
	RefundPayment(ctx context.Context, orderID uint, amount int64, reason string) error

	// 查询相关
	GetUserOrders(ctx context.Context, userID uint, page, pageSize int) ([]*models.Order, int64, error)
	GetOrderTransactions(ctx context.Context, orderID uint) ([]*models.PaymentTransaction, error)
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	UserID           uint                 `json:"user_id" binding:"required"`
	ProductID        string               `json:"product_id" binding:"required"`
	Type             models.OrderType     `json:"type" binding:"required"`
	Title            string               `json:"title" binding:"required"`
	Description      string               `json:"description"`
	Quantity         int                  `json:"quantity" binding:"required,min=1"`
	Currency         string               `json:"currency" binding:"required,len=3"`
	TotalAmount      int64                `json:"total_amount" binding:"required,min=0"`
	PaymentMethod    models.PaymentMethod `json:"payment_method" binding:"required"`
	DeveloperPayload string               `json:"developer_payload"`
}

// ProcessPaymentRequest 处理支付请求
type ProcessPaymentRequest struct {
	OrderID          uint                   `json:"order_id" binding:"required"`
	Provider         models.PaymentProvider `json:"provider" binding:"required"`
	PurchaseToken    string                 `json:"purchase_token" binding:"required"`
	DeveloperPayload string                 `json:"developer_payload"`
}

// paymentServiceImpl 支付服务实现
type paymentServiceImpl struct {
	db            *gorm.DB
	config        *config.Config
	logger        *zap.Logger
	googleService *GooglePlayService
}

// NewPaymentService 创建支付服务
func NewPaymentService(db *gorm.DB, cfg *config.Config, logger *zap.Logger, googleService *GooglePlayService) PaymentService {
	return &paymentServiceImpl{
		db:            db,
		config:        cfg,
		logger:        logger,
		googleService: googleService,
	}
}

// CreateOrder 创建订单
func (s *paymentServiceImpl) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*models.Order, error) {
	// 生成订单号
	orderNo := s.generateOrderNo()

	// 设置过期时间（默认30分钟）
	expiredAt := time.Now().Add(30 * time.Minute)

	order := &models.Order{
		OrderNo:          orderNo,
		UserID:           req.UserID,
		ProductID:        req.ProductID,
		Type:             req.Type,
		Title:            req.Title,
		Description:      req.Description,
		Quantity:         req.Quantity,
		Currency:         req.Currency,
		TotalAmount:      req.TotalAmount,
		Status:           models.OrderStatusCreated,
		PaymentMethod:    req.PaymentMethod,
		PaymentStatus:    models.PaymentStatusPending,
		ExpiredAt:        &expiredAt,
		DeveloperPayload: req.DeveloperPayload,
	}

	// 开始事务
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建订单
	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		s.logger.Error("创建订单失败", zap.Error(err), zap.Any("request", req))
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		s.logger.Error("提交订单事务失败", zap.Error(err))
		return nil, fmt.Errorf("提交订单事务失败: %w", err)
	}

	s.logger.Info("订单创建成功",
		zap.Uint("order_id", order.ID),
		zap.String("order_no", order.OrderNo),
		zap.Uint("user_id", order.UserID))

	return order, nil
}

// GetOrder 根据ID获取订单
func (s *paymentServiceImpl) GetOrder(ctx context.Context, orderID uint) (*models.Order, error) {
	var order models.Order
	err := s.db.WithContext(ctx).
		Preload("User").
		Preload("GooglePayment").
		First(&order, orderID).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("订单不存在: %d", orderID)
		}
		s.logger.Error("获取订单失败", zap.Error(err), zap.Uint("order_id", orderID))
		return nil, fmt.Errorf("获取订单失败: %w", err)
	}

	return &order, nil
}

// GetOrderByOrderNo 根据订单号获取订单
func (s *paymentServiceImpl) GetOrderByOrderNo(ctx context.Context, orderNo string) (*models.Order, error) {
	var order models.Order
	err := s.db.WithContext(ctx).
		Where("order_no = ?", orderNo).
		Preload("User").
		Preload("GooglePayment").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("订单不存在: %s", orderNo)
		}
		s.logger.Error("获取订单失败", zap.Error(err), zap.String("order_no", orderNo))
		return nil, fmt.Errorf("获取订单失败: %w", err)
	}

	return &order, nil
}

// UpdateOrderStatus 更新订单状态
func (s *paymentServiceImpl) UpdateOrderStatus(ctx context.Context, orderID uint, status models.OrderStatus) error {
	result := s.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("id = ?", orderID).
		Update("status", status)

	if result.Error != nil {
		s.logger.Error("更新订单状态失败", zap.Error(result.Error), zap.Uint("order_id", orderID))
		return fmt.Errorf("更新订单状态失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("订单不存在: %d", orderID)
	}

	s.logger.Info("订单状态更新成功",
		zap.Uint("order_id", orderID),
		zap.String("status", string(status)))

	return nil
}

// CancelOrder 取消订单
func (s *paymentServiceImpl) CancelOrder(ctx context.Context, orderID uint, reason string) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取订单
	var order models.Order
	if err := tx.First(&order, orderID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("订单不存在: %d", orderID)
		}
		return fmt.Errorf("获取订单失败: %w", err)
	}

	// 检查订单状态是否可以取消
	if order.Status != models.OrderStatusCreated && order.Status != models.OrderStatusPaid {
		tx.Rollback()
		return fmt.Errorf("订单状态不允许取消: %s", order.Status)
	}

	// 更新订单状态
	order.Status = models.OrderStatusCancelled
	order.RefundReason = reason
	now := time.Now()
	order.RefundAt = &now

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新订单失败: %w", err)
	}

	// 如果订单已支付，需要退款
	if order.PaymentStatus == models.PaymentStatusCompleted {
		// 这里可以调用退款服务
		s.logger.Warn("订单已支付，需要退款处理",
			zap.Uint("order_id", orderID),
			zap.String("reason", reason))
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	s.logger.Info("订单取消成功",
		zap.Uint("order_id", orderID),
		zap.String("reason", reason))

	return nil
}

// ProcessPayment 处理支付
func (s *paymentServiceImpl) ProcessPayment(ctx context.Context, req *ProcessPaymentRequest) (*models.PaymentTransaction, error) {
	// 获取订单
	order, err := s.GetOrder(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}

	// 检查订单状态
	if order.Status != models.OrderStatusCreated {
		return nil, fmt.Errorf("订单状态不允许支付: %s", order.Status)
	}

	// 检查订单是否过期
	if order.ExpiredAt != nil && order.ExpiredAt.Before(time.Now()) {
		return nil, fmt.Errorf("订单已过期")
	}

	// 创建支付交易记录
	transaction := &models.PaymentTransaction{
		OrderID:       order.ID,
		TransactionID: s.generateTransactionID(),
		Provider:      req.Provider,
		Type:          "PAYMENT",
		Amount:        order.TotalAmount,
		Currency:      order.Currency,
		Status:        models.PaymentStatusPending,
		ProviderData: models.JSON{
			"purchase_token":    req.PurchaseToken,
			"developer_payload": req.DeveloperPayload,
		},
	}

	// 开始事务
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建交易记录
	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		s.logger.Error("创建支付交易记录失败", zap.Error(err))
		return nil, fmt.Errorf("创建支付交易记录失败: %w", err)
	}

	// 根据提供商处理支付
	switch req.Provider {
	case models.PaymentProviderGooglePlay:
		if err := s.processGooglePlayPayment(ctx, tx, order, transaction, req.PurchaseToken); err != nil {
			tx.Rollback()
			return nil, err
		}
	default:
		tx.Rollback()
		return nil, fmt.Errorf("不支持的支付提供商: %s", req.Provider)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	s.logger.Info("支付处理成功",
		zap.Uint("order_id", order.ID),
		zap.String("transaction_id", transaction.TransactionID),
		zap.String("provider", string(req.Provider)))

	return transaction, nil
}

// processGooglePlayPayment 处理Google Play支付
func (s *paymentServiceImpl) processGooglePlayPayment(ctx context.Context, tx *gorm.DB, order *models.Order, transaction *models.PaymentTransaction, purchaseToken string) error {
	// 根据订单类型调用不同的验证方法
	var providerData models.JSON

	switch order.Type {
	case models.OrderTypePurchase:
		// 验证一次性购买
		purchase, err := s.googleService.VerifyPurchase(ctx, order.ProductID, purchaseToken)
		if err != nil {
			transaction.Status = models.PaymentStatusFailed
			transaction.ErrorMessage = &[]string{err.Error()}[0]
			tx.Save(transaction)
			return fmt.Errorf("验证购买失败: %w", err)
		}

		// 确认购买
		if err := s.googleService.AcknowledgePurchase(ctx, order.ProductID, purchaseToken, order.DeveloperPayload); err != nil {
			transaction.Status = models.PaymentStatusFailed
			transaction.ErrorMessage = &[]string{err.Error()}[0]
			tx.Save(transaction)
			return fmt.Errorf("确认购买失败: %w", err)
		}

		// 创建Google支付记录
		googlePayment := &models.GooglePayment{
			OrderID:              order.ID,
			PurchaseToken:        purchaseToken,
			OrderIDGoogle:        purchase.OrderId,
			ProductIDGoogle:      order.ProductID,
			PurchaseState:        purchase.PurchaseState,
			ConsumptionState:     purchase.ConsumptionState,
			AcknowledgementState: purchase.AcknowledgementState,
			PurchaseTimeMillis:   purchase.PurchaseTimeMillis,
			ObfuscatedAccountID:  purchase.ObfuscatedExternalAccountId,
			ObfuscatedProfileID:  purchase.ObfuscatedExternalProfileId,
			RegionCode:           purchase.RegionCode,
		}

		if err := tx.Create(googlePayment).Error; err != nil {
			return fmt.Errorf("创建Google支付记录失败: %w", err)
		}

		providerData = models.JSON{
			"purchase_response": purchase,
			"acknowledged":      true,
		}

	case models.OrderTypeSubscription:
		// 验证订阅
		subscription, err := s.googleService.VerifySubscription(ctx, order.ProductID, purchaseToken)
		if err != nil {
			transaction.Status = models.PaymentStatusFailed
			transaction.ErrorMessage = &[]string{err.Error()}[0]
			tx.Save(transaction)
			return fmt.Errorf("验证订阅失败: %w", err)
		}

		// 确认订阅
		if err := s.googleService.AcknowledgeSubscription(ctx, order.ProductID, purchaseToken, order.DeveloperPayload); err != nil {
			transaction.Status = models.PaymentStatusFailed
			transaction.ErrorMessage = &[]string{err.Error()}[0]
			tx.Save(transaction)
			return fmt.Errorf("确认订阅失败: %w", err)
		}

		// 创建Google支付记录
		googlePayment := &models.GooglePayment{
			OrderID:              order.ID,
			PurchaseToken:        purchaseToken,
			OrderIDGoogle:        subscription.OrderId,
			ProductIDGoogle:      order.ProductID,
			AutoRenewing:         &subscription.AutoRenewing,
			CountryCode:          subscription.CountryCode,
			PriceAmountMicros:    subscription.PriceAmountMicros,
			AcknowledgementState: subscription.AcknowledgementState,
			ObfuscatedAccountID:  subscription.ObfuscatedExternalAccountId,
			ObfuscatedProfileID:  subscription.ObfuscatedExternalProfileId,
		}

		if subscription.CancelReason != 0 {
			cancelReason := subscription.CancelReason
			googlePayment.CancelReason = &cancelReason
		}
		if subscription.UserCancellationTimeMillis != "" {
			if cancelTime, err := time.Parse(time.RFC3339, subscription.UserCancellationTimeMillis); err == nil {
				googlePayment.UserCancellationTime = &cancelTime
			}
		}

		if err := tx.Create(googlePayment).Error; err != nil {
			return fmt.Errorf("创建Google支付记录失败: %w", err)
		}

		providerData = models.JSON{
			"subscription_response": subscription,
			"acknowledged":          true,
		}

	default:
		return fmt.Errorf("不支持的订单类型: %s", order.Type)
	}

	// 更新交易记录状态
	transaction.Status = models.PaymentStatusCompleted
	transaction.ProcessedAt = &time.Time{}
	*transaction.ProcessedAt = time.Now()
	transaction.ProviderData = providerData

	if err := tx.Save(transaction).Error; err != nil {
		return fmt.Errorf("更新交易记录失败: %w", err)
	}

	// 更新订单状态
	order.PaymentStatus = models.PaymentStatusCompleted
	now := time.Now()
	order.PaidAt = &now
	order.Status = models.OrderStatusPaid

	if err := tx.Save(order).Error; err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	return nil
}

// ConfirmPayment 确认支付（用于外部确认）
func (s *paymentServiceImpl) ConfirmPayment(ctx context.Context, orderID uint, transactionID string, providerData models.JSON) error {
	// 获取交易记录
	var transaction models.PaymentTransaction
	err := s.db.WithContext(ctx).
		Where("order_id = ? AND transaction_id = ?", orderID, transactionID).
		First(&transaction).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("交易记录不存在")
		}
		return fmt.Errorf("获取交易记录失败: %w", err)
	}

	// 检查交易状态
	if transaction.Status != models.PaymentStatusPending {
		return fmt.Errorf("交易状态不允许确认: %s", transaction.Status)
	}

	// 更新交易记录
	now := time.Now()
	transaction.Status = models.PaymentStatusCompleted
	transaction.ProcessedAt = &now
	transaction.ProviderData = providerData

	if err := s.db.WithContext(ctx).Save(&transaction).Error; err != nil {
		return fmt.Errorf("更新交易记录失败: %w", err)
	}

	// 更新订单状态
	if err := s.UpdateOrderStatus(ctx, orderID, models.OrderStatusPaid); err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	s.logger.Info("支付确认成功",
		zap.Uint("order_id", orderID),
		zap.String("transaction_id", transactionID))

	return nil
}

// RefundPayment 退款
func (s *paymentServiceImpl) RefundPayment(ctx context.Context, orderID uint, amount int64, reason string) error {
	// 获取订单
	order, err := s.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}

	// 检查订单状态
	if order.Status != models.OrderStatusPaid {
		return fmt.Errorf("订单状态不允许退款: %s", order.Status)
	}

	// 创建退款交易记录
	provider := models.PaymentProviderGooglePlay
	if order.GooglePayment == nil {
		provider = ""
	}

	refundTransaction := &models.PaymentTransaction{
		OrderID:       order.ID,
		TransactionID: s.generateTransactionID(),
		Provider:      provider,
		Type:          "REFUND",
		Amount:        amount,
		Currency:      order.Currency,
		Status:        models.PaymentStatusPending,
		ProviderData: models.JSON{
			"refund_reason":   reason,
			"original_amount": order.TotalAmount,
		},
	}

	// 开始事务
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建退款交易记录
	if err := tx.Create(refundTransaction).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("创建退款交易记录失败: %w", err)
	}

	// 更新订单状态
	order.Status = models.OrderStatusRefunded
	order.RefundReason = reason
	now := time.Now()
	order.RefundAt = &now
	order.RefundAmount = amount

	if err := tx.Save(order).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	s.logger.Info("退款处理成功",
		zap.Uint("order_id", orderID),
		zap.Int64("amount", amount),
		zap.String("reason", reason))

	return nil
}

// GetUserOrders 获取用户订单列表
func (s *paymentServiceImpl) GetUserOrders(ctx context.Context, userID uint, page, pageSize int) ([]*models.Order, int64, error) {
	var orders []*models.Order
	var total int64

	offset := (page - 1) * pageSize

	// 查询总数
	err := s.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("user_id = ?", userID).
		Count(&total).Error

	if err != nil {
		return nil, 0, fmt.Errorf("查询订单总数失败: %w", err)
	}

	// 查询订单列表
	err = s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Preload("GooglePayment").
		Find(&orders).Error

	if err != nil {
		return nil, 0, fmt.Errorf("查询订单列表失败: %w", err)
	}

	return orders, total, nil
}

// GetOrderTransactions 获取订单交易记录
func (s *paymentServiceImpl) GetOrderTransactions(ctx context.Context, orderID uint) ([]*models.PaymentTransaction, error) {
	var transactions []*models.PaymentTransaction

	err := s.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("created_at DESC").
		Find(&transactions).Error

	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}

	return transactions, nil
}

// generateOrderNo 生成订单号
func (s *paymentServiceImpl) generateOrderNo() string {
	return fmt.Sprintf("ORD%s%s",
		time.Now().Format("20060102150405"),
		uuid.New().String()[:8])
}

// generateTransactionID 生成交易ID
func (s *paymentServiceImpl) generateTransactionID() string {
	return fmt.Sprintf("TXN%s%s",
		time.Now().Format("20060102150405"),
		uuid.New().String()[:8])
}
