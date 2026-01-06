package services

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/awa/go-iap/appstore"
	"github.com/awa/go-iap/appstore/api"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// AppleService Apple服务核心结构体
// 负责处理所有Apple Store相关的支付验证、订阅管理和Webhook处理
type AppleService struct {
	config      *config.Config
	logger      *zap.Logger
	db          *gorm.DB
	client      *appstore.Client
	storeClient *api.StoreClient
	bundleID    string
}

// ApplePurchaseResponse 购买验证响应结构体
type ApplePurchaseResponse struct {
	TransactionID         string     `json:"transaction_id"`
	OriginalTransactionID string     `json:"original_transaction_id"`
	ProductID             string     `json:"product_id"`
	BundleID              string     `json:"bundle_id"`
	PurchaseDate          time.Time  `json:"purchase_date"`
	OriginalPurchaseDate  time.Time  `json:"original_purchase_date"`
	Quantity              int        `json:"quantity"`
	IsTrialPeriod         bool       `json:"is_trial_period"`
	IsInIntroOfferPeriod  bool       `json:"is_in_intro_offer_period"`
	ExpiresDate           *time.Time `json:"expires_date,omitempty"`
	CancellationDate      *time.Time `json:"cancellation_date,omitempty"`
	WebOrderLineItemID    string     `json:"web_order_line_item_id,omitempty"`
	SubscriptionGroupID   string     `json:"subscription_group_id,omitempty"`
	ProductType           string     `json:"product_type"`
	InAppOwnershipType    string     `json:"in_app_ownership_type"`
	Environment           string     `json:"environment"`
	Status                string     `json:"status"`
}

// AppleSubscriptionResponse 订阅验证响应结构体
type AppleSubscriptionResponse struct {
	ApplePurchaseResponse
	AutoRenewStatus    bool   `json:"auto_renew_status"`
	AutoRenewProductID string `json:"auto_renew_product_id,omitempty"`
	GracePeriodStatus  string `json:"grace_period_status,omitempty"`
	ExpirationIntent   string `json:"expiration_intent,omitempty"`
}

// NewAppleService 创建Apple服务实例
// 初始化Apple Store API连接
// 参数：
//   - cfg: 应用配置
//   - logger: 日志记录器
//   - db: 数据库连接
//
// 返回：AppleService实例或错误
func NewAppleService(cfg *config.Config, logger *zap.Logger, db *gorm.DB) (*AppleService, error) {
	// 获取私钥内容
	privateKey := cfg.Apple.PrivateKey
	if privateKey == "" && cfg.Apple.PrivateKeyPath != "" {
		// 从文件读取私钥
		keyContent, err := ioutil.ReadFile(cfg.Apple.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read Apple private key file: %w", err)
		}
		privateKey = string(keyContent)
	}

	if privateKey == "" {
		return nil, fmt.Errorf("Apple private key is required")
	}

	// 验证私钥格式
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return nil, fmt.Errorf("invalid Apple private key format")
	}

	_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Apple private key: %w", err)
	}

	// 创建App Store客户端
	client := appstore.New()

	// 创建App Store Server API客户端
	storeConfig := &api.StoreConfig{
		KeyContent: []byte(privateKey),
		KeyID:      cfg.Apple.KeyID,
		BundleID:   cfg.Apple.BundleID,
		Issuer:     cfg.Apple.IssuerID,
		Sandbox:    cfg.Apple.Sandbox,
	}

	storeClient := api.NewStoreClient(storeConfig)

	return &AppleService{
		config:      cfg,
		logger:      logger,
		db:          db,
		client:      client,
		storeClient: storeClient,
		bundleID:    cfg.Apple.BundleID,
	}, nil
}

// VerifyPurchase 验证Apple购买
// 参数：
//   - ctx: 上下文
//   - receiptData: Base64编码的收据数据
//   - orderID: 订单ID
//
// 返回：购买响应或错误
func (s *AppleService) VerifyPurchase(ctx context.Context, receiptData string, orderID uint) (*ApplePurchaseResponse, error) {
	// 验证收据
	req := appstore.IAPRequest{
		ReceiptData: receiptData,
	}

	resp := &appstore.IAPResponse{}
	err := s.client.Verify(ctx, req, resp)
	if err != nil {
		s.logger.Error("failed to verify Apple receipt",
			zap.Error(err),
			zap.Uint("order_id", orderID),
		)
		return nil, fmt.Errorf("failed to verify Apple receipt: %w", err)
	}

	// 检查响应状态
	if resp.Status != 0 {
		s.logger.Error("Apple receipt verification failed",
			zap.Int("status", resp.Status),
			zap.Uint("order_id", orderID),
		)
		return nil, fmt.Errorf("Apple receipt verification failed with status: %d", resp.Status)
	}

	// 解析最新的交易信息
	if len(resp.Receipt.InApp) == 0 {
		return nil, fmt.Errorf("no in-app purchases found in receipt")
	}

	// 获取最新的交易
	latestReceipt := resp.Receipt.InApp[0]
	if len(resp.Receipt.InApp) > 1 {
		// 如果有多个交易，找到最新的一个
		for _, receipt := range resp.Receipt.InApp {
			if receipt.PurchaseDateMS > latestReceipt.PurchaseDateMS {
				latestReceipt = receipt
			}
		}
	}

	// 解析时间戳
	purchaseDateMS := int64(0)
	fmt.Sscanf(latestReceipt.PurchaseDateMS, "%d", &purchaseDateMS)
	purchaseDate := time.Unix(purchaseDateMS/1000, 0)

	originalPurchaseDateMS := int64(0)
	fmt.Sscanf(latestReceipt.OriginalPurchaseDateMS, "%d", &originalPurchaseDateMS)
	originalPurchaseDate := time.Unix(originalPurchaseDateMS/1000, 0)

	// 转换数量
	quantity := 1
	if latestReceipt.Quantity != "" {
		fmt.Sscanf(latestReceipt.Quantity, "%d", &quantity)
	}

	response := &ApplePurchaseResponse{
		TransactionID:         latestReceipt.TransactionID,
		OriginalTransactionID: string(latestReceipt.OriginalTransactionID),
		ProductID:             latestReceipt.ProductID,
		BundleID:              resp.Receipt.BundleID,
		PurchaseDate:          purchaseDate,
		OriginalPurchaseDate:  originalPurchaseDate,
		Quantity:              quantity,
		IsTrialPeriod:         latestReceipt.IsTrialPeriod == "true",
		IsInIntroOfferPeriod:  latestReceipt.IsInIntroOfferPeriod == "true",
		WebOrderLineItemID:    latestReceipt.WebOrderLineItemID,
		ProductType:           latestReceipt.ProductID, // 这里需要根据产品ID判断类型
		InAppOwnershipType:    latestReceipt.InAppOwnershipType,
		Environment:           s.getEnvironment(),
		Status:                "VERIFIED",
	}

	// 设置到期时间（如果是订阅）
	if latestReceipt.ExpiresDateMS != "" {
		expiresDateMS := int64(0)
		fmt.Sscanf(latestReceipt.ExpiresDateMS, "%d", &expiresDateMS)
		if expiresDateMS > 0 {
			expiresDate := time.Unix(expiresDateMS/1000, 0)
			response.ExpiresDate = &expiresDate
		}
	}

	// 设置取消时间（如果有）
	if latestReceipt.CancellationDateMS != "" {
		cancellationDateMS := int64(0)
		fmt.Sscanf(latestReceipt.CancellationDateMS, "%d", &cancellationDateMS)
		if cancellationDateMS > 0 {
			cancellationDate := time.Unix(cancellationDateMS/1000, 0)
			response.CancellationDate = &cancellationDate
		}
	}

	return response, nil
}

// VerifyTransaction 使用App Store Server API验证交易
// 这是推荐的验证方式，比收据验证更准确
// 参数：
//   - ctx: 上下文
//   - transactionID: 交易ID
//
// 返回：交易信息或错误
func (s *AppleService) VerifyTransaction(ctx context.Context, transactionID string) (*ApplePurchaseResponse, error) {
	// 获取交易信息
	response, err := s.storeClient.GetTransactionInfo(ctx, transactionID)
	if err != nil {
		s.logger.Error("failed to get Apple transaction info",
			zap.Error(err),
			zap.String("transaction_id", transactionID),
		)
		return nil, fmt.Errorf("failed to get Apple transaction info: %w", err)
	}

	// 解析签名的交易信息
	transaction, err := s.storeClient.ParseSignedTransaction(response.SignedTransactionInfo)
	if err != nil {
		s.logger.Error("failed to parse signed transaction",
			zap.Error(err),
			zap.String("transaction_id", transactionID),
		)
		return nil, fmt.Errorf("failed to parse signed transaction: %w", err)
	}

	purchaseDate := time.Unix(transaction.PurchaseDate/1000, 0)
	originalPurchaseDate := time.Unix(transaction.OriginalPurchaseDate/1000, 0)

	result := &ApplePurchaseResponse{
		TransactionID:         transaction.TransactionID,
		OriginalTransactionID: transaction.OriginalTransactionId,
		ProductID:             transaction.ProductID,
		BundleID:              transaction.BundleID,
		PurchaseDate:          purchaseDate,
		OriginalPurchaseDate:  originalPurchaseDate,
		Quantity:              int(transaction.Quantity),
		IsTrialPeriod:         transaction.Type == "Auto-Renewable Subscription" && transaction.OfferType == 0,
		IsInIntroOfferPeriod:  transaction.OfferType != 0,
		WebOrderLineItemID:    transaction.WebOrderLineItemId,
		SubscriptionGroupID:   "", // 需要从其他地方获取
		ProductType:           transaction.ProductID,
		InAppOwnershipType:    transaction.InAppOwnershipType,
		Environment:           s.getEnvironment(),
		Status:                "VERIFIED",
	}

	// 设置到期时间（如果是订阅）
	if transaction.ExpiresDate > 0 {
		expiresDate := time.Unix(transaction.ExpiresDate/1000, 0)
		result.ExpiresDate = &expiresDate
	}

	// 设置取消时间（如果有）
	if transaction.RevocationDate > 0 {
		cancellationDate := time.Unix(transaction.RevocationDate/1000, 0)
		result.CancellationDate = &cancellationDate
	}

	return result, nil
}

// GetTransactionHistory 获取交易历史
// 参数：
//   - ctx: 上下文
//   - originalTransactionID: 原始交易ID
//
// 返回：交易历史列表或错误
func (s *AppleService) GetTransactionHistory(ctx context.Context, originalTransactionID string) ([]*ApplePurchaseResponse, error) {
	responses, err := s.storeClient.GetTransactionHistory(ctx, originalTransactionID, nil)
	if err != nil {
		s.logger.Error("failed to get Apple transaction history",
			zap.Error(err),
			zap.String("original_transaction_id", originalTransactionID),
		)
		return nil, fmt.Errorf("failed to get Apple transaction history: %w", err)
	}

	var results []*ApplePurchaseResponse
	for _, response := range responses {
		transactions, err := s.storeClient.ParseSignedTransactions(response.SignedTransactions)
		if err != nil {
			s.logger.Error("failed to parse signed transactions",
				zap.Error(err),
				zap.String("original_transaction_id", originalTransactionID),
			)
			continue
		}

		for _, transaction := range transactions {
			purchaseDate := time.Unix(transaction.PurchaseDate/1000, 0)
			originalPurchaseDate := time.Unix(transaction.OriginalPurchaseDate/1000, 0)

			result := &ApplePurchaseResponse{
				TransactionID:         transaction.TransactionID,
				OriginalTransactionID: transaction.OriginalTransactionId,
				ProductID:             transaction.ProductID,
				BundleID:              transaction.BundleID,
				PurchaseDate:          purchaseDate,
				OriginalPurchaseDate:  originalPurchaseDate,
				Quantity:              int(transaction.Quantity),
				IsTrialPeriod:         transaction.Type == "Auto-Renewable Subscription" && transaction.OfferType == 0,
				IsInIntroOfferPeriod:  transaction.OfferType != 0,
				WebOrderLineItemID:    transaction.WebOrderLineItemId,
				SubscriptionGroupID:   "", // 需要从其他地方获取
				ProductType:           transaction.ProductID,
				InAppOwnershipType:    transaction.InAppOwnershipType,
				Environment:           s.getEnvironment(),
				Status:                "VERIFIED",
			}

			// 设置到期时间（如果是订阅）
			if transaction.ExpiresDate > 0 {
				expiresDate := time.Unix(transaction.ExpiresDate/1000, 0)
				result.ExpiresDate = &expiresDate
			}

			// 设置取消时间（如果有）
			if transaction.RevocationDate > 0 {
				cancellationDate := time.Unix(transaction.RevocationDate/1000, 0)
				result.CancellationDate = &cancellationDate
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// AppleNotification Apple通知结构体
type AppleNotification struct {
	NotificationType string                 `json:"notification_type"`
	Subtype          string                 `json:"subtype"`
	NotificationUUID string                 `json:"notification_uuid"`
	Data             *AppleNotificationData `json:"data"`
}

// AppleNotificationData 通知数据结构体
type AppleNotificationData struct {
	AppAppleID            int                   `json:"app_apple_id"`
	BundleID              string                `json:"bundle_id"`
	Environment           string                `json:"environment"`
	Status                int32                 `json:"status"`
	TransactionInfo       *AppleTransactionInfo `json:"transaction_info"`
	RenewalInfo           *AppleRenewalInfo     `json:"renewal_info"`
	SignedTransactionInfo string                `json:"signed_transaction_info"`
	SignedRenewalInfo     string                `json:"signed_renewal_info"`
}

// AppleTransactionInfo 交易信息
type AppleTransactionInfo struct {
	TransactionID         string     `json:"transaction_id"`
	OriginalTransactionID string     `json:"original_transaction_id"`
	ProductID             string     `json:"product_id"`
	BundleID              string     `json:"bundle_id"`
	PurchaseDate          time.Time  `json:"purchase_date"`
	OriginalPurchaseDate  time.Time  `json:"original_purchase_date"`
	ExpiresDate           *time.Time `json:"expires_date,omitempty"`
	Quantity              int64      `json:"quantity"`
	Type                  string     `json:"type"`
	InAppOwnershipType    string     `json:"in_app_ownership_type"`
	Environment           string     `json:"environment"`
	Price                 int64      `json:"price"`
	Currency              string     `json:"currency"`
	RevocationDate        *time.Time `json:"revocation_date,omitempty"`
	RevocationReason      int        `json:"revocation_reason,omitempty"`
	IsUpgraded            bool       `json:"is_upgraded"`
	OfferType             int        `json:"offer_type,omitempty"`
	OfferIdentifier       string     `json:"offer_identifier,omitempty"`
	SubscriptionGroupID   string     `json:"subscription_group_id,omitempty"`
	WebOrderLineItemID    string     `json:"web_order_line_item_id,omitempty"`
}

// AppleRenewalInfo 续订信息
type AppleRenewalInfo struct {
	OriginalTransactionID  string     `json:"original_transaction_id"`
	AutoRenewProductID     string     `json:"auto_renew_product_id"`
	AutoRenewStatus        int        `json:"auto_renew_status"`
	ExpirationIntent       int        `json:"expiration_intent,omitempty"`
	GracePeriodExpiresDate *time.Time `json:"grace_period_expires_date,omitempty"`
	IsInBillingRetryPeriod bool       `json:"is_in_billing_retry_period"`
	ProductID              string     `json:"product_id"`
	RenewalDate            *time.Time `json:"renewal_date,omitempty"`
	RenewalPrice           int64      `json:"renewal_price,omitempty"`
	Currency               string     `json:"currency,omitempty"`
}

// ParseNotification 解析Apple通知
// 参数：
//   - signedPayload: 签名的通知负载
//
// 返回：解析后的通知或错误
func (s *AppleService) ParseNotification(signedPayload string) (*AppleNotification, error) {
	// 使用 ParseNotificationV2WithClaim 解析通知
	payload := &appstore.SubscriptionNotificationV2DecodedPayload{}
	err := s.client.ParseNotificationV2WithClaim(signedPayload, payload)
	if err != nil {
		s.logger.Error("failed to parse Apple notification",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to parse Apple notification: %w", err)
	}

	notification := &AppleNotification{
		NotificationType: string(payload.NotificationType),
		Subtype:          string(payload.Subtype),
		NotificationUUID: payload.NotificationUUID,
		Data: &AppleNotificationData{
			AppAppleID:            payload.Data.AppAppleID,
			BundleID:              payload.Data.BundleID,
			Environment:           payload.Data.Environment,
			Status:                int32(payload.Data.Status),
			SignedTransactionInfo: string(payload.Data.SignedTransactionInfo),
			SignedRenewalInfo:     string(payload.Data.SignedRenewalInfo),
		},
	}

	// 解析签名的交易信息
	if payload.Data.SignedTransactionInfo != "" {
		transactionInfo, err := s.parseSignedTransactionInfo(string(payload.Data.SignedTransactionInfo))
		if err != nil {
			s.logger.Warn("failed to parse signed transaction info", zap.Error(err))
		} else {
			notification.Data.TransactionInfo = transactionInfo
		}
	}

	// 解析签名的续订信息
	if payload.Data.SignedRenewalInfo != "" {
		renewalInfo, err := s.parseSignedRenewalInfo(string(payload.Data.SignedRenewalInfo))
		if err != nil {
			s.logger.Warn("failed to parse signed renewal info", zap.Error(err))
		} else {
			notification.Data.RenewalInfo = renewalInfo
		}
	}

	return notification, nil
}

// parseSignedTransactionInfo 解析签名的交易信息
func (s *AppleService) parseSignedTransactionInfo(signedInfo string) (*AppleTransactionInfo, error) {
	payload := &appstore.JWSTransactionDecodedPayload{}

	// 使用JWT解析（签名已在ParseNotificationV2WithClaim中验证）
	token, _, err := jwt.NewParser().ParseUnverified(signedInfo, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transaction JWT: %w", err)
	}

	claims, ok := token.Claims.(*appstore.JWSTransactionDecodedPayload)
	if !ok {
		return nil, fmt.Errorf("invalid transaction claims")
	}

	info := &AppleTransactionInfo{
		TransactionID:         claims.TransactionId,
		OriginalTransactionID: claims.OriginalTransactionId,
		ProductID:             claims.ProductId,
		BundleID:              claims.BundleId,
		PurchaseDate:          time.Unix(claims.PurchaseDate/1000, 0),
		OriginalPurchaseDate:  time.Unix(claims.OriginalPurchaseDate/1000, 0),
		Quantity:              claims.Quantity,
		Type:                  string(claims.IAPtype),
		InAppOwnershipType:    claims.InAppOwnershipType,
		Environment:           string(claims.Environment),
		Price:                 claims.Price,
		Currency:              claims.Currency,
		IsUpgraded:            claims.IsUpgraded,
		OfferType:             int(claims.OfferType),
		OfferIdentifier:       claims.OfferIdentifier,
		SubscriptionGroupID:   claims.SubscriptionGroupIdentifier,
		WebOrderLineItemID:    claims.WebOrderLineItemId,
	}

	if claims.ExpiresDate > 0 {
		expiresDate := time.Unix(claims.ExpiresDate/1000, 0)
		info.ExpiresDate = &expiresDate
	}

	if claims.RevocationDate > 0 {
		revocationDate := time.Unix(claims.RevocationDate/1000, 0)
		info.RevocationDate = &revocationDate
		info.RevocationReason = int(claims.RevocationReason)
	}

	return info, nil
}

// parseSignedRenewalInfo 解析签名的续订信息
func (s *AppleService) parseSignedRenewalInfo(signedInfo string) (*AppleRenewalInfo, error) {
	payload := &appstore.JWSRenewalInfoDecodedPayload{}

	// 使用JWT解析
	token, _, err := jwt.NewParser().ParseUnverified(signedInfo, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse renewal JWT: %w", err)
	}

	claims, ok := token.Claims.(*appstore.JWSRenewalInfoDecodedPayload)
	if !ok {
		return nil, fmt.Errorf("invalid renewal claims")
	}

	info := &AppleRenewalInfo{
		OriginalTransactionID:  claims.OriginalTransactionId,
		AutoRenewProductID:     claims.AutoRenewProductId,
		AutoRenewStatus:        int(claims.AutoRenewStatus),
		ExpirationIntent:       int(claims.ExpirationIntent),
		IsInBillingRetryPeriod: claims.IsInBillingRetryPeriod,
		ProductID:              claims.ProductId,
		RenewalPrice:           claims.RenewalPrice,
		Currency:               claims.Currency,
	}

	if claims.GracePeriodExpiresDate > 0 {
		gracePeriodExpires := time.Unix(claims.GracePeriodExpiresDate/1000, 0)
		info.GracePeriodExpiresDate = &gracePeriodExpires
	}

	if claims.RenewalDate > 0 {
		renewalDate := time.Unix(claims.RenewalDate/1000, 0)
		info.RenewalDate = &renewalDate
	}

	return info, nil
}

// HandleNotification 处理Apple通知并更新订单状态
func (s *AppleService) HandleNotification(ctx context.Context, notification *AppleNotification) error {
	s.logger.Info("Processing Apple notification",
		zap.String("notification_type", notification.NotificationType),
		zap.String("subtype", notification.Subtype),
		zap.String("notification_uuid", notification.NotificationUUID),
	)

	if notification.Data == nil || notification.Data.TransactionInfo == nil {
		s.logger.Warn("Notification has no transaction info, skipping")
		return nil
	}

	transactionInfo := notification.Data.TransactionInfo

	// 根据通知类型处理
	switch notification.NotificationType {
	case "SUBSCRIBED":
		return s.handleSubscribed(ctx, notification, transactionInfo)
	case "DID_RENEW":
		return s.handleDidRenew(ctx, notification, transactionInfo)
	case "DID_FAIL_TO_RENEW":
		return s.handleDidFailToRenew(ctx, notification, transactionInfo)
	case "DID_CHANGE_RENEWAL_STATUS":
		return s.handleDidChangeRenewalStatus(ctx, notification, transactionInfo)
	case "DID_CHANGE_RENEWAL_PREF":
		return s.handleDidChangeRenewalPref(ctx, notification, transactionInfo)
	case "EXPIRED":
		return s.handleExpired(ctx, notification, transactionInfo)
	case "GRACE_PERIOD_EXPIRED":
		return s.handleGracePeriodExpired(ctx, notification, transactionInfo)
	case "REFUND":
		return s.handleRefund(ctx, notification, transactionInfo)
	case "REFUND_REVERSED":
		return s.handleRefundReversed(ctx, notification, transactionInfo)
	case "REVOKE":
		return s.handleRevoke(ctx, notification, transactionInfo)
	case "CONSUMPTION_REQUEST":
		return s.handleConsumptionRequest(ctx, notification, transactionInfo)
	case "ONE_TIME_CHARGE":
		return s.handleOneTimeCharge(ctx, notification, transactionInfo)
	case "TEST":
		s.logger.Info("Received TEST notification")
		return nil
	default:
		s.logger.Warn("Unknown notification type",
			zap.String("notification_type", notification.NotificationType),
		)
		return nil
	}
}

// handleSubscribed 处理订阅成功通知
func (s *AppleService) handleSubscribed(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling SUBSCRIBED notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
		zap.String("product_id", transactionInfo.ProductID),
		zap.String("subtype", notification.Subtype),
	)

	// 查找或创建Apple支付记录
	return s.upsertApplePayment(ctx, transactionInfo, notification)
}

// handleDidRenew 处理续订成功通知
func (s *AppleService) handleDidRenew(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling DID_RENEW notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
	)

	// 更新支付记录
	return s.upsertApplePayment(ctx, transactionInfo, notification)
}

// handleDidFailToRenew 处理续订失败通知
func (s *AppleService) handleDidFailToRenew(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling DID_FAIL_TO_RENEW notification",
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
		zap.String("subtype", notification.Subtype),
	)

	// 更新支付记录状态
	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("original_transaction_id = ?", transactionInfo.OriginalTransactionID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Apple payment not found for DID_FAIL_TO_RENEW",
				zap.String("original_transaction_id", transactionInfo.OriginalTransactionID))
			return nil
		}
		return err
	}

	// 更新状态
	payment.Status = "RENEWAL_FAILED"
	if notification.Subtype == "GRACE_PERIOD" {
		payment.GracePeriodStatus = "IN_GRACE_PERIOD"
	} else {
		// 非宽限期内的续订失败，更新订单状态为失败
		if payment.OrderID > 0 {
			s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", payment.OrderID).
				Updates(map[string]interface{}{
					"payment_status": models.PaymentStatusFailed,
				})
		}
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleDidChangeRenewalStatus 处理续订状态变更
func (s *AppleService) handleDidChangeRenewalStatus(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling DID_CHANGE_RENEWAL_STATUS notification",
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
		zap.String("subtype", notification.Subtype),
	)

	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("original_transaction_id = ?", transactionInfo.OriginalTransactionID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	// 更新自动续订状态
	if notification.Data.RenewalInfo != nil {
		autoRenewStatus := notification.Data.RenewalInfo.AutoRenewStatus == 1
		payment.AutoRenewStatus = &autoRenewStatus
		payment.AutoRenewProductID = notification.Data.RenewalInfo.AutoRenewProductID
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleDidChangeRenewalPref 处理续订偏好变更
func (s *AppleService) handleDidChangeRenewalPref(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling DID_CHANGE_RENEWAL_PREF notification",
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
	)

	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("original_transaction_id = ?", transactionInfo.OriginalTransactionID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	if notification.Data.RenewalInfo != nil {
		payment.AutoRenewProductID = notification.Data.RenewalInfo.AutoRenewProductID
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleExpired 处理订阅过期
func (s *AppleService) handleExpired(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling EXPIRED notification",
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
		zap.String("subtype", notification.Subtype),
	)

	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("original_transaction_id = ?", transactionInfo.OriginalTransactionID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	payment.Status = "EXPIRED"
	if notification.Data.RenewalInfo != nil {
		payment.ExpirationIntent = fmt.Sprintf("%d", notification.Data.RenewalInfo.ExpirationIntent)
	}

	// 更新订单状态
	if payment.OrderID > 0 {
		s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", payment.OrderID).
			Update("status", models.OrderStatusExpired)
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleGracePeriodExpired 处理宽限期过期
func (s *AppleService) handleGracePeriodExpired(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling GRACE_PERIOD_EXPIRED notification",
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
	)

	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("original_transaction_id = ?", transactionInfo.OriginalTransactionID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	payment.Status = "GRACE_PERIOD_EXPIRED"
	payment.GracePeriodStatus = "EXPIRED"

	// 更新订单状态为过期
	if payment.OrderID > 0 {
		s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", payment.OrderID).
			Updates(map[string]interface{}{
				"status":         models.OrderStatusExpired,
				"payment_status": models.PaymentStatusExpired,
			})
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleRefund 处理退款
func (s *AppleService) handleRefund(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling REFUND notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
		zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
	)

	// 查找对应的支付记录
	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("transaction_id = ?", transactionInfo.TransactionID).
		First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Apple payment not found for REFUND",
				zap.String("transaction_id", transactionInfo.TransactionID))
			return nil
		}
		return err
	}

	// 创建退款记录
	refund := &models.AppleRefund{
		OrderID:               payment.OrderID,
		ApplePaymentID:        payment.ID,
		RefundTransactionID:   transactionInfo.TransactionID,
		OriginalTransactionID: transactionInfo.OriginalTransactionID,
		RefundStatus:          "REFUNDED",
	}

	if transactionInfo.RevocationDate != nil {
		refund.RefundDate = transactionInfo.RevocationDate
	}

	if err := s.db.WithContext(ctx).Create(refund).Error; err != nil {
		return err
	}

	// 更新支付记录状态
	payment.Status = "REFUNDED"
	payment.RevocationDate = transactionInfo.RevocationDate
	if transactionInfo.RevocationReason > 0 {
		payment.RevocationReason = fmt.Sprintf("%d", transactionInfo.RevocationReason)
	}

	// 更新订单状态
	if payment.OrderID > 0 {
		s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", payment.OrderID).
			Update("status", models.OrderStatusRefunded)
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleRefundReversed 处理退款撤销
func (s *AppleService) handleRefundReversed(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling REFUND_REVERSED notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
	)

	// 更新退款记录
	s.db.WithContext(ctx).Model(&models.AppleRefund{}).
		Where("refund_transaction_id = ?", transactionInfo.TransactionID).
		Update("refund_status", "REVERSED")

	// 更新支付记录
	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("transaction_id = ?", transactionInfo.TransactionID).
		First(&payment).Error; err == nil {
		payment.Status = "VERIFIED"
		payment.RevocationDate = nil
		payment.RevocationReason = ""
		s.db.WithContext(ctx).Save(&payment)

		// 恢复订单状态（退款被撤销意味着订单重新有效）
		if payment.OrderID > 0 {
			s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", payment.OrderID).
				Updates(map[string]interface{}{
					"status":         models.OrderStatusPaid,
					"payment_status": models.PaymentStatusCompleted,
				})
		}
	}

	return nil
}

// handleRevoke 处理撤销
func (s *AppleService) handleRevoke(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling REVOKE notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
	)

	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("transaction_id = ?", transactionInfo.TransactionID).
		First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	payment.Status = "REVOKED"
	payment.RevocationDate = transactionInfo.RevocationDate
	if transactionInfo.RevocationReason > 0 {
		payment.RevocationReason = fmt.Sprintf("%d", transactionInfo.RevocationReason)
	}

	// 更新订单状态（撤销通常因为家庭共享被移除等原因）
	if payment.OrderID > 0 {
		s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", payment.OrderID).
			Updates(map[string]interface{}{
				"status":         models.OrderStatusCancelled,
				"payment_status": models.PaymentStatusCancelled,
			})
	}

	return s.db.WithContext(ctx).Save(&payment).Error
}

// handleConsumptionRequest 处理消耗请求
func (s *AppleService) handleConsumptionRequest(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling CONSUMPTION_REQUEST notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
	)
	// 消耗请求需要业务方响应，这里只记录日志
	return nil
}

// handleOneTimeCharge 处理一次性购买
func (s *AppleService) handleOneTimeCharge(ctx context.Context, notification *AppleNotification, transactionInfo *AppleTransactionInfo) error {
	s.logger.Info("Handling ONE_TIME_CHARGE notification",
		zap.String("transaction_id", transactionInfo.TransactionID),
		zap.String("product_id", transactionInfo.ProductID),
	)

	return s.upsertApplePayment(ctx, transactionInfo, notification)
}

// upsertApplePayment 创建或更新Apple支付记录
func (s *AppleService) upsertApplePayment(ctx context.Context, transactionInfo *AppleTransactionInfo, notification *AppleNotification) error {
	var payment models.ApplePayment
	err := s.db.WithContext(ctx).Where("transaction_id = ?", transactionInfo.TransactionID).First(&payment).Error

	if err == gorm.ErrRecordNotFound {
		// 尝试通过 original_transaction_id 查找已有的 ApplePayment 获取 OrderID
		// 这是关键：首次购买时客户端会调用 verify-receipt/verify-transaction 并传入 order_id
		// 后续的续订/退款等 webhook 可以通过 original_transaction_id 关联到同一个订单
		var orderID uint
		var existingPayment models.ApplePayment
		if err := s.db.WithContext(ctx).
			Where("original_transaction_id = ?", transactionInfo.OriginalTransactionID).
			Order("created_at ASC"). // 找到最早的那条记录（首次购买）
			First(&existingPayment).Error; err == nil && existingPayment.OrderID > 0 {
			orderID = existingPayment.OrderID
			s.logger.Info("Found existing order for Apple transaction",
				zap.Uint("order_id", orderID),
				zap.String("original_transaction_id", transactionInfo.OriginalTransactionID),
			)
		}

		// 创建新记录
		payment = models.ApplePayment{
			OrderID:               orderID, // 关联到订单（如果找到）
			TransactionID:         transactionInfo.TransactionID,
			OriginalTransactionID: transactionInfo.OriginalTransactionID,
			ProductIDApple:        transactionInfo.ProductID,
			BundleID:              transactionInfo.BundleID,
			Quantity:              int(transactionInfo.Quantity),
			PurchaseDate:          &transactionInfo.PurchaseDate,
			OriginalPurchaseDate:  &transactionInfo.OriginalPurchaseDate,
			ExpiresDate:           transactionInfo.ExpiresDate,
			ProductType:           transactionInfo.Type,
			InAppOwnershipType:    transactionInfo.InAppOwnershipType,
			SubscriptionGroupID:   transactionInfo.SubscriptionGroupID,
			WebOrderLineItemID:    transactionInfo.WebOrderLineItemID,
			Price:                 transactionInfo.Price,
			Currency:              transactionInfo.Currency,
			Environment:           transactionInfo.Environment,
			Status:                "VERIFIED",
		}

		if notification.Data.RenewalInfo != nil {
			autoRenewStatus := notification.Data.RenewalInfo.AutoRenewStatus == 1
			payment.AutoRenewStatus = &autoRenewStatus
			payment.AutoRenewProductID = notification.Data.RenewalInfo.AutoRenewProductID
		}

		if err := s.db.WithContext(ctx).Create(&payment).Error; err != nil {
			return err
		}

		// 如果找到了关联订单，更新订单状态
		if orderID > 0 {
			s.updateOrderStatusFromNotification(ctx, orderID, notification)
		}

		return nil
	} else if err != nil {
		return err
	}

	// 更新现有记录
	payment.ExpiresDate = transactionInfo.ExpiresDate
	payment.Status = "VERIFIED"
	if notification.Data.RenewalInfo != nil {
		autoRenewStatus := notification.Data.RenewalInfo.AutoRenewStatus == 1
		payment.AutoRenewStatus = &autoRenewStatus
		payment.AutoRenewProductID = notification.Data.RenewalInfo.AutoRenewProductID
	}

	if err := s.db.WithContext(ctx).Save(&payment).Error; err != nil {
		return err
	}

	// 更新关联订单状态
	if payment.OrderID > 0 {
		s.updateOrderStatusFromNotification(ctx, payment.OrderID, notification)
	}

	return nil
}

// updateOrderStatusFromNotification 根据 Apple 通知类型更新订单状态
func (s *AppleService) updateOrderStatusFromNotification(ctx context.Context, orderID uint, notification *AppleNotification) {
	var order models.Order
	if err := s.db.WithContext(ctx).First(&order, orderID).Error; err != nil {
		s.logger.Error("Failed to find order for status update",
			zap.Uint("order_id", orderID),
			zap.Error(err),
		)
		return
	}

	var newStatus models.OrderStatus
	var newPaymentStatus models.PaymentStatus
	needUpdate := false

	switch notification.NotificationType {
	case "SUBSCRIBED", "DID_RENEW":
		// 订阅成功或续订成功
		newStatus = models.OrderStatusPaid
		newPaymentStatus = models.PaymentStatusCompleted
		needUpdate = true
	case "EXPIRED", "GRACE_PERIOD_EXPIRED":
		// 订阅过期
		newStatus = models.OrderStatusExpired
		newPaymentStatus = models.PaymentStatusExpired
		needUpdate = true
	case "REFUND":
		// 退款
		newStatus = models.OrderStatusRefunded
		newPaymentStatus = models.PaymentStatusRefunded
		needUpdate = true
	case "REVOKE":
		// 撤销（如家庭共享被移除）
		newStatus = models.OrderStatusCancelled
		newPaymentStatus = models.PaymentStatusCancelled
		needUpdate = true
	case "DID_FAIL_TO_RENEW":
		// 续订失败，但根据 subtype 可能在宽限期内
		if notification.Subtype == "GRACE_PERIOD" {
			// 在宽限期内，订单仍然有效
			s.logger.Info("Subscription in grace period",
				zap.Uint("order_id", orderID),
			)
		} else {
			newStatus = models.OrderStatusExpired
			newPaymentStatus = models.PaymentStatusFailed
			needUpdate = true
		}
	}

	if needUpdate {
		order.Status = newStatus
		order.PaymentStatus = newPaymentStatus
		if err := s.db.WithContext(ctx).Save(&order).Error; err != nil {
			s.logger.Error("Failed to update order status",
				zap.Uint("order_id", orderID),
				zap.String("notification_type", notification.NotificationType),
				zap.Error(err),
			)
		} else {
			s.logger.Info("Order status updated from Apple notification",
				zap.Uint("order_id", orderID),
				zap.String("notification_type", notification.NotificationType),
				zap.String("new_status", string(newStatus)),
			)
		}
	}
}

// GetSubscriptionStatus 获取订阅状态
func (s *AppleService) GetSubscriptionStatus(ctx context.Context, originalTransactionID string) (*AppleSubscriptionResponse, error) {
	// 首先查询本地数据库
	var payment models.ApplePayment
	if err := s.db.WithContext(ctx).Where("original_transaction_id = ?", originalTransactionID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	// 尝试从Apple获取最新状态
	transactions, err := s.GetTransactionHistory(ctx, originalTransactionID)
	if err != nil {
		// 如果获取失败但有本地记录，返回本地数据
		if payment.ID > 0 {
			return s.convertPaymentToSubscriptionResponse(&payment), nil
		}
		return nil, err
	}

	if len(transactions) == 0 {
		if payment.ID > 0 {
			return s.convertPaymentToSubscriptionResponse(&payment), nil
		}
		return nil, fmt.Errorf("no subscription found for original_transaction_id: %s", originalTransactionID)
	}

	// 返回最新的交易信息
	latest := transactions[len(transactions)-1]
	return &AppleSubscriptionResponse{
		ApplePurchaseResponse: *latest,
		AutoRenewStatus:       payment.AutoRenewStatus != nil && *payment.AutoRenewStatus,
		AutoRenewProductID:    payment.AutoRenewProductID,
		GracePeriodStatus:     payment.GracePeriodStatus,
		ExpirationIntent:      payment.ExpirationIntent,
	}, nil
}

// convertPaymentToSubscriptionResponse 将支付记录转换为订阅响应
func (s *AppleService) convertPaymentToSubscriptionResponse(payment *models.ApplePayment) *AppleSubscriptionResponse {
	response := &AppleSubscriptionResponse{
		ApplePurchaseResponse: ApplePurchaseResponse{
			TransactionID:         payment.TransactionID,
			OriginalTransactionID: payment.OriginalTransactionID,
			ProductID:             payment.ProductIDApple,
			BundleID:              payment.BundleID,
			Quantity:              payment.Quantity,
			ExpiresDate:           payment.ExpiresDate,
			CancellationDate:      payment.CancellationDate,
			SubscriptionGroupID:   payment.SubscriptionGroupID,
			ProductType:           payment.ProductType,
			InAppOwnershipType:    payment.InAppOwnershipType,
			WebOrderLineItemID:    payment.WebOrderLineItemID,
			Environment:           payment.Environment,
			Status:                payment.Status,
		},
		AutoRenewProductID: payment.AutoRenewProductID,
		GracePeriodStatus:  payment.GracePeriodStatus,
		ExpirationIntent:   payment.ExpirationIntent,
	}

	if payment.PurchaseDate != nil {
		response.PurchaseDate = *payment.PurchaseDate
	}
	if payment.OriginalPurchaseDate != nil {
		response.OriginalPurchaseDate = *payment.OriginalPurchaseDate
	}
	if payment.IsTrialPeriod != nil {
		response.IsTrialPeriod = *payment.IsTrialPeriod
	}
	if payment.IsInIntroOfferPeriod != nil {
		response.IsInIntroOfferPeriod = *payment.IsInIntroOfferPeriod
	}
	if payment.AutoRenewStatus != nil {
		response.AutoRenewStatus = *payment.AutoRenewStatus
	}

	return response
}

// SaveApplePayment 保存Apple支付信息到数据库
// 参数：
//   - ctx: 上下文
//   - orderID: 订单ID
//   - response: Apple购买响应
//
// 返回：错误或nil
func (s *AppleService) SaveApplePayment(ctx context.Context, orderID uint, response *ApplePurchaseResponse) error {
	applePayment := &models.ApplePayment{
		OrderID:               orderID,
		TransactionID:         response.TransactionID,
		OriginalTransactionID: response.OriginalTransactionID,
		ProductIDApple:        response.ProductID,
		BundleID:              response.BundleID,
		Quantity:              response.Quantity,
		PurchaseDate:          &response.PurchaseDate,
		OriginalPurchaseDate:  &response.OriginalPurchaseDate,
		IsTrialPeriod:         &response.IsTrialPeriod,
		IsInIntroOfferPeriod:  &response.IsInIntroOfferPeriod,
		SubscriptionGroupID:   response.SubscriptionGroupID,
		ProductType:           response.ProductType,
		InAppOwnershipType:    response.InAppOwnershipType,
		WebOrderLineItemID:    response.WebOrderLineItemID,
		Environment:           response.Environment,
		Status:                response.Status,
	}

	// 设置可选字段
	if response.ExpiresDate != nil {
		applePayment.ExpiresDate = response.ExpiresDate
	}
	if response.CancellationDate != nil {
		applePayment.CancellationDate = response.CancellationDate
	}

	if err := s.db.WithContext(ctx).Create(applePayment).Error; err != nil {
		s.logger.Error("failed to save Apple payment",
			zap.Error(err),
			zap.Uint("order_id", orderID),
			zap.String("transaction_id", response.TransactionID),
		)
		return fmt.Errorf("failed to save Apple payment: %w", err)
	}

	s.logger.Info("Apple payment saved successfully",
		zap.Uint("order_id", orderID),
		zap.String("transaction_id", response.TransactionID),
	)

	return nil
}

// getEnvironment 获取当前环境
func (s *AppleService) getEnvironment() string {
	if s.config.Apple.Sandbox {
		return "Sandbox"
	}
	return "Production"
}
