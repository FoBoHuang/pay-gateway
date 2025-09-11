package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/services"
)

// AlipayHandler 支付宝处理器
type AlipayHandler struct {
	alipayService *services.AlipayService
	paymentService services.PaymentService
	logger        *zap.Logger
}

// NewAlipayHandler 创建支付宝处理器
func NewAlipayHandler(alipayService *services.AlipayService, paymentService services.PaymentService, logger *zap.Logger) *AlipayHandler {
	return &AlipayHandler{
		alipayService:  alipayService,
		paymentService: paymentService,
		logger:         logger,
	}
}

// CreateAlipayOrderRequest 创建支付宝订单请求
type CreateAlipayOrderRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	Body        string `json:"body"`
	TotalAmount int64  `json:"total_amount" binding:"required,min=1"`
}

// CreateAlipayOrderResponse 创建支付宝订单响应
type CreateAlipayOrderResponse struct {
	OrderID     uint   `json:"order_id"`
	OrderNo     string `json:"order_no"`
	TotalAmount int64  `json:"total_amount"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

// CreateAlipayPaymentRequest 创建支付宝支付请求
type CreateAlipayPaymentRequest struct {
	OrderNo   string `json:"order_no" binding:"required"`
	PayType   string `json:"pay_type" binding:"required,oneof=WAP PAGE"`
}

// CreateAlipayPaymentResponse 创建支付宝支付响应
type CreateAlipayPaymentResponse struct {
	PaymentURL string `json:"payment_url"`
	OrderNo    string `json:"order_no"`
}

// QueryAlipayOrderResponse 查询支付宝订单响应
type QueryAlipayOrderResponse struct {
	OrderNo       string                `json:"order_no"`
	TradeNo       string                `json:"trade_no,omitempty"`
	TradeStatus   string                `json:"trade_status"`
	TotalAmount   int64                 `json:"total_amount"`
	PaymentStatus string                `json:"payment_status"`
	PaidAt        *string               `json:"paid_at,omitempty"`
}

// RefundRequest 退款请求
type RefundRequest struct {
	OrderNo      string `json:"order_no" binding:"required"`
	RefundAmount int64  `json:"refund_amount" binding:"required,min=1"`
	RefundReason string `json:"refund_reason" binding:"required"`
}

// RefundResponse 退款响应
type RefundResponse struct {
	RefundRequestNo string `json:"refund_request_no"`
	RefundAmount    int64  `json:"refund_amount"`
	RefundStatus    string `json:"refund_status"`
	RefundAt        string `json:"refund_at,omitempty"`
}

// CreateAlipayOrder 创建支付宝订单
// @Summary 创建支付宝订单
// @Description 创建新的支付宝支付订单
// @Tags 支付宝支付
// @Accept json
// @Produce json
// @Param request body CreateAlipayOrderRequest true "创建支付宝订单请求"
// @Success 200 {object} Response{data=CreateAlipayOrderResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/orders [post]
func (h *AlipayHandler) CreateAlipayOrder(c *gin.Context) {
	var req CreateAlipayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("创建支付宝订单请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	// 转换为服务层请求
	serviceReq := &services.CreateAlipayOrderRequest{
		UserID:      req.UserID,
		ProductID:   req.ProductID,
		Subject:     req.Subject,
		Body:        req.Body,
		TotalAmount: req.TotalAmount,
	}

	result, err := h.alipayService.CreateOrder(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("创建支付宝订单失败", zap.Error(err))
		h.errorResponse(c, 500, "创建支付宝订单失败", err)
		return
	}

	response := &CreateAlipayOrderResponse{
		OrderID:     result.OrderID,
		OrderNo:     result.OrderNo,
		TotalAmount: result.TotalAmount,
		Subject:     result.Subject,
		Description: result.Description,
	}

	h.logger.Info("支付宝订单创建成功", zap.Uint("order_id", result.OrderID))
	h.successResponse(c, response)
}

// CreateAlipayPayment 创建支付宝支付
// @Summary 创建支付宝支付
// @Description 创建支付宝支付（WAP或PC网页支付）
// @Tags 支付宝支付
// @Accept json
// @Produce json
// @Param request body CreateAlipayPaymentRequest true "创建支付宝支付请求"
// @Success 200 {object} Response{data=CreateAlipayPaymentResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/payments [post]
func (h *AlipayHandler) CreateAlipayPayment(c *gin.Context) {
	var req CreateAlipayPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("创建支付宝支付请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	var paymentURL string
	var err error

	// 根据不同的支付类型创建支付
	switch req.PayType {
	case "WAP":
		paymentURL, err = h.alipayService.CreateWapPayment(c.Request.Context(), req.OrderNo)
	case "PAGE":
		paymentURL, err = h.alipayService.CreatePagePayment(c.Request.Context(), req.OrderNo)
	default:
		h.errorResponse(c, 400, "不支持的支付类型", nil)
		return
	}

	if err != nil {
		h.logger.Error("创建支付宝支付失败", zap.Error(err))
		h.errorResponse(c, 500, "创建支付宝支付失败", err)
		return
	}

	response := &CreateAlipayPaymentResponse{
		PaymentURL: paymentURL,
		OrderNo:    req.OrderNo,
	}

	h.logger.Info("支付宝支付创建成功", zap.String("order_no", req.OrderNo))
	h.successResponse(c, response)
}

// QueryAlipayOrder 查询支付宝订单
// @Summary 查询支付宝订单
// @Description 查询支付宝订单状态
// @Tags 支付宝支付
// @Accept json
// @Produce json
// @Param order_no query string true "订单号"
// @Success 200 {object} Response{data=QueryAlipayOrderResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/orders/query [get]
func (h *AlipayHandler) QueryAlipayOrder(c *gin.Context) {
	orderNo := c.Query("order_no")
	if orderNo == "" {
		h.errorResponse(c, 400, "订单号不能为空", nil)
		return
	}

	result, err := h.alipayService.QueryOrder(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("查询支付宝订单失败", zap.Error(err))
		h.errorResponse(c, 500, "查询支付宝订单失败", err)
		return
	}

	response := &QueryAlipayOrderResponse{
		OrderNo:       result.OrderNo,
		TradeNo:       result.TradeNo,
		TradeStatus:   result.TradeStatus,
		TotalAmount:   result.TotalAmount,
		PaymentStatus: string(result.PaymentStatus),
	}

	if result.PaidAt != nil {
		paidAt := result.PaidAt.Format("2006-01-02 15:04:05")
		response.PaidAt = &paidAt
	}

	h.successResponse(c, response)
}

// AlipayRefund 支付宝退款
// @Summary 支付宝退款
// @Description 对支付宝订单进行退款
// @Tags 支付宝支付
// @Accept json
// @Produce json
// @Param request body RefundRequest true "退款请求"
// @Success 200 {object} Response{data=RefundResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/refunds [post]
func (h *AlipayHandler) AlipayRefund(c *gin.Context) {
	var req RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("支付宝退款请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	serviceReq := &services.RefundRequest{
		OrderNo:      req.OrderNo,
		RefundAmount: req.RefundAmount,
		RefundReason: req.RefundReason,
	}

	result, err := h.alipayService.Refund(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("支付宝退款失败", zap.Error(err))
		h.errorResponse(c, 500, "支付宝退款失败", err)
		return
	}

	response := &RefundResponse{
		RefundRequestNo: result.RefundRequestNo,
		RefundAmount:    result.RefundAmount,
		RefundStatus:    result.RefundStatus,
	}

	if result.RefundAt != nil {
		response.RefundAt = result.RefundAt.Format("2006-01-02 15:04:05")
	}

	h.logger.Info("支付宝退款成功", zap.String("order_no", req.OrderNo))
	h.successResponse(c, response)
}

// successResponse 成功响应
func (h *AlipayHandler) successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// errorResponse 错误响应
func (h *AlipayHandler) errorResponse(c *gin.Context, code int, message string, err error) {
	response := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}
	c.JSON(http.StatusOK, response)
}