package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/services"
)

// WechatHandler 微信支付处理器
type WechatHandler struct {
	wechatService *services.WechatService
	logger        *zap.Logger
}

// NewWechatHandler 创建微信支付处理器
func NewWechatHandler(wechatService *services.WechatService, logger *zap.Logger) *WechatHandler {
	return &WechatHandler{
		wechatService: wechatService,
		logger:        logger,
	}
}

// CreateOrder 创建微信支付订单
// @Summary 创建微信支付订单
// @Description 创建微信支付订单，支持JSAPI、NATIVE、APP、MWEB等支付方式
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param request body services.CreateWechatOrderRequest true "订单信息"
// @Success 200 {object} services.CreateWechatOrderResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/orders [post]
func (h *WechatHandler) CreateOrder(c *gin.Context) {
	var req services.CreateWechatOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("参数验证失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数验证失败: " + err.Error()})
		return
	}

	resp, err := h.wechatService.CreateOrder(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("创建微信订单失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建订单失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// CreateJSAPIPayment 创建JSAPI支付
// @Summary 创建JSAPI支付（小程序、公众号）
// @Description 创建JSAPI支付，返回调起支付所需参数
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Param request body JSAPIPaymentRequest true "支付信息"
// @Success 200 {object} services.JSAPIPaymentResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/payments/jsapi/{order_no} [post]
func (h *WechatHandler) CreateJSAPIPayment(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号不能为空"})
		return
	}

	var req struct {
		OpenID string `json:"openid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("参数验证失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数验证失败: " + err.Error()})
		return
	}

	resp, err := h.wechatService.CreateJSAPIPayment(c.Request.Context(), orderNo, req.OpenID)
	if err != nil {
		h.logger.Error("创建JSAPI支付失败", zap.Error(err), zap.String("order_no", orderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// CreateNativePayment 创建Native支付
// @Summary 创建Native支付（扫码支付）
// @Description 创建Native支付，返回二维码链接
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Success 200 {object} services.NativePaymentResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/payments/native/{order_no} [post]
func (h *WechatHandler) CreateNativePayment(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号不能为空"})
		return
	}

	resp, err := h.wechatService.CreateNativePayment(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("创建Native支付失败", zap.Error(err), zap.String("order_no", orderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// CreateAPPPayment 创建APP支付
// @Summary 创建APP支付
// @Description 创建APP支付，返回调起支付所需参数
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Success 200 {object} services.APPPaymentResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/payments/app/{order_no} [post]
func (h *WechatHandler) CreateAPPPayment(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号不能为空"})
		return
	}

	resp, err := h.wechatService.CreateAPPPayment(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("创建APP支付失败", zap.Error(err), zap.String("order_no", orderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// CreateH5Payment 创建H5支付
// @Summary 创建H5支付
// @Description 创建H5支付，返回支付链接
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Param request body H5PaymentRequest true "H5支付场景信息"
// @Success 200 {object} services.H5PaymentResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/payments/h5/{order_no} [post]
func (h *WechatHandler) CreateH5Payment(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号不能为空"})
		return
	}

	var req struct {
		SceneInfo map[string]interface{} `json:"scene_info"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 场景信息可选，不强制要求
		req.SceneInfo = nil
	}

	resp, err := h.wechatService.CreateH5Payment(c.Request.Context(), orderNo, req.SceneInfo)
	if err != nil {
		h.logger.Error("创建H5支付失败", zap.Error(err), zap.String("order_no", orderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// QueryOrder 查询订单状态
// @Summary 查询微信订单状态
// @Description 查询微信支付订单状态
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Success 200 {object} services.QueryWechatOrderResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/orders/{order_no} [get]
func (h *WechatHandler) QueryOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号不能为空"})
		return
	}

	resp, err := h.wechatService.QueryOrder(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("查询订单失败", zap.Error(err), zap.String("order_no", orderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询订单失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// Refund 退款
// @Summary 微信支付退款
// @Description 发起微信支付退款
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param request body services.WechatRefundRequest true "退款信息"
// @Success 200 {object} services.WechatRefundResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/refunds [post]
func (h *WechatHandler) Refund(c *gin.Context) {
	var req services.WechatRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("参数验证失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数验证失败: " + err.Error()})
		return
	}

	resp, err := h.wechatService.Refund(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("退款失败", zap.Error(err), zap.String("order_no", req.OrderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "退款失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// CloseOrder 关闭订单
// @Summary 关闭微信支付订单
// @Description 关闭未支付的微信订单
// @Tags 微信支付
// @Accept json
// @Produce json
// @Param order_no path string true "订单号"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/wechat/orders/{order_no}/close [post]
func (h *WechatHandler) CloseOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号不能为空"})
		return
	}

	err := h.wechatService.CloseOrder(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("关闭订单失败", zap.Error(err), zap.String("order_no", orderNo))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关闭订单失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单关闭成功",
	})
}

// HandleNotify 处理微信支付异步通知
// @Summary 处理微信支付异步通知
// @Description 接收微信支付异步通知，更新订单状态
// @Tags 微信支付
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /webhook/wechat/notify [post]
func (h *WechatHandler) HandleNotify(c *gin.Context) {
	// 读取请求体
	var notifyData map[string]interface{}
	if err := c.ShouldBindJSON(&notifyData); err != nil {
		h.logger.Error("解析通知数据失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "FAIL",
			"message": "解析数据失败",
		})
		return
	}

	// 验证签名（实际应用中需要验证微信签名）
	// TODO: 实现签名验证逻辑

	// 处理通知
	err := h.wechatService.HandleNotify(c.Request.Context(), notifyData)
	if err != nil {
		h.logger.Error("处理微信通知失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "FAIL",
			"message": "处理失败",
		})
		return
	}

	h.logger.Info("微信支付通知处理成功")

	// 返回成功响应给微信
	c.JSON(http.StatusOK, gin.H{
		"code":    "SUCCESS",
		"message": "成功",
	})
}

// 请求结构体定义

type JSAPIPaymentRequest struct {
	OpenID string `json:"openid" binding:"required"`
}

type H5PaymentRequest struct {
	SceneInfo map[string]interface{} `json:"scene_info"`
}

