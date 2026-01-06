package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/services"
)

// AppleWebhookHandler Apple Webhook处理器
type AppleWebhookHandler struct {
	db             *gorm.DB
	appleService   *services.AppleService
	paymentService services.PaymentService
	logger         *zap.Logger
}

// NewAppleWebhookHandler 创建Apple Webhook处理器
func NewAppleWebhookHandler(
	db *gorm.DB,
	appleService *services.AppleService,
	paymentService services.PaymentService,
	_ interface{}, // 保留参数位置以保持向后兼容
	logger *zap.Logger,
) *AppleWebhookHandler {
	return &AppleWebhookHandler{
		db:             db,
		appleService:   appleService,
		paymentService: paymentService,
		logger:         logger,
	}
}

// AppleWebhookRequest Apple Webhook请求结构
type AppleWebhookRequest struct {
	SignedPayload string `json:"signedPayload"`
}

// HandleAppleWebhook 处理Apple Webhook
// @Summary 处理Apple Webhook
// @Description 接收并处理Apple的Webhook通知
// @Tags Webhook
// @Accept json
// @Produce json
// @Param request body AppleWebhookRequest true "Webhook请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /webhook/apple [post]
func (h *AppleWebhookHandler) HandleAppleWebhook(c *gin.Context) {
	ctx := context.Background()

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("failed to read Apple webhook body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request body",
		})
		return
	}

	var request AppleWebhookRequest
	if err := json.Unmarshal(body, &request); err != nil {
		h.logger.Error("failed to unmarshal Apple webhook request",
			zap.Error(err),
			zap.String("body", string(body)),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// 验证签名
	if err := h.validateSignature(c.Request); err != nil {
		h.logger.Error("Apple webhook signature validation failed",
			zap.Error(err),
		)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid signature",
		})
		return
	}

	// 解析通知
	notification, err := h.appleService.ParseNotification(request.SignedPayload)
	if err != nil {
		h.logger.Error("failed to parse Apple notification",
			zap.Error(err),
			zap.String("signed_payload", request.SignedPayload),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to parse notification",
		})
		return
	}

	// 处理通知
	if err := h.processNotification(ctx, notification); err != nil {
		h.logger.Error("failed to process Apple notification",
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process notification",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notification processed successfully",
	})
}

// validateSignature 验证Apple Webhook签名
func (h *AppleWebhookHandler) validateSignature(r *http.Request) error {
	// Apple使用JWT签名，我们需要验证JWT的有效性
	// 这里需要根据Apple的文档实现具体的签名验证逻辑
	// 暂时返回nil，实际需要实现完整的签名验证

	// TODO: 实现Apple Webhook签名验证
	// 参考：https://developer.apple.com/documentation/appstoreservernotifications/signed_payloads

	return nil
}

// processNotification 处理Apple通知
func (h *AppleWebhookHandler) processNotification(ctx context.Context, notification *jwt.Token) error {
	h.logger.Info("Processing Apple notification",
		zap.Any("notification", notification),
	)

	// 这里需要根据实际的通知类型来处理不同的业务逻辑
	// 暂时只记录日志，实际需要实现具体的业务逻辑
	h.logger.Info("Apple notification processed",
		zap.Any("notification", notification),
	)

	return nil
}
