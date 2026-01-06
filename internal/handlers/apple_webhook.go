package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
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
// @Description 接收并处理Apple App Store Server Notifications V2
// @Tags Apple Webhook
// @Accept json
// @Produce json
// @Param request body AppleWebhookRequest true "Webhook请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /webhook/apple [post]
func (h *AppleWebhookHandler) HandleAppleWebhook(c *gin.Context) {
	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read Apple webhook body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request body",
		})
		return
	}

	h.logger.Debug("Received Apple webhook",
		zap.String("body", string(body)),
	)

	// 解析请求
	var request AppleWebhookRequest

	// Apple发送的是 {"signedPayload": "xxx"} 格式
	if err := c.ShouldBindJSON(&request); err != nil {
		// 尝试直接使用body作为signedPayload（某些情况下可能直接发送JWT字符串）
		request.SignedPayload = string(body)
	}

	if request.SignedPayload == "" {
		h.logger.Error("Empty signedPayload in Apple webhook")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Empty signedPayload",
		})
		return
	}

	// 解析通知（ParseNotification内部会验证签名）
	notification, err := h.appleService.ParseNotification(request.SignedPayload)
	if err != nil {
		h.logger.Error("Failed to parse Apple notification",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to parse notification",
		})
		return
	}

	h.logger.Info("Parsed Apple notification",
		zap.String("notification_type", notification.NotificationType),
		zap.String("subtype", notification.Subtype),
		zap.String("notification_uuid", notification.NotificationUUID),
	)

	// 处理通知
	if err := h.appleService.HandleNotification(c.Request.Context(), notification); err != nil {
		h.logger.Error("Failed to process Apple notification",
			zap.Error(err),
			zap.String("notification_type", notification.NotificationType),
			zap.String("notification_uuid", notification.NotificationUUID),
		)
		// 即使处理失败，也返回200，避免Apple重复发送
		// 但记录错误以便后续处理
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "Notification received but processing failed",
			"error":   err.Error(),
		})
		return
	}

	h.logger.Info("Apple notification processed successfully",
		zap.String("notification_type", notification.NotificationType),
		zap.String("notification_uuid", notification.NotificationUUID),
	)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notification processed successfully",
	})
}
