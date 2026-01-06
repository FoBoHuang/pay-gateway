package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/models"
	"pay-gateway/internal/services"
)

// GoogleWebhookHandler Google Play Webhook处理器
type GoogleWebhookHandler struct {
	db             *gorm.DB
	googleService  *services.GooglePlayService
	paymentService services.PaymentService
	logger         *zap.Logger
}

// NewGoogleWebhookHandler 创建Google Play Webhook处理器
func NewGoogleWebhookHandler(
	db *gorm.DB,
	googleService *services.GooglePlayService,
	paymentService services.PaymentService,
	logger *zap.Logger,
) *GoogleWebhookHandler {
	return &GoogleWebhookHandler{
		db:             db,
		googleService:  googleService,
		paymentService: paymentService,
		logger:         logger,
	}
}

// ==================== Webhook请求结构体 ====================

// GooglePlayWebhookRequest Google Play Webhook请求结构
type GooglePlayWebhookRequest struct {
	Message struct {
		Data        string `json:"data"`
		MessageID   string `json:"messageId"`
		PublishTime string `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// GoogleWebhookData Webhook数据
type GoogleWebhookData struct {
	Version                    string                         `json:"version"`
	PackageName                string                         `json:"packageName"`
	EventTimeMillis            string                         `json:"eventTimeMillis"`
	OneTimeProductNotification *GoogleOneTimeProductNotification `json:"oneTimeProductNotification,omitempty"`
	SubscriptionNotification   *GoogleSubscriptionNotification   `json:"subscriptionNotification,omitempty"`
	TestNotification           *GoogleTestNotification           `json:"testNotification,omitempty"`
}

// GoogleOneTimeProductNotification 一次性产品通知
type GoogleOneTimeProductNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notificationType"`
	PurchaseToken    string `json:"purchaseToken"`
	SKU              string `json:"sku"`
}

// GoogleSubscriptionNotification 订阅通知
type GoogleSubscriptionNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notificationType"`
	PurchaseToken    string `json:"purchaseToken"`
	SubscriptionID   string `json:"subscriptionId"`
}

// GoogleTestNotification 测试通知
type GoogleTestNotification struct {
	Version string `json:"version"`
}

// ==================== Webhook处理 ====================

// HandleGooglePlayWebhook 处理Google Play Webhook
// @Summary 处理Google Play Webhook
// @Description 接收并处理Google Play的Webhook通知（订阅、购买状态变更）
// @Tags Google Play Webhook
// @Accept json
// @Produce json
// @Param request body GooglePlayWebhookRequest true "Webhook请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /webhook/google [post]
func (h *GoogleWebhookHandler) HandleGooglePlayWebhook(c *gin.Context) {
	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("读取Google Webhook请求体失败", zap.Error(err))
		ErrorJSON(c, 400, "读取请求体失败", err)
		return
	}

	// 解析Webhook请求
	var webhookReq GooglePlayWebhookRequest
	if err := json.Unmarshal(body, &webhookReq); err != nil {
		h.logger.Error("解析Google Webhook请求失败", zap.Error(err))
		ErrorJSON(c, 400, "解析请求失败", err)
		return
	}

	// 解码数据
	decodedData, err := base64.StdEncoding.DecodeString(webhookReq.Message.Data)
	if err != nil {
		h.logger.Error("解码Google Webhook数据失败", zap.Error(err))
		ErrorJSON(c, 400, "解码数据失败", err)
		return
	}

	// 解析Webhook数据
	var webhookData GoogleWebhookData
	if err := json.Unmarshal(decodedData, &webhookData); err != nil {
		h.logger.Error("解析Google Webhook数据失败", zap.Error(err))
		ErrorJSON(c, 400, "解析数据失败", err)
		return
	}

	// 创建Webhook事件记录
	webhookEvent := &models.WebhookEvent{
		EventID:     webhookReq.Message.MessageID,
		Type:        h.determineWebhookType(&webhookData),
		Version:     webhookData.Version,
		PackageName: webhookData.PackageName,
		EventTime:   h.parseEventTime(webhookData.EventTimeMillis),
		Status:      models.WebhookStatusPending,
		RawPayload: models.JSON{
			"version":           webhookData.Version,
			"package_name":      webhookData.PackageName,
			"event_time_millis": webhookData.EventTimeMillis,
		},
		Processed: false,
	}

	// 设置通知数据
	if webhookData.OneTimeProductNotification != nil {
		webhookEvent.OneTimeProductNotification = &models.OneTimeProductNotification{
			Version:          webhookData.OneTimeProductNotification.Version,
			NotificationType: webhookData.OneTimeProductNotification.NotificationType,
			PurchaseToken:    webhookData.OneTimeProductNotification.PurchaseToken,
			SKU:              webhookData.OneTimeProductNotification.SKU,
		}
	}

	if webhookData.SubscriptionNotification != nil {
		webhookEvent.SubscriptionNotification = &models.SubscriptionNotification{
			Version:          webhookData.SubscriptionNotification.Version,
			NotificationType: webhookData.SubscriptionNotification.NotificationType,
			PurchaseToken:    webhookData.SubscriptionNotification.PurchaseToken,
			SubscriptionID:   webhookData.SubscriptionNotification.SubscriptionID,
		}
	}

	if webhookData.TestNotification != nil {
		webhookEvent.TestNotification = &models.TestNotification{
			Version: webhookData.TestNotification.Version,
		}
	}

	// 保存Webhook事件
	if err := h.db.Create(webhookEvent).Error; err != nil {
		h.logger.Error("保存Google Webhook事件失败", zap.Error(err))
		ErrorJSON(c, 500, "保存事件失败", err)
		return
	}

	// 异步处理Webhook事件
	go h.processWebhookEvent(context.Background(), webhookEvent)

	h.logger.Info("Google Webhook事件接收成功",
		zap.String("event_id", webhookEvent.EventID),
		zap.String("type", string(webhookEvent.Type)))

	SuccessJSON(c, gin.H{
		"message":  "Webhook received successfully",
		"event_id": webhookEvent.EventID,
	})
}

// processWebhookEvent 处理Webhook事件
func (h *GoogleWebhookHandler) processWebhookEvent(ctx context.Context, event *models.WebhookEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("处理Google Webhook事件时发生panic", zap.Any("panic", r))
			event.MarkAsFailed(fmt.Sprintf("Panic: %v", r))
			h.db.Save(event)
		}
	}()

	h.logger.Info("开始处理Google Webhook事件",
		zap.String("event_id", event.EventID),
		zap.String("type", string(event.Type)))

	// 根据事件类型处理
	switch {
	case event.IsTestEvent():
		h.processTestEvent(ctx, event)
	case event.IsOneTimeProductEvent():
		h.processOneTimeProductEvent(ctx, event)
	case event.IsSubscriptionEvent():
		h.processSubscriptionEvent(ctx, event)
	default:
		h.logger.Warn("未知的Google Webhook事件类型", zap.String("event_id", event.EventID))
		event.MarkAsFailed("未知的事件类型")
		h.db.Save(event)
		return
	}

	// 标记为已处理
	event.MarkAsProcessed()
	if err := h.db.Save(event).Error; err != nil {
		h.logger.Error("更新Google Webhook事件状态失败", zap.Error(err))
	}

	h.logger.Info("Google Webhook事件处理完成",
		zap.String("event_id", event.EventID),
		zap.String("status", string(event.Status)))
}

// processTestEvent 处理测试事件
func (h *GoogleWebhookHandler) processTestEvent(_ context.Context, event *models.WebhookEvent) {
	h.logger.Info("处理Google测试事件", zap.String("event_id", event.EventID))

	event.ProcessedData = models.JSON{
		"message": "Test notification received",
		"version": event.TestNotification.Version,
	}
}

// processOneTimeProductEvent 处理一次性产品事件
func (h *GoogleWebhookHandler) processOneTimeProductEvent(ctx context.Context, event *models.WebhookEvent) {
	notification := event.OneTimeProductNotification
	h.logger.Info("处理Google一次性产品事件",
		zap.String("event_id", event.EventID),
		zap.String("sku", notification.SKU),
		zap.Int("notification_type", notification.NotificationType))

	switch notification.NotificationType {
	case models.OneTimeProductNotificationTypePurchased:
		h.handleOneTimeProductPurchased(ctx, event, notification)
	case models.OneTimeProductNotificationTypeCanceled:
		h.handleOneTimeProductCanceled(ctx, event, notification)
	default:
		h.logger.Warn("未知的一次性产品通知类型",
			zap.Int("notification_type", notification.NotificationType))
		event.MarkAsFailed("未知的通知类型")
	}
}

// processSubscriptionEvent 处理订阅事件
func (h *GoogleWebhookHandler) processSubscriptionEvent(ctx context.Context, event *models.WebhookEvent) {
	notification := event.SubscriptionNotification
	h.logger.Info("处理Google订阅事件",
		zap.String("event_id", event.EventID),
		zap.String("subscription_id", notification.SubscriptionID),
		zap.Int("notification_type", notification.NotificationType))

	switch notification.NotificationType {
	case models.SubscriptionNotificationTypePurchased:
		h.handleSubscriptionPurchased(ctx, event, notification)
	case models.SubscriptionNotificationTypeRenewed:
		h.handleSubscriptionRenewed(ctx, event, notification)
	case models.SubscriptionNotificationTypeCanceled:
		h.handleSubscriptionCanceled(ctx, event, notification)
	case models.SubscriptionNotificationTypeExpired:
		h.handleSubscriptionExpired(ctx, event, notification)
	case models.SubscriptionNotificationTypeInGracePeriod:
		h.handleSubscriptionInGracePeriod(ctx, event, notification)
	case models.SubscriptionNotificationTypeRevoked:
		h.handleSubscriptionRevoked(ctx, event, notification)
	default:
		h.logger.Warn("未知的订阅通知类型",
			zap.Int("notification_type", notification.NotificationType))
		event.MarkAsFailed("未知的通知类型")
	}
}

// ==================== 一次性产品事件处理 ====================

func (h *GoogleWebhookHandler) handleOneTimeProductPurchased(ctx context.Context, event *models.WebhookEvent, notification *models.OneTimeProductNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订单")
			return
		}
		h.logger.Error("查询订单失败", zap.Error(err))
		event.MarkAsFailed("查询订单失败")
		return
	}

	if err := h.paymentService.UpdateOrderStatus(ctx, order.ID, models.OrderStatusPaid); err != nil {
		h.logger.Error("更新订单状态失败", zap.Error(err))
		event.MarkAsFailed("更新订单状态失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "order_status_updated",
		"status":   "PAID",
	}

	h.logger.Info("一次性产品购买处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("sku", notification.SKU))
}

func (h *GoogleWebhookHandler) handleOneTimeProductCanceled(ctx context.Context, event *models.WebhookEvent, notification *models.OneTimeProductNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订单")
			return
		}
		h.logger.Error("查询订单失败", zap.Error(err))
		event.MarkAsFailed("查询订单失败")
		return
	}

	if err := h.paymentService.CancelOrder(ctx, order.ID, "Google Play取消"); err != nil {
		h.logger.Error("取消订单失败", zap.Error(err))
		event.MarkAsFailed("取消订单失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "order_cancelled",
		"reason":   "Google Play取消",
	}

	h.logger.Info("一次性产品取消处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("sku", notification.SKU))
}

// ==================== 订阅事件处理 ====================

func (h *GoogleWebhookHandler) handleSubscriptionPurchased(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订阅订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订阅订单")
			return
		}
		h.logger.Error("查询订阅订单失败", zap.Error(err))
		event.MarkAsFailed("查询订阅订单失败")
		return
	}

	if err := h.paymentService.UpdateOrderStatus(ctx, order.ID, models.OrderStatusPaid); err != nil {
		h.logger.Error("更新订单状态失败", zap.Error(err))
		event.MarkAsFailed("更新订单状态失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "subscription_activated",
		"status":   "ACTIVE",
	}

	h.logger.Info("订阅购买处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

func (h *GoogleWebhookHandler) handleSubscriptionRenewed(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订阅订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订阅订单")
			return
		}
		h.logger.Error("查询订阅订单失败", zap.Error(err))
		event.MarkAsFailed("查询订阅订单失败")
		return
	}

	// 从Google获取最新的订阅信息
	subscription, err := h.googleService.VerifySubscription(ctx, notification.SubscriptionID, notification.PurchaseToken)
	if err != nil {
		h.logger.Error("验证订阅失败", zap.Error(err))
		event.MarkAsFailed("验证订阅失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id":      order.ID,
		"action":        "subscription_renewed",
		"auto_renewing": subscription.AutoRenewing,
	}

	h.logger.Info("订阅续订处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

func (h *GoogleWebhookHandler) handleSubscriptionCanceled(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订阅订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订阅订单")
			return
		}
		h.logger.Error("查询订阅订单失败", zap.Error(err))
		event.MarkAsFailed("查询订阅订单失败")
		return
	}

	if err := h.paymentService.CancelOrder(ctx, order.ID, "Google Play取消订阅"); err != nil {
		h.logger.Error("取消订阅失败", zap.Error(err))
		event.MarkAsFailed("取消订阅失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "subscription_cancelled",
		"reason":   "Google Play取消",
	}

	h.logger.Info("订阅取消处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

func (h *GoogleWebhookHandler) handleSubscriptionExpired(_ context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订阅订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订阅订单")
			return
		}
		h.logger.Error("查询订阅订单失败", zap.Error(err))
		event.MarkAsFailed("查询订阅订单失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "subscription_expired",
		"status":   "EXPIRED",
	}

	h.logger.Info("订阅过期处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

func (h *GoogleWebhookHandler) handleSubscriptionInGracePeriod(_ context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订阅订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订阅订单")
			return
		}
		h.logger.Error("查询订阅订单失败", zap.Error(err))
		event.MarkAsFailed("查询订阅订单失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "subscription_in_grace_period",
		"status":   "IN_GRACE_PERIOD",
	}

	h.logger.Info("订阅宽限期处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

func (h *GoogleWebhookHandler) handleSubscriptionRevoked(_ context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	var order models.Order
	err := h.db.Where("google_payments.purchase_token = ?", notification.PurchaseToken).
		Joins("JOIN google_payments ON orders.id = google_payments.order_id").
		First(&order).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.logger.Warn("未找到对应的订阅订单", zap.String("purchase_token", notification.PurchaseToken))
			event.MarkAsFailed("未找到对应的订阅订单")
			return
		}
		h.logger.Error("查询订阅订单失败", zap.Error(err))
		event.MarkAsFailed("查询订阅订单失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id": order.ID,
		"action":   "subscription_revoked",
		"status":   "EXPIRED",
	}

	h.logger.Info("订阅撤销处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

// ==================== 辅助函数 ====================

func (h *GoogleWebhookHandler) determineWebhookType(data *GoogleWebhookData) models.WebhookType {
	if data.TestNotification != nil {
		return models.WebhookTypeTest
	}
	if data.OneTimeProductNotification != nil {
		return models.WebhookTypeOneTimeProduct
	}
	if data.SubscriptionNotification != nil {
		return models.WebhookTypeSubscription
	}
	return models.WebhookTypeUnknown
}

func (h *GoogleWebhookHandler) parseEventTime(timeMillis string) int64 {
	if timeMillis == "" {
		return time.Now().UnixMilli()
	}

	var millis int64
	if _, err := fmt.Sscanf(timeMillis, "%d", &millis); err == nil {
		return millis
	}

	return time.Now().UnixMilli()
}

