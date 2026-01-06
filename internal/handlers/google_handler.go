package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/models"
	"pay-gateway/internal/services"
)

// GoogleHandler Google Play处理器
type GoogleHandler struct {
	googleService  *services.GooglePlayService
	paymentService services.PaymentService
	logger         *zap.Logger
}

// NewGoogleHandler 创建Google Play处理器
func NewGoogleHandler(
	googleService *services.GooglePlayService,
	paymentService services.PaymentService,
	logger *zap.Logger,
) *GoogleHandler {
	return &GoogleHandler{
		googleService:  googleService,
		paymentService: paymentService,
		logger:         logger,
	}
}

// ==================== 请求/响应结构体 ====================

// GoogleVerifyPurchaseRequest 验证购买请求
type GoogleVerifyPurchaseRequest struct {
	ProductID     string `json:"product_id" binding:"required"`
	PurchaseToken string `json:"purchase_token" binding:"required"`
	OrderID       uint   `json:"order_id" binding:"required"`
}

// GoogleVerifySubscriptionRequest 验证订阅请求
type GoogleVerifySubscriptionRequest struct {
	SubscriptionID string `json:"subscription_id" binding:"required"`
	PurchaseToken  string `json:"purchase_token" binding:"required"`
	OrderID        uint   `json:"order_id" binding:"required"`
}

// GoogleAcknowledgePurchaseRequest 确认购买请求
type GoogleAcknowledgePurchaseRequest struct {
	ProductID        string `json:"product_id" binding:"required"`
	PurchaseToken    string `json:"purchase_token" binding:"required"`
	DeveloperPayload string `json:"developer_payload"`
}

// GoogleAcknowledgeSubscriptionRequest 确认订阅请求
type GoogleAcknowledgeSubscriptionRequest struct {
	SubscriptionID   string `json:"subscription_id" binding:"required"`
	PurchaseToken    string `json:"purchase_token" binding:"required"`
	DeveloperPayload string `json:"developer_payload"`
}

// GoogleConsumePurchaseRequest 消费购买请求
type GoogleConsumePurchaseRequest struct {
	ProductID     string `json:"product_id" binding:"required"`
	PurchaseToken string `json:"purchase_token" binding:"required"`
}

// GoogleCreateSubscriptionRequest 创建Google订阅请求
type GoogleCreateSubscriptionRequest struct {
	UserID           uint   `json:"user_id" binding:"required"`
	ProductID        string `json:"product_id" binding:"required"`
	Title            string `json:"title" binding:"required"`
	Description      string `json:"description"`
	Currency         string `json:"currency" binding:"required,len=3"`
	Price            int64  `json:"price" binding:"required,min=0"`
	Period           string `json:"period" binding:"required"` // P1M, P1Y等
	DeveloperPayload string `json:"developer_payload"`
}

// ==================== 购买验证接口 ====================

// VerifyPurchase 验证Google Play购买
// @Summary 验证Google Play购买
// @Description 验证Google Play一次性购买
// @Tags Google Play
// @Accept json
// @Produce json
// @Param request body GoogleVerifyPurchaseRequest true "验证购买请求"
// @Success 200 {object} Response{data=services.PurchaseResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/verify-purchase [post]
func (h *GoogleHandler) VerifyPurchase(c *gin.Context) {
	var req GoogleVerifyPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("验证购买请求参数错误", zap.Error(err))
		ErrorJSON(c, 400, "请求参数错误", err)
		return
	}

	purchase, err := h.googleService.VerifyPurchase(c.Request.Context(), req.ProductID, req.PurchaseToken)
	if err != nil {
		h.logger.Error("验证Google购买失败", zap.Error(err))
		ErrorJSON(c, 500, "验证购买失败", err)
		return
	}

	h.logger.Info("Google购买验证成功",
		zap.String("product_id", req.ProductID),
		zap.String("order_id", purchase.OrderId))

	SuccessJSON(c, purchase)
}

// VerifySubscription 验证Google Play订阅
// @Summary 验证Google Play订阅
// @Description 验证Google Play订阅购买
// @Tags Google Play
// @Accept json
// @Produce json
// @Param request body GoogleVerifySubscriptionRequest true "验证订阅请求"
// @Success 200 {object} Response{data=services.SubscriptionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/verify-subscription [post]
func (h *GoogleHandler) VerifySubscription(c *gin.Context) {
	var req GoogleVerifySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("验证订阅请求参数错误", zap.Error(err))
		ErrorJSON(c, 400, "请求参数错误", err)
		return
	}

	subscription, err := h.googleService.VerifySubscription(c.Request.Context(), req.SubscriptionID, req.PurchaseToken)
	if err != nil {
		h.logger.Error("验证Google订阅失败", zap.Error(err))
		ErrorJSON(c, 500, "验证订阅失败", err)
		return
	}

	h.logger.Info("Google订阅验证成功",
		zap.String("subscription_id", req.SubscriptionID),
		zap.Bool("auto_renewing", subscription.AutoRenewing))

	SuccessJSON(c, subscription)
}

// ==================== 确认购买接口 ====================

// AcknowledgePurchase 确认Google Play购买
// @Summary 确认Google Play购买
// @Description 确认Google Play一次性购买
// @Tags Google Play
// @Accept json
// @Produce json
// @Param request body GoogleAcknowledgePurchaseRequest true "确认购买请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/acknowledge-purchase [post]
func (h *GoogleHandler) AcknowledgePurchase(c *gin.Context) {
	var req GoogleAcknowledgePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("确认购买请求参数错误", zap.Error(err))
		ErrorJSON(c, 400, "请求参数错误", err)
		return
	}

	err := h.googleService.AcknowledgePurchase(c.Request.Context(), req.ProductID, req.PurchaseToken, req.DeveloperPayload)
	if err != nil {
		h.logger.Error("确认Google购买失败", zap.Error(err))
		ErrorJSON(c, 500, "确认购买失败", err)
		return
	}

	h.logger.Info("Google购买确认成功", zap.String("product_id", req.ProductID))
	SuccessJSON(c, gin.H{"message": "购买确认成功"})
}

// AcknowledgeSubscription 确认Google Play订阅
// @Summary 确认Google Play订阅
// @Description 确认Google Play订阅购买
// @Tags Google Play
// @Accept json
// @Produce json
// @Param request body GoogleAcknowledgeSubscriptionRequest true "确认订阅请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/acknowledge-subscription [post]
func (h *GoogleHandler) AcknowledgeSubscription(c *gin.Context) {
	var req GoogleAcknowledgeSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("确认订阅请求参数错误", zap.Error(err))
		ErrorJSON(c, 400, "请求参数错误", err)
		return
	}

	err := h.googleService.AcknowledgeSubscription(c.Request.Context(), req.SubscriptionID, req.PurchaseToken, req.DeveloperPayload)
	if err != nil {
		h.logger.Error("确认Google订阅失败", zap.Error(err))
		ErrorJSON(c, 500, "确认订阅失败", err)
		return
	}

	h.logger.Info("Google订阅确认成功", zap.String("subscription_id", req.SubscriptionID))
	SuccessJSON(c, gin.H{"message": "订阅确认成功"})
}

// ==================== 消费购买接口 ====================

// ConsumePurchase 消费Google Play购买
// @Summary 消费Google Play购买
// @Description 消费Google Play一次性购买（适用于消耗型商品）
// @Tags Google Play
// @Accept json
// @Produce json
// @Param request body GoogleConsumePurchaseRequest true "消费购买请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/consume-purchase [post]
func (h *GoogleHandler) ConsumePurchase(c *gin.Context) {
	var req GoogleConsumePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("消费购买请求参数错误", zap.Error(err))
		ErrorJSON(c, 400, "请求参数错误", err)
		return
	}

	err := h.googleService.ConsumePurchase(c.Request.Context(), req.ProductID, req.PurchaseToken)
	if err != nil {
		h.logger.Error("消费Google购买失败", zap.Error(err))
		ErrorJSON(c, 500, "消费购买失败", err)
		return
	}

	h.logger.Info("Google购买消费成功", zap.String("product_id", req.ProductID))
	SuccessJSON(c, gin.H{"message": "购买消费成功"})
}

// ==================== 订阅管理接口 ====================

// CreateSubscription 创建Google订阅订单
// @Summary 创建Google订阅订单
// @Description 创建新的Google Play订阅订单
// @Tags Google Play
// @Accept json
// @Produce json
// @Param request body GoogleCreateSubscriptionRequest true "创建订阅请求"
// @Success 200 {object} Response{data=models.Order}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/subscriptions [post]
func (h *GoogleHandler) CreateSubscription(c *gin.Context) {
	var req GoogleCreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("创建订阅请求参数错误", zap.Error(err))
		ErrorJSON(c, 400, "请求参数错误", err)
		return
	}

	// 创建订阅订单
	orderReq := &services.CreateOrderRequest{
		UserID:           req.UserID,
		ProductID:        req.ProductID,
		Type:             models.OrderTypeSubscription,
		Title:            req.Title,
		Description:      req.Description,
		Quantity:         1,
		Currency:         req.Currency,
		TotalAmount:      req.Price,
		PaymentMethod:    models.PaymentMethodGooglePlay,
		DeveloperPayload: req.DeveloperPayload,
	}

	order, err := h.paymentService.CreateOrder(c.Request.Context(), orderReq)
	if err != nil {
		h.logger.Error("创建Google订阅订单失败", zap.Error(err))
		ErrorJSON(c, 500, "创建订阅订单失败", err)
		return
	}

	h.logger.Info("Google订阅订单创建成功",
		zap.Uint("order_id", order.ID),
		zap.String("product_id", req.ProductID))

	SuccessJSON(c, order)
}

// GetSubscriptionStatus 获取Google订阅状态
// @Summary 获取Google订阅状态
// @Description 获取Google Play订阅的当前状态
// @Tags Google Play
// @Accept json
// @Produce json
// @Param subscription_id query string true "订阅ID"
// @Param purchase_token query string true "购买令牌"
// @Success 200 {object} Response{data=services.SubscriptionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/subscriptions/status [get]
func (h *GoogleHandler) GetSubscriptionStatus(c *gin.Context) {
	subscriptionID := c.Query("subscription_id")
	purchaseToken := c.Query("purchase_token")

	if subscriptionID == "" || purchaseToken == "" {
		ErrorJSON(c, 400, "订阅ID和购买令牌不能为空", nil)
		return
	}

	subscription, err := h.googleService.VerifySubscription(c.Request.Context(), subscriptionID, purchaseToken)
	if err != nil {
		h.logger.Error("获取Google订阅状态失败", zap.Error(err))
		ErrorJSON(c, 500, "获取订阅状态失败", err)
		return
	}

	SuccessJSON(c, subscription)
}

// ==================== 用户订阅查询接口 ====================

// GetUserSubscriptions 获取用户Google订阅列表
// @Summary 获取用户Google订阅列表
// @Description 获取指定用户的Google Play订阅列表
// @Tags Google Play
// @Accept json
// @Produce json
// @Param user_id path int true "用户ID"
// @Param status query string false "订阅状态过滤"
// @Success 200 {object} Response{data=[]models.Order}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/google/users/{user_id}/subscriptions [get]
func (h *GoogleHandler) GetUserSubscriptions(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		ErrorJSON(c, 400, "无效的用户ID", err)
		return
	}

	orders, _, err := h.paymentService.GetUserOrders(c.Request.Context(), uint(userID), 1, 100)
	if err != nil {
		h.logger.Error("获取用户订阅失败", zap.Error(err))
		ErrorJSON(c, 500, "获取用户订阅失败", err)
		return
	}

	// 过滤出Google Play订阅订单
	var subscriptions []*models.Order
	for _, order := range orders {
		if order.Type == models.OrderTypeSubscription && order.PaymentMethod == models.PaymentMethodGooglePlay {
			subscriptions = append(subscriptions, order)
		}
	}

	SuccessJSON(c, subscriptions)
}

// ==================== 响应辅助方法 ====================

func (h *GoogleHandler) successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func (h *GoogleHandler) errorResponse(c *gin.Context, code int, message string, err error) {
	response := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

