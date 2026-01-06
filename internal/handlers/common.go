package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/models"
	"pay-gateway/internal/services"
)

// ==================== 通用响应结构体 ====================

// Response 通用响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// SuccessResponse 成功响应（用于Swagger文档）
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== 通用请求结构体 ====================

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	UserID           uint                 `json:"user_id" binding:"required"`
	ProductID        string               `json:"product_id" binding:"required"`
	Type             models.OrderType     `json:"type" binding:"required"`
	Title            string               `json:"title" binding:"required"`
	Description      string               `json:"description"`
	Quantity         int                  `json:"quantity" binding:"required,min=1"`
	Currency         string               `json:"currency" binding:"required,len=3"`
	TotalAmount      int64                `json:"total_amount" binding:"required,min=0"`
	PaymentMethod    models.PaymentMethod `json:"payment_method" binding:"required"`
	DeveloperPayload string               `json:"developer_payload"`
}

// ProcessPaymentRequest 处理支付请求
type ProcessPaymentRequest struct {
	OrderID          uint                   `json:"order_id" binding:"required"`
	Provider         models.PaymentProvider `json:"provider" binding:"required"`
	PurchaseToken    string                 `json:"purchase_token" binding:"required"`
	DeveloperPayload string                 `json:"developer_payload"`
}

// CreateSubscriptionRequest 创建订阅请求
type CreateSubscriptionRequest struct {
	UserID           uint                 `json:"user_id" binding:"required"`
	ProductID        string               `json:"product_id" binding:"required"`
	Title            string               `json:"title" binding:"required"`
	Description      string               `json:"description"`
	Currency         string               `json:"currency" binding:"required,len=3"`
	Price            int64                `json:"price" binding:"required,min=0"`
	Period           string               `json:"period" binding:"required"`
	PaymentMethod    models.PaymentMethod `json:"payment_method" binding:"required"`
	DeveloperPayload string               `json:"developer_payload"`
}

// ==================== 通用Handler ====================

// CommonHandler 通用处理器
type CommonHandler struct {
	paymentService services.PaymentService
	logger         *zap.Logger
}

// NewCommonHandler 创建通用处理器
func NewCommonHandler(paymentService services.PaymentService, logger *zap.Logger) *CommonHandler {
	return &CommonHandler{
		paymentService: paymentService,
		logger:         logger,
	}
}

// ==================== 订单通用接口 ====================

// CreateOrder 创建订单
// @Summary 创建订单
// @Description 创建新的支付订单
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param request body CreateOrderRequest true "创建订单请求"
// @Success 200 {object} Response{data=models.Order}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/orders [post]
func (h *CommonHandler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("创建订单请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	serviceReq := &services.CreateOrderRequest{
		UserID:           req.UserID,
		ProductID:        req.ProductID,
		Type:             req.Type,
		Title:            req.Title,
		Description:      req.Description,
		Quantity:         req.Quantity,
		Currency:         req.Currency,
		TotalAmount:      req.TotalAmount,
		PaymentMethod:    req.PaymentMethod,
		DeveloperPayload: req.DeveloperPayload,
	}

	order, err := h.paymentService.CreateOrder(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("创建订单失败", zap.Error(err))
		h.errorResponse(c, 500, "创建订单失败", err)
		return
	}

	h.logger.Info("订单创建成功", zap.Uint("order_id", order.ID))
	h.successResponse(c, order)
}

// GetOrder 获取订单详情
// @Summary 获取订单详情
// @Description 根据订单ID获取订单详情
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} Response{data=models.Order}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/orders/{id} [get]
func (h *CommonHandler) GetOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.errorResponse(c, 400, "无效的订单ID", err)
		return
	}

	order, err := h.paymentService.GetOrder(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("获取订单失败", zap.Error(err), zap.Uint64("order_id", id))
		h.errorResponse(c, 500, "获取订单失败", err)
		return
	}

	h.successResponse(c, order)
}

// GetOrderByOrderNo 根据订单号获取订单
// @Summary 根据订单号获取订单
// @Description 根据订单号获取订单详情
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Success 200 {object} Response{data=models.Order}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/orders/no/{order_no} [get]
func (h *CommonHandler) GetOrderByOrderNo(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		h.errorResponse(c, 400, "订单号不能为空", nil)
		return
	}

	order, err := h.paymentService.GetOrderByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("获取订单失败", zap.Error(err), zap.String("order_no", orderNo))
		h.errorResponse(c, 500, "获取订单失败", err)
		return
	}

	h.successResponse(c, order)
}

// CancelOrder 取消订单
// @Summary 取消订单
// @Description 取消指定订单
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Param reason query string false "取消原因"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/orders/{id}/cancel [post]
func (h *CommonHandler) CancelOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.errorResponse(c, 400, "无效的订单ID", err)
		return
	}

	reason := c.DefaultQuery("reason", "用户取消")

	err = h.paymentService.CancelOrder(c.Request.Context(), uint(id), reason)
	if err != nil {
		h.logger.Error("取消订单失败", zap.Error(err), zap.Uint64("order_id", id))
		h.errorResponse(c, 500, "取消订单失败", err)
		return
	}

	h.logger.Info("订单取消成功", zap.Uint64("order_id", id))
	h.successResponse(c, gin.H{"message": "订单取消成功"})
}

// GetUserOrders 获取用户订单列表
// @Summary 获取用户订单列表
// @Description 获取指定用户的订单列表
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param user_id path int true "用户ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} Response{data=gin.H}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/{user_id}/orders [get]
func (h *CommonHandler) GetUserOrders(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		h.errorResponse(c, 400, "无效的用户ID", err)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	orders, total, err := h.paymentService.GetUserOrders(c.Request.Context(), uint(userID), page, pageSize)
	if err != nil {
		h.logger.Error("获取用户订单失败", zap.Error(err), zap.Uint64("user_id", userID))
		h.errorResponse(c, 500, "获取用户订单失败", err)
		return
	}

	h.successResponse(c, gin.H{
		"orders":    orders,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ProcessPayment 处理支付
// @Summary 处理支付
// @Description 处理订单支付
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param request body ProcessPaymentRequest true "支付请求"
// @Success 200 {object} Response{data=models.PaymentTransaction}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/payments/process [post]
func (h *CommonHandler) ProcessPayment(c *gin.Context) {
	var req ProcessPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("支付请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	serviceReq := &services.ProcessPaymentRequest{
		OrderID:          req.OrderID,
		Provider:         req.Provider,
		PurchaseToken:    req.PurchaseToken,
		DeveloperPayload: req.DeveloperPayload,
	}

	transaction, err := h.paymentService.ProcessPayment(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("处理支付失败", zap.Error(err))
		h.errorResponse(c, 500, "处理支付失败", err)
		return
	}

	h.logger.Info("支付处理成功", zap.String("transaction_id", transaction.TransactionID))
	h.successResponse(c, transaction)
}

// HealthCheck 健康检查
// @Summary 健康检查
// @Description 检查服务健康状态
// @Tags 系统
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=gin.H}
// @Router /health [get]
func (h *CommonHandler) HealthCheck(c *gin.Context) {
	h.successResponse(c, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "pay-gateway",
	})
}

// ==================== 响应辅助函数 ====================

func (h *CommonHandler) successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func (h *CommonHandler) errorResponse(c *gin.Context, code int, message string, err error) {
	response := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// ==================== 通用响应辅助函数（供其他handler使用） ====================

// SuccessJSON 发送成功响应
func SuccessJSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// ErrorJSON 发送错误响应
func ErrorJSON(c *gin.Context, code int, message string, err error) {
	response := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

