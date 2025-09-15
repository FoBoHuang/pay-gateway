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

// ParseNotification 解析Apple通知
// 参数：
//   - signedPayload: 签名的通知负载
//
// 返回：解析后的通知或错误
func (s *AppleService) ParseNotification(signedPayload string) (*jwt.Token, error) {
	token := &jwt.Token{}

	err := s.client.ParseNotificationV2(signedPayload, token)
	if err != nil {
		s.logger.Error("failed to parse Apple notification",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to parse Apple notification: %w", err)
	}

	// 从token中提取通知数据
	_, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid notification claims")
	}

	// 这里需要根据实际的通知结构来处理
	// 返回一个简化的通知对象
	return token, nil
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
