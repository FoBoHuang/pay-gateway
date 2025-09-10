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

	"google-play-billing/internal/config"
	"google-play-billing/internal/models"
)

type GooglePlayService struct {
	config      *config.Config
	logger      *zap.Logger
	service     *androidpublisher.Service
	packageName string
}

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

// VerifyPurchase verifies a one-time purchase
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

// VerifySubscription verifies a subscription
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

// AcknowledgePurchase acknowledges a purchase
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

// AcknowledgeSubscription acknowledges a subscription
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

// ConsumePurchase consumes a purchase (for consumable products)
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

// VerifyWebhookSignature verifies the webhook signature from Google Play
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

// GetSubscriptionStatus determines the current status of a subscription
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

// ParseWebhookPayload parses the webhook payload from Google Play
func ParseWebhookPayload(payload []byte) (*models.WebhookEvent, error) {
	var event models.WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	return &event, nil
}
