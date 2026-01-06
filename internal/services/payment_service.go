package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// PaymentService 支付服务接口
type PaymentService interface {
	// 订单相关
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*models.Order, error)
	GetOrder(ctx context.Context, orderID uint) (*models.Order, error)
	GetOrderByOrderNo(ctx context.Context, orderNo string) (*models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uint, status models.OrderStatus) error
	CancelOrder(ctx context.Context, orderID uint, reason string) error

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

// paymentServiceImpl 支付服务实现
type paymentServiceImpl struct {
	db            *gorm.DB
	config        *config.Config
	logger        *zap.Logger
	googleService *GooglePlayService
	alipayService *AlipayService
	appleService  *AppleService
}

// NewPaymentService 创建支付服务
func NewPaymentService(db *gorm.DB, cfg *config.Config, logger *zap.Logger, googleService *GooglePlayService, alipayService *AlipayService, appleService *AppleService) PaymentService {
	return &paymentServiceImpl{
		db:            db,
		config:        cfg,
		logger:        logger,
		googleService: googleService,
		alipayService: alipayService,
		appleService:  appleService,
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
