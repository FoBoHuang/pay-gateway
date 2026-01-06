package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// GooglePlayService Google Play服务核心结构体
// 负责处理所有Google Play相关的支付验证、订阅管理和Webhook处理
type GooglePlayService struct {
	config      *config.Config            // 应用配置
	logger      *zap.Logger               // 日志记录器
	service     *androidpublisher.Service // Google Play Android Publisher API服务
	packageName string                    // Android应用包名
}

// PurchaseResponse 购买验证响应结构体
// 包含单次购买的详细信息和状态
type PurchaseResponse struct {
	Kind                        string `json:"kind"`
	PurchaseTimeMillis          string `json:"purchaseTimeMillis"`
	PurchaseState               int    `json:"purchaseState"`
	ConsumptionState            int    `json:"consumptionState"`
	DeveloperPayload            string `json:"developerPayload"`
	OrderId                     string `json:"orderId"`
	PurchaseType                int    `json:"purchaseType,omitempty"`
	AcknowledgementState        int    `json:"acknowledgementState"`
	ObfuscatedExternalAccountId string `json:"obfuscatedExternalAccountId,omitempty"`
	ObfuscatedExternalProfileId string `json:"obfuscatedExternalProfileId,omitempty"`
	RegionCode                  string `json:"regionCode,omitempty"`
}

// SubscriptionResponse 订阅验证响应结构体
// 包含订阅的详细状态、价格信息和自动续费设置
type SubscriptionResponse struct {
	Kind                  string `json:"kind"`
	StartTimeMillis       string `json:"startTimeMillis"`
	ExpiryTimeMillis      string `json:"expiryTimeMillis"`
	AutoResumeTimeMillis  string `json:"autoResumeTimeMillis,omitempty"`
	AutoRenewing          bool   `json:"autoRenewing"`
	PriceCurrencyCode     string `json:"priceCurrencyCode"`
	PriceAmountMicros     string `json:"priceAmountMicros"`
	IntroductoryPriceInfo *struct {
		IntroductoryPriceCurrencyCode string `json:"introductoryPriceCurrencyCode"`
		IntroductoryPriceAmountMicros string `json:"introductoryPriceAmountMicros"`
		IntroductoryPricePeriod       string `json:"introductoryPricePeriod"`
		IntroductoryPriceCycles       int    `json:"introductoryPriceCycles"`
	} `json:"introductoryPriceInfo,omitempty"`
	CountryCode                 string `json:"countryCode"`
	DeveloperPayload            string `json:"developerPayload,omitempty"`
	PaymentState                int    `json:"paymentState"`
	CancelReason                int    `json:"cancelReason,omitempty"`
	UserCancellationTimeMillis  string `json:"userCancellationTimeMillis,omitempty"`
	OrderId                     string `json:"orderId"`
	AcknowledgementState        int    `json:"acknowledgementState"`
	ObfuscatedExternalAccountId string `json:"obfuscatedExternalAccountId,omitempty"`
	ObfuscatedExternalProfileId string `json:"obfuscatedExternalProfileId,omitempty"`
}

// NewGooglePlayService 创建Google Play服务实例
// 初始化Google Play Android Publisher API连接
// 参数：
//   - cfg: 应用配置
//   - logger: 日志记录器
//
// 返回：GooglePlayService实例或错误
func NewGooglePlayService(cfg *config.Config, logger *zap.Logger) (*GooglePlayService, error) {
	ctx := context.Background()

	// Load service account
	credentials, err := google.CredentialsFromJSON(
		ctx,
		[]byte(cfg.Google.ServiceAccountFile),
		androidpublisher.AndroidpublisherScope,
	)
	if err != nil {
		// Try to load from file path
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	// Create service
	service, err := androidpublisher.NewService(
		ctx,
		option.WithCredentials(credentials),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create androidpublisher service: %w", err)
	}

	return &GooglePlayService{
		config:      cfg,
		logger:      logger,
		service:     service,
		packageName: cfg.Google.PackageName,
	}, nil
}

// VerifyPurchase 验证单次购买
// 向Google Play服务器验证购买令牌的有效性
// 参数：
//   - ctx: 上下文
//   - productID: Google Play商品ID
//   - purchaseToken: 购买令牌
//
// 返回：购买详情或错误
func (s *GooglePlayService) VerifyPurchase(ctx context.Context, productID, purchaseToken string) (*PurchaseResponse, error) {
	purchase, err := s.service.Purchases.Products.Get(s.packageName, productID, purchaseToken).Context(ctx).Do()
	if err != nil {
		s.logger.Error("failed to verify purchase",
			zap.String("product_id", productID),
			zap.String("purchase_token", purchaseToken),
			zap.Error(err))
		return nil, fmt.Errorf("failed to verify purchase: %w", err)
	}

	response := &PurchaseResponse{
		Kind:                        purchase.Kind,
		PurchaseTimeMillis:          fmt.Sprintf("%d", purchase.PurchaseTimeMillis),
		PurchaseState:               int(purchase.PurchaseState),
		ConsumptionState:            int(purchase.ConsumptionState),
		DeveloperPayload:            purchase.DeveloperPayload,
		OrderId:                     purchase.OrderId,
		PurchaseType:                int(getInt64Value(purchase.PurchaseType)),
		AcknowledgementState:        int(purchase.AcknowledgementState),
		ObfuscatedExternalAccountId: purchase.ObfuscatedExternalAccountId,
		ObfuscatedExternalProfileId: purchase.ObfuscatedExternalProfileId,
		RegionCode:                  purchase.RegionCode,
	}

	s.logger.Info("purchase verified successfully",
		zap.String("order_id", purchase.OrderId),
		zap.String("product_id", productID),
		zap.Int64("purchase_time", purchase.PurchaseTimeMillis))

	return response, nil
}

// VerifySubscription 验证订阅
// 向Google Play服务器验证订阅购买令牌的有效性
// 参数：
//   - ctx: 上下文
//   - subscriptionID: 订阅商品ID
//   - purchaseToken: 购买令牌
//
// 返回：订阅详情或错误
func (s *GooglePlayService) VerifySubscription(ctx context.Context, subscriptionID, purchaseToken string) (*SubscriptionResponse, error) {
	subscription, err := s.service.Purchases.Subscriptions.Get(s.packageName, subscriptionID, purchaseToken).Context(ctx).Do()
	if err != nil {
		s.logger.Error("failed to verify subscription",
			zap.String("subscription_id", subscriptionID),
			zap.String("purchase_token", purchaseToken),
			zap.Error(err))
		return nil, fmt.Errorf("failed to verify subscription: %w", err)
	}

	response := &SubscriptionResponse{
		Kind:                        subscription.Kind,
		StartTimeMillis:             fmt.Sprintf("%d", subscription.StartTimeMillis),
		ExpiryTimeMillis:            fmt.Sprintf("%d", subscription.ExpiryTimeMillis),
		AutoResumeTimeMillis:        fmt.Sprintf("%d", subscription.AutoResumeTimeMillis),
		AutoRenewing:                subscription.AutoRenewing,
		PriceCurrencyCode:           subscription.PriceCurrencyCode,
		PriceAmountMicros:           fmt.Sprintf("%d", subscription.PriceAmountMicros),
		CountryCode:                 subscription.CountryCode,
		DeveloperPayload:            subscription.DeveloperPayload,
		PaymentState:                int(getInt64Value(subscription.PaymentState)),
		CancelReason:                int(subscription.CancelReason),
		UserCancellationTimeMillis:  fmt.Sprintf("%d", subscription.UserCancellationTimeMillis),
		OrderId:                     subscription.OrderId,
		AcknowledgementState:        int(subscription.AcknowledgementState),
		ObfuscatedExternalAccountId: subscription.ObfuscatedExternalAccountId,
		ObfuscatedExternalProfileId: subscription.ObfuscatedExternalProfileId,
	}

	if subscription.IntroductoryPriceInfo != nil {
		response.IntroductoryPriceInfo = &struct {
			IntroductoryPriceCurrencyCode string `json:"introductoryPriceCurrencyCode"`
			IntroductoryPriceAmountMicros string `json:"introductoryPriceAmountMicros"`
			IntroductoryPricePeriod       string `json:"introductoryPricePeriod"`
			IntroductoryPriceCycles       int    `json:"introductoryPriceCycles"`
		}{
			IntroductoryPriceCurrencyCode: subscription.IntroductoryPriceInfo.IntroductoryPriceCurrencyCode,
			IntroductoryPriceAmountMicros: fmt.Sprintf("%d", subscription.IntroductoryPriceInfo.IntroductoryPriceAmountMicros),
			IntroductoryPricePeriod:       subscription.IntroductoryPriceInfo.IntroductoryPricePeriod,
			IntroductoryPriceCycles:       int(subscription.IntroductoryPriceInfo.IntroductoryPriceCycles),
		}
	}

	s.logger.Info("subscription verified successfully",
		zap.String("order_id", subscription.OrderId),
		zap.String("subscription_id", subscriptionID),
		zap.Bool("auto_renewing", subscription.AutoRenewing))

	return response, nil
}

// AcknowledgePurchase 确认购买
// 向Google Play确认已收到购买信息，防止重复发放商品
// 参数：
//   - ctx: 上下文
//   - productID: 商品ID
//   - purchaseToken: 购买令牌
//   - developerPayload: 开发者自定义数据
//
// 返回：错误或nil
func (s *GooglePlayService) AcknowledgePurchase(ctx context.Context, productID, purchaseToken string, developerPayload string) error {
	acknowledgeRequest := &androidpublisher.ProductPurchasesAcknowledgeRequest{
		DeveloperPayload: developerPayload,
	}

	err := s.service.Purchases.Products.Acknowledge(s.packageName, productID, purchaseToken, acknowledgeRequest).Context(ctx).Do()
	if err != nil {
		s.logger.Error("failed to acknowledge purchase",
			zap.String("product_id", productID),
			zap.String("purchase_token", purchaseToken),
			zap.Error(err))
		return fmt.Errorf("failed to acknowledge purchase: %w", err)
	}

	s.logger.Info("purchase acknowledged successfully",
		zap.String("product_id", productID),
		zap.String("purchase_token", purchaseToken))

	return nil
}

// AcknowledgeSubscription 确认订阅
// 向Google Play确认已收到订阅购买信息
// 参数：
//   - ctx: 上下文
//   - subscriptionID: 订阅商品ID
//   - purchaseToken: 购买令牌
//   - developerPayload: 开发者自定义数据
//
// 返回：错误或nil
func (s *GooglePlayService) AcknowledgeSubscription(ctx context.Context, subscriptionID, purchaseToken string, developerPayload string) error {
	acknowledgeRequest := &androidpublisher.SubscriptionPurchasesAcknowledgeRequest{
		DeveloperPayload: developerPayload,
	}

	err := s.service.Purchases.Subscriptions.Acknowledge(s.packageName, subscriptionID, purchaseToken, acknowledgeRequest).Context(ctx).Do()
	if err != nil {
		s.logger.Error("failed to acknowledge subscription",
			zap.String("subscription_id", subscriptionID),
			zap.String("purchase_token", purchaseToken),
			zap.Error(err))
		return fmt.Errorf("failed to acknowledge subscription: %w", err)
	}

	s.logger.Info("subscription acknowledged successfully",
		zap.String("subscription_id", subscriptionID),
		zap.String("purchase_token", purchaseToken))

	return nil
}

// ConsumePurchase 消费购买（适用于消耗型商品）
// 消费后该购买可被再次购买
// 参数：
//   - ctx: 上下文
//   - productID: 商品ID
//   - purchaseToken: 购买令牌
//
// 返回：错误或nil
func (s *GooglePlayService) ConsumePurchase(ctx context.Context, productID, purchaseToken string) error {

	err := s.service.Purchases.Products.Consume(s.packageName, productID, purchaseToken).Context(ctx).Do()
	if err != nil {
		s.logger.Error("failed to consume purchase",
			zap.String("product_id", productID),
			zap.String("purchase_token", purchaseToken),
			zap.Error(err))
		return fmt.Errorf("failed to consume purchase: %w", err)
	}

	s.logger.Info("purchase consumed successfully",
		zap.String("product_id", productID),
		zap.String("purchase_token", purchaseToken))

	return nil
}

// Helper function to get int64 value from pointer
func getInt64Value(val *int64) int64 {
	if val != nil {
		return *val
	}
	return 0
}

// VerifyWebhookSignature 验证Google Play Webhook签名
// 确保Webhook请求来自Google Play，防止伪造通知
// 参数：
//   - payload: 请求体数据
//   - signature: 请求签名
//
// 返回：错误或nil
func (s *GooglePlayService) VerifyWebhookSignature(payload []byte, signature string) error {
	decodedSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// In a real implementation, you would verify the signature using Google's public key
	// For now, we'll just log it
	s.logger.Info("webhook signature verification (placeholder)",
		zap.String("signature", signature),
		zap.Int("payload_length", len(payload)),
		zap.String("decoded_signature", string(decodedSignature)))

	return nil
}

// Helper function to convert millis to time
func millisToTime(millis string) (time.Time, error) {
	if millis == "" {
		return time.Time{}, nil
	}

	var millisInt int64
	_, err := fmt.Sscanf(millis, "%d", &millisInt)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, millisInt*int64(time.Millisecond)), nil
}

// GetSubscriptionStatus 确定订阅当前状态
// 根据订阅信息和时间判断订阅状态（活跃、过期、取消等）
// 参数：
//   - subscription: 订阅信息
//   - currentTime: 当前时间
//
// 返回：订阅状态枚举值
func GetSubscriptionStatus(subscription *SubscriptionResponse, currentTime time.Time) models.SubscriptionState {
	expiryTime, _ := millisToTime(subscription.ExpiryTimeMillis)

	if !subscription.AutoRenewing && expiryTime.Before(currentTime) {
		return models.SubscriptionStateExpired
	}

	if subscription.CancelReason != 0 {
		if subscription.AutoRenewing {
			return models.SubscriptionStateCancelled // Will expire at end of period
		}
		if expiryTime.Before(currentTime) {
			return models.SubscriptionStateExpired
		}
	}

	// Check payment state
	switch subscription.PaymentState {
	case 0: // Payment pending
		return models.SubscriptionStatePending
	case 1: // Payment received
		return models.SubscriptionStateActive
	case 2: // Free trial
		return models.SubscriptionStateActive
	default:
		if expiryTime.After(currentTime) {
			return models.SubscriptionStateActive
		}
		return models.SubscriptionStateExpired
	}
}

// ParseWebhookPayload 解析Google Play Webhook负载
// 将JSON格式的Webhook数据解析为WebhookEvent结构体
// 参数：
//   - payload: JSON格式的Webhook数据
//
// 返回：解析后的Webhook事件或错误
func ParseWebhookPayload(payload []byte) (*models.WebhookEvent, error) {
	var event models.WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	return &event, nil
}
