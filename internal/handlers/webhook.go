package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/models"
	"pay-gateway/internal/services"
)

// WebhookHandler Webhook处理器
type WebhookHandler struct {
	db                  *gorm.DB
	googleService       *services.GooglePlayService
	alipayService       *services.AlipayService
	paymentService      services.PaymentService
	subscriptionService services.SubscriptionService
	logger              *zap.Logger
}

// NewWebhookHandler 创建Webhook处理器
func NewWebhookHandler(
	db *gorm.DB,
	googleService *services.GooglePlayService,
	alipayService *services.AlipayService,
	paymentService services.PaymentService,
	subscriptionService services.SubscriptionService,
	logger *zap.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		db:                  db,
		googleService:       googleService,
		alipayService:       alipayService,
		paymentService:      paymentService,
		subscriptionService: subscriptionService,
		logger:              logger,
	}
}

// GooglePlayWebhookRequest Google Play Webhook请求结构
type GooglePlayWebhookRequest struct {
	Message struct {
		Data        string `json:"data"`
		MessageID   string `json:"messageId"`
		PublishTime string `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// WebhookData Webhook数据
type WebhookData struct {
	Version                    string                      `json:"version"`
	PackageName                string                      `json:"packageName"`
	EventTimeMillis            string                      `json:"eventTimeMillis"`
	OneTimeProductNotification *OneTimeProductNotification `json:"oneTimeProductNotification,omitempty"`
	SubscriptionNotification   *SubscriptionNotification   `json:"subscriptionNotification,omitempty"`
	TestNotification           *TestNotification           `json:"testNotification,omitempty"`
}

// OneTimeProductNotification 一次性产品通知
type OneTimeProductNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notificationType"`
	PurchaseToken    string `json:"purchaseToken"`
	SKU              string `json:"sku"`
}

// SubscriptionNotification 订阅通知
type SubscriptionNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notificationType"`
	PurchaseToken    string `json:"purchaseToken"`
	SubscriptionID   string `json:"subscriptionId"`
}

// TestNotification 测试通知
type TestNotification struct {
	Version string `json:"version"`
}

// HandleGooglePlayWebhook 处理Google Play Webhook
// @Summary 处理Google Play Webhook
// @Description 接收并处理Google Play的Webhook通知
// @Tags Webhook
// @Accept json
// @Produce json
// @Param request body GooglePlayWebhookRequest true "Webhook请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /webhook/google-play [post]
func (h *WebhookHandler) HandleGooglePlayWebhook(c *gin.Context) {
	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("读取Webhook请求体失败", zap.Error(err))
		h.errorResponse(c, 400, "读取请求体失败", err)
		return
	}

	// 解析Webhook请求
	var webhookReq GooglePlayWebhookRequest
	if err := json.Unmarshal(body, &webhookReq); err != nil {
		h.logger.Error("解析Webhook请求失败", zap.Error(err))
		h.errorResponse(c, 400, "解析请求失败", err)
		return
	}

	// 解码数据
	decodedData, err := base64.StdEncoding.DecodeString(webhookReq.Message.Data)
	if err != nil {
		h.logger.Error("解码Webhook数据失败", zap.Error(err))
		h.errorResponse(c, 400, "解码数据失败", err)
		return
	}

	// 解析Webhook数据
	var webhookData WebhookData
	if err := json.Unmarshal(decodedData, &webhookData); err != nil {
		h.logger.Error("解析Webhook数据失败", zap.Error(err))
		h.errorResponse(c, 400, "解析数据失败", err)
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
		h.logger.Error("保存Webhook事件失败", zap.Error(err))
		h.errorResponse(c, 500, "保存事件失败", err)
		return
	}

	// 异步处理Webhook事件
	go h.processWebhookEvent(context.Background(), webhookEvent)

	h.logger.Info("Webhook事件接收成功",
		zap.String("event_id", webhookEvent.EventID),
		zap.String("type", string(webhookEvent.Type)))

	h.successResponse(c, gin.H{
		"message":  "Webhook received successfully",
		"event_id": webhookEvent.EventID,
	})
}

// processWebhookEvent 处理Webhook事件
func (h *WebhookHandler) processWebhookEvent(ctx context.Context, event *models.WebhookEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("处理Webhook事件时发生panic", zap.Any("panic", r))
			event.MarkAsFailed(fmt.Sprintf("Panic: %v", r))
			h.db.Save(event)
		}
	}()

	h.logger.Info("开始处理Webhook事件",
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
		h.logger.Warn("未知的Webhook事件类型", zap.String("event_id", event.EventID))
		event.MarkAsFailed("未知的事件类型")
		h.db.Save(event)
		return
	}

	// 标记为已处理
	event.MarkAsProcessed()
	if err := h.db.Save(event).Error; err != nil {
		h.logger.Error("更新Webhook事件状态失败", zap.Error(err))
	}

	h.logger.Info("Webhook事件处理完成",
		zap.String("event_id", event.EventID),
		zap.String("status", string(event.Status)))
}

// processTestEvent 处理测试事件
func (h *WebhookHandler) processTestEvent(ctx context.Context, event *models.WebhookEvent) {
	h.logger.Info("处理测试事件", zap.String("event_id", event.EventID))

	// 测试事件不需要特殊处理，直接标记为已处理
	event.ProcessedData = models.JSON{
		"message": "Test notification received",
		"version": event.TestNotification.Version,
	}
}

// processOneTimeProductEvent 处理一次性产品事件
func (h *WebhookHandler) processOneTimeProductEvent(ctx context.Context, event *models.WebhookEvent) {
	notification := event.OneTimeProductNotification
	h.logger.Info("处理一次性产品事件",
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
func (h *WebhookHandler) processSubscriptionEvent(ctx context.Context, event *models.WebhookEvent) {
	notification := event.SubscriptionNotification
	h.logger.Info("处理订阅事件",
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

// handleOneTimeProductPurchased 处理一次性产品购买
func (h *WebhookHandler) handleOneTimeProductPurchased(ctx context.Context, event *models.WebhookEvent, notification *models.OneTimeProductNotification) {
	// 查找对应的订单
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

	// 更新订单状态为已支付
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

// handleOneTimeProductCanceled 处理一次性产品取消
func (h *WebhookHandler) handleOneTimeProductCanceled(ctx context.Context, event *models.WebhookEvent, notification *models.OneTimeProductNotification) {
	// 查找对应的订单
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

	// 取消订单
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

// handleSubscriptionPurchased 处理订阅购买
func (h *WebhookHandler) handleSubscriptionPurchased(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	// 查找对应的订单
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

	// 更新订阅状态为活跃
	if err := h.subscriptionService.UpdateSubscriptionStatus(ctx, order.ID, models.SubscriptionStateActive, "Google Play购买确认"); err != nil {
		h.logger.Error("更新订阅状态失败", zap.Error(err))
		event.MarkAsFailed("更新订阅状态失败")
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

// handleSubscriptionRenewed 处理订阅续订
func (h *WebhookHandler) handleSubscriptionRenewed(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	// 查找对应的订单
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

	// 更新订阅到期时间
	expiryTimeMillis := h.parseEventTime(subscription.ExpiryTimeMillis)
	expiryTime := time.Unix(expiryTimeMillis/1000, (expiryTimeMillis%1000)*1000000)

	if err := h.subscriptionService.RenewSubscription(ctx, order.ID, expiryTime); err != nil {
		h.logger.Error("续订订阅失败", zap.Error(err))
		event.MarkAsFailed("续订订阅失败")
		return
	}

	event.ProcessedData = models.JSON{
		"order_id":      order.ID,
		"action":        "subscription_renewed",
		"expiry_time":   expiryTime,
		"auto_renewing": subscription.AutoRenewing,
	}

	h.logger.Info("订阅续订处理完成",
		zap.Uint("order_id", order.ID),
		zap.String("subscription_id", notification.SubscriptionID))
}

// handleSubscriptionCanceled 处理订阅取消
func (h *WebhookHandler) handleSubscriptionCanceled(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	// 查找对应的订单
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

	// 取消订阅
	if err := h.subscriptionService.CancelSubscription(ctx, order.ID, "Google Play取消"); err != nil {
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

// handleSubscriptionExpired 处理订阅过期
func (h *WebhookHandler) handleSubscriptionExpired(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	// 查找对应的订单
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

	// 更新订阅状态为过期
	if err := h.subscriptionService.UpdateSubscriptionStatus(ctx, order.ID, models.SubscriptionStateExpired, "Google Play过期通知"); err != nil {
		h.logger.Error("更新订阅状态失败", zap.Error(err))
		event.MarkAsFailed("更新订阅状态失败")
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

// handleSubscriptionInGracePeriod 处理订阅宽限期
func (h *WebhookHandler) handleSubscriptionInGracePeriod(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	// 查找对应的订单
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

	// 更新订阅状态为宽限期
	if err := h.subscriptionService.UpdateSubscriptionStatus(ctx, order.ID, models.SubscriptionStateInGracePeriod, "Google Play宽限期通知"); err != nil {
		h.logger.Error("更新订阅状态失败", zap.Error(err))
		event.MarkAsFailed("更新订阅状态失败")
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

// handleSubscriptionRevoked 处理订阅撤销
func (h *WebhookHandler) handleSubscriptionRevoked(ctx context.Context, event *models.WebhookEvent, notification *models.SubscriptionNotification) {
	// 查找对应的订单
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

	// 更新订阅状态为过期（撤销等同于过期）
	if err := h.subscriptionService.UpdateSubscriptionStatus(ctx, order.ID, models.SubscriptionStateExpired, "Google Play撤销通知"); err != nil {
		h.logger.Error("更新订阅状态失败", zap.Error(err))
		event.MarkAsFailed("更新订阅状态失败")
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

// determineWebhookType 确定Webhook类型
func (h *WebhookHandler) determineWebhookType(data *WebhookData) models.WebhookType {
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

// parseEventTime 解析事件时间
func (h *WebhookHandler) parseEventTime(timeMillis string) int64 {
	if timeMillis == "" {
		return time.Now().UnixMilli()
	}

	// 尝试解析毫秒时间戳
	var millis int64
	if _, err := fmt.Sscanf(timeMillis, "%d", &millis); err == nil {
		return millis
	}

	// 如果解析失败，返回当前时间
	return time.Now().UnixMilli()
}

// 错误响应
func (h *WebhookHandler) errorResponse(c *gin.Context, code int, message string, err error) {
	response := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// 成功响应
func (h *WebhookHandler) successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// HandleAlipayWebhook 处理支付宝Webhook
// @Summary 处理支付宝Webhook
// @Description 接收并处理支付宝的异步通知
// @Tags Webhook
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param notify_data formData string true "支付宝通知数据"
// @Success 200 {string} string "success"
// @Failure 400 {string} string "fail"
// @Router /webhook/alipay [post]
func (h *WebhookHandler) HandleAlipayWebhook(c *gin.Context) {
	// 解析表单数据
	notifyData := make(map[string]string)
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			notifyData[key] = values[0]
		}
	}

	h.logger.Info("收到支付宝Webhook通知",
		zap.String("out_trade_no", notifyData["out_trade_no"]),
		zap.String("trade_status", notifyData["trade_status"]))

	// 处理通知
	if err := h.alipayService.HandleNotify(c.Request.Context(), notifyData); err != nil {
		h.logger.Error("处理支付宝Webhook失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	// 必须返回 success，否则支付宝会一直重试
	c.String(http.StatusOK, "success")
}
