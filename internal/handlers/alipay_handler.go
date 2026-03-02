package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/services"
)

// AlipayHandler 支付宝处理器
type AlipayHandler struct {
	alipayService           *services.AlipayService
	reconciliationService   *services.AlipayReconciliationService
	paymentService          services.PaymentService
	logger                  *zap.Logger
}

// NewAlipayHandler 创建支付宝处理器
func NewAlipayHandler(alipayService *services.AlipayService, reconciliationService *services.AlipayReconciliationService, paymentService services.PaymentService, logger *zap.Logger) *AlipayHandler {
	return &AlipayHandler{
		alipayService:         alipayService,
		reconciliationService: reconciliationService,
		paymentService:        paymentService,
		logger:                logger,
	}
}

// CreateAlipayOrderRequest 创建支付宝订单请求
type CreateAlipayOrderRequest struct {
	UserID         uint   `json:"user_id" binding:"required"`
	ProductID      string `json:"product_id" binding:"required"`
	Subject        string `json:"subject" binding:"required"`
	Body           string `json:"body"`
	TotalAmount    int64  `json:"total_amount" binding:"required,min=1"`
	AllowDuplicate bool   `json:"allow_duplicate"` // 是否允许重复下单，默认 false 时复用已有待支付订单
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
	OrderNo string `json:"order_no" binding:"required"`
	PayType string `json:"pay_type" binding:"required,oneof=WAP PAGE APP"`
}

// CreateAlipayPaymentResponse 创建支付宝支付响应
type CreateAlipayPaymentResponse struct {
	PaymentURL string `json:"payment_url"`
	OrderNo    string `json:"order_no"`
}

// QueryAlipayOrderResponse 查询支付宝订单响应
type QueryAlipayOrderResponse struct {
	OrderNo       string  `json:"order_no"`
	TradeNo       string  `json:"trade_no,omitempty"`
	TradeStatus   string  `json:"trade_status"`
	TotalAmount   int64   `json:"total_amount"`
	PaymentStatus string  `json:"payment_status"`
	PaidAt        *string `json:"paid_at,omitempty"`
}

// RefundRequest 退款请求
type RefundRequest struct {
	OrderNo       string `json:"order_no" binding:"required"`
	RefundAmount  int64  `json:"refund_amount" binding:"required,min=1"`
	RefundReason  string `json:"refund_reason" binding:"required"`
	OutRequestNo  string `json:"out_request_no"` // 可选，退款请求号，传入相同值可实现重试幂等
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
		UserID:         req.UserID,
		ProductID:      req.ProductID,
		Subject:        req.Subject,
		Body:           req.Body,
		TotalAmount:    req.TotalAmount,
		AllowDuplicate: req.AllowDuplicate,
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
	case "APP":
		paymentURL, err = h.alipayService.CreateAppPayment(c.Request.Context(), req.OrderNo)
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
		OrderNo:       req.OrderNo,
		RefundAmount:  req.RefundAmount,
		RefundReason:  req.RefundReason,
		OutRequestNo:  req.OutRequestNo,
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

// ==================== 周期扣款（订阅）API ====================

// CreateAlipaySubscription 创建支付宝周期扣款（签约）
// @Summary 创建支付宝周期扣款
// @Description 创建支付宝周期扣款协议（签约）
// @Tags 支付宝订阅
// @Accept json
// @Produce json
// @Param request body CreateAlipaySubscriptionRequest true "创建周期扣款请求"
// @Success 200 {object} Response{data=CreateAlipaySubscriptionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/subscriptions [post]
func (h *AlipayHandler) CreateAlipaySubscription(c *gin.Context) {
	var req CreateAlipaySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("创建支付宝周期扣款请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	// 转换为服务层请求
	serviceReq := &services.CreateAlipaySubscriptionRequest{
		UserID:              req.UserID,
		ProductID:           req.ProductID,
		ProductName:         req.ProductName,
		ProductDesc:         req.ProductDesc,
		PeriodType:          req.PeriodType,
		Period:              req.Period,
		ExecutionTime:       req.ExecutionTime,
		SingleAmount:        req.SingleAmount,
		TotalAmount:         req.TotalAmount,
		TotalPayments:       req.TotalPayments,
		PersonalProductCode: req.PersonalProductCode,
		SignScene:           req.SignScene,
	}

	result, err := h.alipayService.CreateSubscription(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("创建支付宝周期扣款失败", zap.Error(err))
		h.errorResponse(c, 500, "创建周期扣款失败", err)
		return
	}

	response := &CreateAlipaySubscriptionResponse{
		OrderID:       result.OrderID,
		OutRequestNo:  result.OutRequestNo,
		SignURL:       result.SignURL,
		Status:        result.Status,
		ExecutionTime: result.ExecutionTime.Format("2006-01-02 15:04:05"),
	}

	h.logger.Info("支付宝周期扣款创建成功", zap.Uint("order_id", result.OrderID))
	h.successResponse(c, response)
}

// QueryAlipaySubscription 查询支付宝周期扣款状态
// @Summary 查询支付宝周期扣款
// @Description 查询支付宝周期扣款协议状态
// @Tags 支付宝订阅
// @Accept json
// @Produce json
// @Param out_request_no query string true "商户签约号"
// @Success 200 {object} Response{data=QueryAlipaySubscriptionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/subscriptions/query [get]
func (h *AlipayHandler) QueryAlipaySubscription(c *gin.Context) {
	outRequestNo := c.Query("out_request_no")
	if outRequestNo == "" {
		h.errorResponse(c, 400, "商户签约号不能为空", nil)
		return
	}

	result, err := h.alipayService.QuerySubscription(c.Request.Context(), outRequestNo)
	if err != nil {
		h.logger.Error("查询支付宝周期扣款失败", zap.Error(err))
		h.errorResponse(c, 500, "查询周期扣款失败", err)
		return
	}

	response := &QueryAlipaySubscriptionResponse{
		OutRequestNo:        result.OutRequestNo,
		AgreementNo:         result.AgreementNo,
		ExternalAgreementNo: result.ExternalAgreementNo,
		Status:              result.Status,
		PeriodType:          result.PeriodType,
		Period:              result.Period,
		SingleAmount:        result.SingleAmount,
		TotalAmount:         result.TotalAmount,
		TotalPayments:       result.TotalPayments,
		CurrentPeriod:       result.CurrentPeriod,
		DeductSuccessCount:  result.DeductSuccessCount,
		DeductFailCount:     result.DeductFailCount,
	}

	// 格式化时间字段
	if result.SignTime != nil {
		signTime := result.SignTime.Format("2006-01-02 15:04:05")
		response.SignTime = &signTime
	}
	if result.ValidTime != nil {
		validTime := result.ValidTime.Format("2006-01-02 15:04:05")
		response.ValidTime = &validTime
	}
	if result.InvalidTime != nil {
		invalidTime := result.InvalidTime.Format("2006-01-02 15:04:05")
		response.InvalidTime = &invalidTime
	}
	if result.ExecutionTime != nil {
		executionTime := result.ExecutionTime.Format("2006-01-02 15:04:05")
		response.ExecutionTime = &executionTime
	}
	if result.LastDeductTime != nil {
		lastDeductTime := result.LastDeductTime.Format("2006-01-02 15:04:05")
		response.LastDeductTime = &lastDeductTime
	}
	if result.NextDeductTime != nil {
		nextDeductTime := result.NextDeductTime.Format("2006-01-02 15:04:05")
		response.NextDeductTime = &nextDeductTime
	}

	h.successResponse(c, response)
}

// CancelAlipaySubscription 取消支付宝周期扣款（解约）
// @Summary 取消支付宝周期扣款
// @Description 取消支付宝周期扣款协议（解约）
// @Tags 支付宝订阅
// @Accept json
// @Produce json
// @Param request body CancelAlipaySubscriptionRequest true "取消周期扣款请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/subscriptions/cancel [post]
func (h *AlipayHandler) CancelAlipaySubscription(c *gin.Context) {
	var req CancelAlipaySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("取消支付宝周期扣款请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	serviceReq := &services.CancelAlipaySubscriptionRequest{
		OutRequestNo: req.OutRequestNo,
		AgreementNo:  req.AgreementNo,
		CancelReason: req.CancelReason,
	}

	err := h.alipayService.CancelSubscription(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("取消支付宝周期扣款失败", zap.Error(err))
		h.errorResponse(c, 500, "取消周期扣款失败", err)
		return
	}

	h.logger.Info("支付宝周期扣款取消成功", zap.String("out_request_no", req.OutRequestNo))
	h.successResponse(c, gin.H{"message": "周期扣款取消成功"})
}

// ==================== 免密支付（商户代扣）API ====================

// CreateWithholdAgreement 创建免密签约
// @Summary 创建免密签约
// @Description 创建支付宝免密签约（商户代扣），用户签约后可在无需输入密码的情况下完成扣款
// @Tags 支付宝免密
// @Accept json
// @Produce json
// @Param request body CreateWithholdAgreementRequest true "创建免密签约请求"
// @Success 200 {object} Response{data=CreateWithholdAgreementResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/withhold/agreements [post]
func (h *AlipayHandler) CreateWithholdAgreement(c *gin.Context) {
	var req CreateWithholdAgreementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("创建免密签约请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	serviceReq := &services.CreateWithholdAgreementRequest{
		UserID:              req.UserID,
		PersonalProductCode: req.PersonalProductCode,
		SignScene:           req.SignScene,
	}

	result, err := h.alipayService.CreateWithholdAgreement(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("创建免密签约失败", zap.Error(err))
		h.errorResponse(c, 500, "创建免密签约失败", err)
		return
	}

	h.successResponse(c, &CreateWithholdAgreementResponse{
		OutRequestNo: result.OutRequestNo,
		SignURL:      result.SignURL,
		Status:       result.Status,
	})
}

// QueryWithholdAgreement 查询免密签约状态
// @Summary 查询免密签约状态
// @Description 查询支付宝免密签约协议状态
// @Tags 支付宝免密
// @Accept json
// @Produce json
// @Param out_request_no query string true "商户签约号"
// @Success 200 {object} Response{data=QueryWithholdAgreementResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/withhold/agreements/query [get]
func (h *AlipayHandler) QueryWithholdAgreement(c *gin.Context) {
	outRequestNo := c.Query("out_request_no")
	if outRequestNo == "" {
		h.errorResponse(c, 400, "商户签约号不能为空", nil)
		return
	}

	result, err := h.alipayService.QueryWithholdAgreement(c.Request.Context(), outRequestNo)
	if err != nil {
		h.logger.Error("查询免密签约失败", zap.Error(err))
		h.errorResponse(c, 500, "查询免密签约失败", err)
		return
	}

	h.successResponse(c, &QueryWithholdAgreementResponse{
		OutRequestNo: result.OutRequestNo,
		AgreementNo:  result.AgreementNo,
		Status:       result.Status,
		SignTime:     result.SignTime,
	})
}

// ExecuteWithhold 执行单次代扣（免密支付）
// @Summary 执行单次代扣
// @Description 对已签约用户执行免密扣款，无需用户输入密码
// @Tags 支付宝免密
// @Accept json
// @Produce json
// @Param request body ExecuteWithholdRequest true "代扣请求"
// @Success 200 {object} Response{data=ExecuteWithholdResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/withhold/execute [post]
func (h *AlipayHandler) ExecuteWithhold(c *gin.Context) {
	var req ExecuteWithholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("执行代扣请求参数错误", zap.Error(err))
		h.errorResponse(c, 400, "请求参数错误", err)
		return
	}

	serviceReq := &services.ExecuteWithholdRequest{
		UserID:      req.UserID,
		AgreementNo: req.AgreementNo,
		ProductID:   req.ProductID,
		Subject:     req.Subject,
		Body:        req.Body,
		TotalAmount: req.TotalAmount,
	}

	result, err := h.alipayService.ExecuteWithhold(c.Request.Context(), serviceReq)
	if err != nil {
		h.logger.Error("执行代扣失败", zap.Error(err))
		h.errorResponse(c, 500, "执行代扣失败", err)
		return
	}

	h.successResponse(c, &ExecuteWithholdResponse{
		OrderNo:     result.OrderNo,
		TradeNo:     result.TradeNo,
		TotalAmount: result.TotalAmount,
		TradeStatus: result.TradeStatus,
		PaidAt:      result.PaidAt,
	})
}

// ==================== 周期扣款请求和响应结构体 ====================

type CreateAlipaySubscriptionRequest struct {
	UserID              uint       `json:"user_id" binding:"required"`
	ProductID           string     `json:"product_id" binding:"required"`
	ProductName         string     `json:"product_name" binding:"required"`
	ProductDesc         string     `json:"product_desc"`
	PeriodType          string     `json:"period_type" binding:"required,oneof=DAY MONTH"`
	Period              int        `json:"period" binding:"required,min=1"`
	ExecutionTime       *time.Time `json:"execution_time"`
	SingleAmount        int64      `json:"single_amount" binding:"required,min=1"`
	TotalAmount         int64      `json:"total_amount"`
	TotalPayments       int        `json:"total_payments"`
	PersonalProductCode string     `json:"personal_product_code" binding:"required"`
	SignScene           string     `json:"sign_scene" binding:"required"`
}

type CreateAlipaySubscriptionResponse struct {
	OrderID       uint   `json:"order_id"`
	OutRequestNo  string `json:"out_request_no"`
	SignURL       string `json:"sign_url"`
	Status        string `json:"status"`
	ExecutionTime string `json:"execution_time"`
}

type QueryAlipaySubscriptionResponse struct {
	OutRequestNo        string  `json:"out_request_no"`
	AgreementNo         string  `json:"agreement_no"`
	ExternalAgreementNo string  `json:"external_agreement_no,omitempty"`
	Status              string  `json:"status"`
	SignTime            *string `json:"sign_time,omitempty"`
	ValidTime           *string `json:"valid_time,omitempty"`
	InvalidTime         *string `json:"invalid_time,omitempty"`
	PeriodType          string  `json:"period_type"`
	Period              int     `json:"period"`
	ExecutionTime       *string `json:"execution_time,omitempty"`
	SingleAmount        string  `json:"single_amount"`
	TotalAmount         string  `json:"total_amount,omitempty"`
	TotalPayments       int     `json:"total_payments"`
	CurrentPeriod       int     `json:"current_period"`
	LastDeductTime      *string `json:"last_deduct_time,omitempty"`
	NextDeductTime      *string `json:"next_deduct_time,omitempty"`
	DeductSuccessCount  int     `json:"deduct_success_count"`
	DeductFailCount     int     `json:"deduct_fail_count"`
}

type CancelAlipaySubscriptionRequest struct {
	OutRequestNo string `json:"out_request_no"`
	AgreementNo  string `json:"agreement_no"`
	CancelReason string `json:"cancel_reason" binding:"required"`
}

// 免密支付（商户代扣）请求和响应结构体

type CreateWithholdAgreementRequest struct {
	UserID              uint   `json:"user_id" binding:"required"`
	PersonalProductCode string `json:"personal_product_code"` // 默认 GENERAL_WITHHOLDING_P
	SignScene           string `json:"sign_scene"`             // 默认 DEFAULT|DEFAULT
}

type CreateWithholdAgreementResponse struct {
	OutRequestNo string `json:"out_request_no"`
	SignURL      string `json:"sign_url"`
	Status       string `json:"status"`
}

type ExecuteWithholdRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	AgreementNo string `json:"agreement_no" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	Body        string `json:"body"`
	TotalAmount int64  `json:"total_amount" binding:"required,min=1"`
}

type ExecuteWithholdResponse struct {
	OrderNo     string     `json:"order_no"`
	TradeNo     string     `json:"trade_no"`
	TotalAmount int64      `json:"total_amount"`
	TradeStatus string     `json:"trade_status"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

type QueryWithholdAgreementResponse struct {
	OutRequestNo string     `json:"out_request_no"`
	AgreementNo  string     `json:"agreement_no"`
	Status       string     `json:"status"`
	SignTime     *time.Time `json:"sign_time,omitempty"`
}

// ==================== 对账 API ====================

// RunReconciliation 执行支付宝对账
// @Summary 执行支付宝对账
// @Description 下载指定日期的对账文件并与本地订单比对
// @Tags 支付宝对账
// @Produce json
// @Param bill_date query string true "对账日期 yyyy-MM-dd"
// @Success 200 {object} Response{data=models.AlipayReconciliationReport}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alipay/reconciliation/run [post]
func (h *AlipayHandler) RunReconciliation(c *gin.Context) {
	if h.reconciliationService == nil {
		h.errorResponse(c, 503, "对账服务未配置", nil)
		return
	}
	billDate := c.Query("bill_date")
	if billDate == "" {
		h.errorResponse(c, 400, "缺少 bill_date 参数（格式：yyyy-MM-dd）", nil)
		return
	}
	report, err := h.reconciliationService.RunReconciliation(c.Request.Context(), billDate)
	if err != nil {
		h.logger.Error("执行对账失败", zap.Error(err), zap.String("bill_date", billDate))
		h.errorResponse(c, 500, "执行对账失败", err)
		return
	}
	h.successResponse(c, report)
}

// GetReconciliationReport 获取对账报告详情
// @Summary 获取对账报告详情
// @Description 获取对账报告及差异明细
// @Tags 支付宝对账
// @Produce json
// @Param id path int true "报告ID"
// @Success 200 {object} Response{data=object}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/alipay/reconciliation/reports/{id} [get]
func (h *AlipayHandler) GetReconciliationReport(c *gin.Context) {
	if h.reconciliationService == nil {
		h.errorResponse(c, 503, "对账服务未配置", nil)
		return
	}
	var idParam struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&idParam); err != nil {
		h.errorResponse(c, 400, "无效的报告ID", err)
		return
	}
	report, details, err := h.reconciliationService.GetReconciliationReport(c.Request.Context(), idParam.ID)
	if err != nil {
		h.errorResponse(c, 404, "对账报告不存在", err)
		return
	}
	h.successResponse(c, gin.H{
		"report":  report,
		"details": details,
	})
}

// ListReconciliationReports 列出对账报告
// @Summary 列出对账报告
// @Description 按日期或最近记录列出对账报告
// @Tags 支付宝对账
// @Produce json
// @Param bill_date query string false "对账日期 yyyy-MM-dd"
// @Param limit query int false "返回条数，默认20"
// @Success 200 {object} Response{data=[]models.AlipayReconciliationReport}
// @Router /api/v1/alipay/reconciliation/reports [get]
func (h *AlipayHandler) ListReconciliationReports(c *gin.Context) {
	if h.reconciliationService == nil {
		h.errorResponse(c, 503, "对账服务未配置", nil)
		return
	}
	billDate := c.Query("bill_date")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	reports, err := h.reconciliationService.ListReconciliationReports(c.Request.Context(), billDate, limit)
	if err != nil {
		h.logger.Error("列出对账报告失败", zap.Error(err))
		h.errorResponse(c, 500, "列出对账报告失败", err)
		return
	}
	h.successResponse(c, reports)
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

