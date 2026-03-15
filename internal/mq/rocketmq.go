package mq

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"go.uber.org/zap"

	"pay-gateway/internal/config"
)

// Client 封装 RocketMQ Producer 和 SimpleConsumer
type Client struct {
	producer golang.Producer
	config   *config.RocketMQConfig
	logger   *zap.Logger
}

// NewClient 创建 RocketMQ 客户端（Producer）
func NewClient(cfg *config.RocketMQConfig, logger *zap.Logger) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	producer, err := golang.NewProducer(
		&golang.Config{
			Endpoint: cfg.Endpoint,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		golang.WithTopics(cfg.OrderDelayTopic),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 RocketMQ Producer 失败: %w", err)
	}

	if err := producer.Start(); err != nil {
		return nil, fmt.Errorf("启动 RocketMQ Producer 失败: %w", err)
	}

	logger.Info("RocketMQ Producer 启动成功", zap.String("endpoint", cfg.Endpoint))

	return &Client{
		producer: producer,
		config:   cfg,
		logger:   logger,
	}, nil
}

// SendDelayMessage 发送延迟消息
// topic: 消息主题
// body: 消息体
// delay: 延迟时间
// keys: 消息键（用于查询和去重）
// tag: 消息标签（用于过滤）
func (c *Client) SendDelayMessage(ctx context.Context, topic string, body []byte, delay time.Duration, keys []string, tag string) error {
	msg := &golang.Message{
		Topic: topic,
		Body:  body,
	}
	msg.SetDelayTimestamp(time.Now().Add(delay))
	if len(keys) > 0 {
		msg.SetKeys(keys...)
	}
	if tag != "" {
		msg.SetTag(tag)
	}

	resp, err := c.producer.Send(ctx, msg)
	if err != nil {
		c.logger.Error("发送延迟消息失败",
			zap.String("topic", topic),
			zap.Duration("delay", delay),
			zap.Error(err))
		return fmt.Errorf("发送延迟消息失败: %w", err)
	}

	for _, r := range resp {
		c.logger.Info("延迟消息发送成功",
			zap.String("topic", topic),
			zap.String("message_id", r.MessageID),
			zap.Duration("delay", delay))
	}

	return nil
}

// NewSimpleConsumer 创建 SimpleConsumer
func NewSimpleConsumer(cfg *config.RocketMQConfig, logger *zap.Logger, filterExpressions map[string]*golang.FilterExpression) (golang.SimpleConsumer, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	consumer, err := golang.NewSimpleConsumer(
		&golang.Config{
			Endpoint:      cfg.Endpoint,
			ConsumerGroup: cfg.ConsumerGroup,
			Credentials: &credentials.SessionCredentials{
				AccessKey:    cfg.AccessKey,
				AccessSecret: cfg.SecretKey,
			},
		},
		golang.WithSimpleAwaitDuration(5*time.Second),
		golang.WithSimpleSubscriptionExpressions(filterExpressions),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 RocketMQ SimpleConsumer 失败: %w", err)
	}

	if err := consumer.Start(); err != nil {
		return nil, fmt.Errorf("启动 RocketMQ SimpleConsumer 失败: %w", err)
	}

	logger.Info("RocketMQ SimpleConsumer 启动成功",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("group", cfg.ConsumerGroup))

	return consumer, nil
}

// Close 关闭 Producer
func (c *Client) Close() error {
	if c == nil || c.producer == nil {
		return nil
	}
	c.logger.Info("正在关闭 RocketMQ Producer...")
	return c.producer.GracefulStop()
}
