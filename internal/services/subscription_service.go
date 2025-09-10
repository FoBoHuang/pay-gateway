package services

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"google-play-billing/internal/config"
	"google-play-billing/internal/models"
)

// SubscriptionService 订阅服务接口
type SubscriptionService interface {
	// 订阅管理
	CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*models.Order, error)
	GetSubscription(ctx context.Context, orderID uint) (*SubscriptionInfo, error)
	GetUserSubscriptions(ctx context.Context, userID uint, status *models.SubscriptionState) ([]*SubscriptionInfo, error)

	// 订阅状态管理
	UpdateSubscriptionStatus(ctx context.Context, orderID uint, status models.SubscriptionState, reason string) error
	CancelSubscription(ctx context.Context, orderID uint, reason string) error
	RenewSubscription(ctx context.Context, orderID uint, newExpiryTime time.Time) error

	// 订阅查询
	GetActiveSubscriptions(ctx context.Context, userID uint) ([]*SubscriptionInfo, error)
	GetExpiredSubscriptions(ctx context.Context, userID uint) ([]*SubscriptionInfo, error)

	// 订阅验证
	ValidateSubscription(ctx context.Context, orderID uint) (*SubscriptionValidationResult, error)

	// 订阅统计
	GetSubscriptionStats(ctx context.Context, userID uint) (*SubscriptionStats, error)
}

// CreateSubscriptionRequest 创建订阅请求
type CreateSubscriptionRequest struct {
	UserID           uint   `json:"user_id" binding:"required"`
	ProductID        string `json:"product_id" binding:"required"`
	Title            string `json:"title" binding:"required"`
	Description      string `json:"description"`
	Currency         string `json:"currency" binding:"required,len=3"`
	Price            int64  `json:"price" binding:"required,min=0"`
	Period           string `json:"period" binding:"required"` // 订阅周期，如 "P1M", "P1Y"
	DeveloperPayload string `json:"developer_payload"`
}

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	Order              *models.Order            `json:"order"`
	GooglePayment      *models.GooglePayment    `json:"google_payment,omitempty"`
	CurrentStatus      models.SubscriptionState `json:"current_status"`
	StartTime          time.Time                `json:"start_time"`
	ExpiryTime         time.Time                `json:"expiry_time"`
	AutoRenewing       bool                     `json:"auto_renewing"`
	GracePeriodEndTime *time.Time               `json:"grace_period_end_time,omitempty"`
	CancelReason       *string                  `json:"cancel_reason,omitempty"`
	NextBillingTime    *time.Time               `json:"next_billing_time,omitempty"`
}

// SubscriptionValidationResult 订阅验证结果
type SubscriptionValidationResult struct {
	IsValid        bool                     `json:"is_valid"`
	CurrentStatus  models.SubscriptionState `json:"current_status"`
	ExpiryTime     time.Time                `json:"expiry_time"`
	AutoRenewing   bool                     `json:"auto_renewing"`
	CancelReason   *int                     `json:"cancel_reason,omitempty"`
	ValidationTime time.Time                `json:"validation_time"`
	Message        string                   `json:"message"`
}

// SubscriptionStats 订阅统计
type SubscriptionStats struct {
	TotalSubscriptions     int64 `json:"total_subscriptions"`
	ActiveSubscriptions    int64 `json:"active_subscriptions"`
	ExpiredSubscriptions   int64 `json:"expired_subscriptions"`
	CancelledSubscriptions int64 `json:"cancelled_subscriptions"`
	TotalSpent             int64 `json:"total_spent"` // 总消费金额（微单位）
}

// subscriptionServiceImpl 订阅服务实现
type subscriptionServiceImpl struct {
	db             *gorm.DB
	config         *config.Config
	logger         *zap.Logger
	googleService  *GooglePlayService
	paymentService PaymentService
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService(db *gorm.DB, cfg *config.Config, logger *zap.Logger,
	googleService *GooglePlayService, paymentService PaymentService) SubscriptionService {
	return &subscriptionServiceImpl{
		db:             db,
		config:         cfg,
		logger:         logger,
		googleService:  googleService,
		paymentService: paymentService,
	}
}

// CreateSubscription 创建订阅
func (s *subscriptionServiceImpl) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*models.Order, error) {
	// 创建订单请求
	orderReq := &CreateOrderRequest{
		UserID:           req.UserID,
		ProductID:        req.ProductID,
		Type:             models.OrderTypeSubscription,
		Title:            req.Title,
		Description:      req.Description,
		Quantity:         1, // 订阅数量固定为1
		Currency:         req.Currency,
		TotalAmount:      req.Price,
		PaymentMethod:    models.PaymentMethodGooglePlay,
		DeveloperPayload: req.DeveloperPayload,
	}

	// 调用支付服务创建订单
	order, err := s.paymentService.CreateOrder(ctx, orderReq)
	if err != nil {
		return nil, fmt.Errorf("创建订阅订单失败: %w", err)
	}

	s.logger.Info("订阅订单创建成功",
		zap.Uint("order_id", order.ID),
		zap.Uint("user_id", req.UserID),
		zap.String("product_id", req.ProductID))

	return order, nil
}

// GetSubscription 获取订阅信息
func (s *subscriptionServiceImpl) GetSubscription(ctx context.Context, orderID uint) (*SubscriptionInfo, error) {
	// 获取订单
	order, err := s.paymentService.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// 检查订单类型
	if order.Type != models.OrderTypeSubscription {
		return nil, fmt.Errorf("订单不是订阅类型: %s", order.Type)
	}

	// 获取Google支付详情
	var googlePayment *models.GooglePayment
	if order.GooglePayment != nil {
		googlePayment = order.GooglePayment
	}

	// 计算订阅状态
	currentStatus := s.determineSubscriptionStatus(googlePayment)

	// 构建订阅信息
	info := &SubscriptionInfo{
		Order:         order,
		GooglePayment: googlePayment,
		CurrentStatus: currentStatus,
	}

	// 设置时间和状态信息
	if googlePayment != nil {
		// 开始时间
		if purchaseTime, err := time.Parse(time.RFC3339, googlePayment.PurchaseTimeMillis); err == nil {
			info.StartTime = purchaseTime
		}

		// 到期时间
		if expiryTime, err := time.Parse(time.RFC3339, googlePayment.ExpiryTimeMillis); err == nil {
			info.ExpiryTime = expiryTime

			// 计算下次计费时间（如果自动续订）
			if googlePayment.AutoRenewing != nil && *googlePayment.AutoRenewing {
				nextBilling := info.ExpiryTime
				info.NextBillingTime = &nextBilling
			}
		}

		// 自动续订状态
		if googlePayment.AutoRenewing != nil {
			info.AutoRenewing = *googlePayment.AutoRenewing
		}

		// 宽限期结束时间
		if googlePayment.GracePeriodExpiryTime != nil {
			info.GracePeriodEndTime = googlePayment.GracePeriodExpiryTime
		}

		// 取消原因
		if googlePayment.CancelReason != nil {
			reason := fmt.Sprintf("Cancel reason: %d", *googlePayment.CancelReason)
			info.CancelReason = &reason
		}
	}

	return info, nil
}

// GetUserSubscriptions 获取用户订阅列表
func (s *subscriptionServiceImpl) GetUserSubscriptions(ctx context.Context, userID uint, status *models.SubscriptionState) ([]*SubscriptionInfo, error) {
	// 基础查询：获取用户的订阅订单
	query := s.db.WithContext(ctx).
		Where("user_id = ? AND type = ?", userID, models.OrderTypeSubscription).
		Preload("User").
		Preload("GooglePayment").
		Order("created_at DESC")

	var orders []*models.Order
	err := query.Find(&orders).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户订阅失败: %w", err)
	}

	// 构建订阅信息列表
	var subscriptions []*SubscriptionInfo
	for _, order := range orders {
		// 获取订阅信息
		subscription, err := s.GetSubscription(ctx, order.ID)
		if err != nil {
			s.logger.Warn("获取订阅信息失败", zap.Error(err), zap.Uint("order_id", order.ID))
			continue
		}

		// 如果指定了状态过滤
		if status != nil && subscription.CurrentStatus != *status {
			continue
		}

		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

// UpdateSubscriptionStatus 更新订阅状态
func (s *subscriptionServiceImpl) UpdateSubscriptionStatus(ctx context.Context, orderID uint, status models.SubscriptionState, reason string) error {
	// 获取订阅信息
	subscription, err := s.GetSubscription(ctx, orderID)
	if err != nil {
		return err
	}

	// 记录状态变更
	s.logger.Info("更新订阅状态",
		zap.Uint("order_id", orderID),
		zap.String("old_status", string(subscription.CurrentStatus)),
		zap.String("new_status", string(status)),
		zap.String("reason", reason))

	// 如果Google支付记录存在，更新相关字段
	if subscription.GooglePayment != nil {
		switch status {
		case models.SubscriptionStateCancelled:
			cancelReason := 0 // 用户取消
			subscription.GooglePayment.CancelReason = &cancelReason
			now := time.Now()
			subscription.GooglePayment.UserCancellationTime = &now
		case models.SubscriptionStateExpired:
			// 设置过期时间
			if subscription.GooglePayment.ExpiryTimeMillis == "" {
				subscription.GooglePayment.ExpiryTimeMillis = time.Now().Format(time.RFC3339)
			}
		}

		if err := s.db.WithContext(ctx).Save(subscription.GooglePayment).Error; err != nil {
			return fmt.Errorf("更新Google支付记录失败: %w", err)
		}
	}

	// 如果需要，可以更新订单状态
	if status == models.SubscriptionStateExpired || status == models.SubscriptionStateCancelled {
		if err := s.paymentService.UpdateOrderStatus(ctx, orderID, models.OrderStatusDelivered); err != nil {
			s.logger.Warn("更新订单状态失败", zap.Error(err))
		}
	}

	return nil
}

// CancelSubscription 取消订阅
func (s *subscriptionServiceImpl) CancelSubscription(ctx context.Context, orderID uint, reason string) error {
	// 获取订阅信息
	subscription, err := s.GetSubscription(ctx, orderID)
	if err != nil {
		return err
	}

	// 检查当前状态
	if subscription.CurrentStatus != models.SubscriptionStateActive {
		return fmt.Errorf("订阅状态不允许取消: %s", subscription.CurrentStatus)
	}

	// 更新订阅状态为已取消
	return s.UpdateSubscriptionStatus(ctx, orderID, models.SubscriptionStateCancelled, reason)
}

// RenewSubscription 续订订阅
func (s *subscriptionServiceImpl) RenewSubscription(ctx context.Context, orderID uint, newExpiryTime time.Time) error {
	// 获取订阅信息
	subscription, err := s.GetSubscription(ctx, orderID)
	if err != nil {
		return err
	}

	// 更新Google支付记录的到期时间
	if subscription.GooglePayment != nil {
		subscription.GooglePayment.ExpiryTimeMillis = newExpiryTime.Format(time.RFC3339)

		// 清除取消相关信息
		subscription.GooglePayment.CancelReason = nil
		subscription.GooglePayment.UserCancellationTime = nil

		if err := s.db.WithContext(ctx).Save(subscription.GooglePayment).Error; err != nil {
			return fmt.Errorf("更新订阅到期时间失败: %w", err)
		}
	}

	s.logger.Info("订阅续订成功",
		zap.Uint("order_id", orderID),
		zap.Time("new_expiry_time", newExpiryTime))

	return nil
}

// GetActiveSubscriptions 获取用户活跃订阅
func (s *subscriptionServiceImpl) GetActiveSubscriptions(ctx context.Context, userID uint) ([]*SubscriptionInfo, error) {
	activeStatus := models.SubscriptionStateActive
	return s.GetUserSubscriptions(ctx, userID, &activeStatus)
}

// GetExpiredSubscriptions 获取用户过期订阅
func (s *subscriptionServiceImpl) GetExpiredSubscriptions(ctx context.Context, userID uint) ([]*SubscriptionInfo, error) {
	expiredStatus := models.SubscriptionStateExpired
	return s.GetUserSubscriptions(ctx, userID, &expiredStatus)
}

// ValidateSubscription 验证订阅有效性
func (s *subscriptionServiceImpl) ValidateSubscription(ctx context.Context, orderID uint) (*SubscriptionValidationResult, error) {
	// 获取订阅信息
	subscription, err := s.GetSubscription(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// 如果存在Google支付记录，尝试从Google验证
	if subscription.GooglePayment != nil {
		// 调用Google API验证订阅状态
		googleSubscription, err := s.googleService.VerifySubscription(ctx,
			subscription.Order.ProductID,
			subscription.GooglePayment.PurchaseToken)

		if err != nil {
			s.logger.Warn("Google订阅验证失败", zap.Error(err))
			// 继续使用本地状态
		} else {
			// 根据Google响应更新状态
			newStatus := GetSubscriptionStatus(googleSubscription, time.Now())

			// 如果状态发生变化，更新本地记录
			if newStatus != subscription.CurrentStatus {
				s.UpdateSubscriptionStatus(ctx, orderID, newStatus, "Google validation")
				subscription.CurrentStatus = newStatus
			}

			return &SubscriptionValidationResult{
				IsValid:        newStatus == models.SubscriptionStateActive,
				CurrentStatus:  newStatus,
				ExpiryTime:     subscription.ExpiryTime,
				AutoRenewing:   googleSubscription.AutoRenewing,
				CancelReason:   &googleSubscription.CancelReason,
				ValidationTime: time.Now(),
				Message:        "Subscription validated with Google",
			}, nil
		}
	}

	// 返回本地验证结果
	isValid := subscription.CurrentStatus == models.SubscriptionStateActive
	message := "Subscription validated locally"
	if !isValid {
		message = fmt.Sprintf("Subscription is not active: %s", subscription.CurrentStatus)
	}

	return &SubscriptionValidationResult{
		IsValid:        isValid,
		CurrentStatus:  subscription.CurrentStatus,
		ExpiryTime:     subscription.ExpiryTime,
		AutoRenewing:   subscription.AutoRenewing,
		ValidationTime: time.Now(),
		Message:        message,
	}, nil
}

// GetSubscriptionStats 获取订阅统计
func (s *subscriptionServiceImpl) GetSubscriptionStats(ctx context.Context, userID uint) (*SubscriptionStats, error) {
	var stats SubscriptionStats

	// 总订阅数
	err := s.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("user_id = ? AND type = ?", userID, models.OrderTypeSubscription).
		Count(&stats.TotalSubscriptions).Error

	if err != nil {
		return nil, fmt.Errorf("统计总订阅数失败: %w", err)
	}

	// 活跃订阅数
	err = s.db.WithContext(ctx).
		Model(&models.Order{}).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		Where("orders.user_id = ? AND orders.type = ? AND orders.payment_status = ?",
			userID, models.OrderTypeSubscription, models.PaymentStatusCompleted).
		Where("google_payments.expiry_time_millis > ?", time.Now().Format(time.RFC3339)).
		Count(&stats.ActiveSubscriptions).Error

	if err != nil {
		return nil, fmt.Errorf("统计活跃订阅数失败: %w", err)
	}

	// 过期订阅数
	err = s.db.WithContext(ctx).
		Model(&models.Order{}).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		Where("orders.user_id = ? AND orders.type = ?", userID, models.OrderTypeSubscription).
		Where("google_payments.expiry_time_millis <= ?", time.Now().Format(time.RFC3339)).
		Count(&stats.ExpiredSubscriptions).Error

	if err != nil {
		return nil, fmt.Errorf("统计过期订阅数失败: %w", err)
	}

	// 已取消订阅数
	err = s.db.WithContext(ctx).
		Model(&models.Order{}).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		Where("orders.user_id = ? AND orders.type = ?", userID, models.OrderTypeSubscription).
		Where("google_payments.cancel_reason IS NOT NULL").
		Count(&stats.CancelledSubscriptions).Error

	if err != nil {
		return nil, fmt.Errorf("统计已取消订阅数失败: %w", err)
	}

	// 总消费金额
	var totalAmount int64
	err = s.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("user_id = ? AND type = ? AND payment_status = ?",
			userID, models.OrderTypeSubscription, models.PaymentStatusCompleted).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&totalAmount).Error

	if err != nil {
		return nil, fmt.Errorf("统计总消费金额失败: %w", err)
	}

	stats.TotalSpent = totalAmount

	return &stats, nil
}

// determineSubscriptionStatus 确定订阅状态
func (s *subscriptionServiceImpl) determineSubscriptionStatus(googlePayment *models.GooglePayment) models.SubscriptionState {
	if googlePayment == nil {
		return models.SubscriptionStatePending
	}

	// 如果有取消原因，说明已取消
	if googlePayment.CancelReason != nil {
		// 检查是否已过期
		if expiryTime, err := time.Parse(time.RFC3339, googlePayment.ExpiryTimeMillis); err == nil {
			if time.Now().After(expiryTime) {
				return models.SubscriptionStateExpired
			}
		}
		// 如果未过期，但已取消，返回已取消状态
		return models.SubscriptionStateCancelled
	}

	// 检查宽限期
	if googlePayment.GracePeriodExpiryTime != nil {
		if time.Now().Before(*googlePayment.GracePeriodExpiryTime) {
			return models.SubscriptionStateInGracePeriod
		}
	}

	// 检查到期时间
	if googlePayment.ExpiryTimeMillis != "" {
		if expiryTime, err := time.Parse(time.RFC3339, googlePayment.ExpiryTimeMillis); err == nil {
			if time.Now().After(expiryTime) {
				return models.SubscriptionStateExpired
			}
		}
	}

	// 检查自动续订状态
	if googlePayment.AutoRenewing != nil {
		if *googlePayment.AutoRenewing {
			return models.SubscriptionStateActive
		}
	}

	// 默认返回活跃状态
	return models.SubscriptionStateActive
}

// WebhookRetryStrategy 结构定义
type WebhookRetryStrategy struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	MaxRetries    int
}

// DefaultWebhookRetryStrategy 默认重试策略
var DefaultWebhookRetryStrategy = WebhookRetryStrategy{
	InitialDelay:  1 * time.Minute,
	MaxDelay:      1 * time.Hour,
	BackoffFactor: 2.0,
	MaxRetries:    3,
}
