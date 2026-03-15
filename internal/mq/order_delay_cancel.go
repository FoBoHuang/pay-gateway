package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apache/rocketmq-clients/golang/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

const (
	TagOrderTimeoutCancel = "ORDER_TIMEOUT_CANCEL"
)

// OrderCancelMessage 订单超时取消消息体
type OrderCancelMessage struct {
	OrderNo   string `json:"order_no"`
	OrderID   uint   `json:"order_id"`
	CreatedAt int64  `json:"created_at"`
}

// OrderDelayCancelProducer 订单延迟取消消息生产者
type OrderDelayCancelProducer struct {
	client *Client
	config *config.RocketMQConfig
	logger *zap.Logger
}

// NewOrderDelayCancelProducer 创建订单延迟取消生产者
func NewOrderDelayCancelProducer(client *Client, cfg *config.RocketMQConfig, logger *zap.Logger) *OrderDelayCancelProducer {
	return &OrderDelayCancelProducer{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// SendOrderTimeoutMessage 发送订单超时取消延迟消息
// 在订单创建成功后调用，延迟 OrderTimeout 时间后投递
func (p *OrderDelayCancelProducer) SendOrderTimeoutMessage(ctx context.Context, orderNo string, orderID uint) error {
	if p == nil || p.client == nil {
		return nil
	}

	msg := OrderCancelMessage{
		OrderNo:   orderNo,
		OrderID:   orderID,
		CreatedAt: time.Now().Unix(),
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化订单取消消息失败: %w", err)
	}

	err = p.client.SendDelayMessage(
		ctx,
		p.config.OrderDelayTopic,
		body,
		p.config.OrderTimeout,
		[]string{orderNo},
		TagOrderTimeoutCancel,
	)
	if err != nil {
		p.logger.Error("发送订单超时取消延迟消息失败",
			zap.String("order_no", orderNo),
			zap.Uint("order_id", orderID),
			zap.Error(err))
		return err
	}

	p.logger.Info("订单超时取消延迟消息已发送",
		zap.String("order_no", orderNo),
		zap.Uint("order_id", orderID),
		zap.Duration("delay", p.config.OrderTimeout))

	return nil
}

// OrderDelayCancelConsumer 订单延迟取消消息消费者
type OrderDelayCancelConsumer struct {
	consumer golang.SimpleConsumer
	db       *gorm.DB
	config   *config.RocketMQConfig
	logger   *zap.Logger
	stopCh   chan struct{}
}

// NewOrderDelayCancelConsumer 创建订单延迟取消消费者
func NewOrderDelayCancelConsumer(cfg *config.RocketMQConfig, db *gorm.DB, logger *zap.Logger) (*OrderDelayCancelConsumer, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	filterExpr := map[string]*golang.FilterExpression{
		cfg.OrderDelayTopic: golang.NewFilterExpression(TagOrderTimeoutCancel),
	}

	consumer, err := NewSimpleConsumer(cfg, logger, filterExpr)
	if err != nil {
		return nil, err
	}

	return &OrderDelayCancelConsumer{
		consumer: consumer,
		db:       db,
		config:   cfg,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start 启动消费者循环
func (c *OrderDelayCancelConsumer) Start() {
	if c == nil || c.consumer == nil {
		return
	}
	c.logger.Info("订单超时取消消费者已启动")

	go func() {
		for {
			select {
			case <-c.stopCh:
				c.logger.Info("订单超时取消消费者收到停止信号")
				return
			default:
				c.receiveAndProcess()
			}
		}
	}()
}

// receiveAndProcess 接收并处理消息
func (c *OrderDelayCancelConsumer) receiveAndProcess() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mvs, err := c.consumer.Receive(ctx, 16, 20*time.Second)
	if err != nil {
		// Receive 超时或暂无消息是正常情况，不打错误日志
		return
	}

	for _, mv := range mvs {
		if err := c.handleMessage(mv); err != nil {
			c.logger.Error("处理订单超时取消消息失败",
				zap.String("message_id", mv.GetMessageId()),
				zap.Error(err))
			continue
		}
		if err := c.consumer.Ack(context.Background(), mv); err != nil {
			c.logger.Error("ACK 消息失败",
				zap.String("message_id", mv.GetMessageId()),
				zap.Error(err))
		}
	}
}

// handleMessage 处理单条订单超时取消消息
func (c *OrderDelayCancelConsumer) handleMessage(mv *golang.MessageView) error {
	var msg OrderCancelMessage
	if err := json.Unmarshal(mv.GetBody(), &msg); err != nil {
		c.logger.Error("反序列化订单取消消息失败",
			zap.String("message_id", mv.GetMessageId()),
			zap.Error(err))
		// 消息格式错误，ACK 掉避免重复投递
		return nil
	}

	c.logger.Info("收到订单超时取消消息",
		zap.String("order_no", msg.OrderNo),
		zap.Uint("order_id", msg.OrderID))

	// 查询订单当前状态
	var order models.Order
	if err := c.db.Where("id = ? AND order_no = ?", msg.OrderID, msg.OrderNo).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.logger.Warn("订单不存在，跳过取消",
				zap.String("order_no", msg.OrderNo),
				zap.Uint("order_id", msg.OrderID))
			return nil
		}
		return fmt.Errorf("查询订单失败: %w", err)
	}

	// 仅取消状态为 CREATED 且支付状态为 PENDING 的订单
	if order.Status != models.OrderStatusCreated || order.PaymentStatus != models.PaymentStatusPending {
		c.logger.Info("订单状态不满足取消条件，跳过",
			zap.String("order_no", msg.OrderNo),
			zap.String("status", string(order.Status)),
			zap.String("payment_status", string(order.PaymentStatus)))
		return nil
	}

	// 执行取消
	now := time.Now()
	result := c.db.Model(&models.Order{}).
		Where("id = ? AND status = ? AND payment_status = ?",
			order.ID, models.OrderStatusCreated, models.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":        models.OrderStatusCancelled,
			"refund_reason": "订单超时自动取消",
			"refund_at":     now,
		})

	if result.Error != nil {
		return fmt.Errorf("取消订单失败: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		c.logger.Info("订单超时自动取消成功",
			zap.String("order_no", msg.OrderNo),
			zap.Uint("order_id", msg.OrderID))
	} else {
		c.logger.Info("订单已被其他流程处理，跳过取消",
			zap.String("order_no", msg.OrderNo))
	}

	return nil
}

// Stop 停止消费者
func (c *OrderDelayCancelConsumer) Stop() error {
	if c == nil || c.consumer == nil {
		return nil
	}
	close(c.stopCh)
	c.logger.Info("正在关闭订单超时取消消费者...")
	return c.consumer.GracefulStop()
}
