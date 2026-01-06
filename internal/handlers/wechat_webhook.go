package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/services"
)

// WechatWebhookHandler 微信支付Webhook处理器
type WechatWebhookHandler struct {
	wechatService *services.WechatService
	logger        *zap.Logger
}

// NewWechatWebhookHandler 创建微信支付Webhook处理器
func NewWechatWebhookHandler(
	wechatService *services.WechatService,
	logger *zap.Logger,
) *WechatWebhookHandler {
	return &WechatWebhookHandler{
		wechatService: wechatService,
		logger:        logger,
	}
}

// HandleWechatNotify 处理微信支付异步通知
// @Summary 处理微信支付异步通知
// @Description 接收微信支付异步通知，更新订单状态
// @Tags 微信支付Webhook
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /webhook/wechat/notify [post]
func (h *WechatWebhookHandler) HandleWechatNotify(c *gin.Context) {
	// 读取请求体
	var notifyData map[string]interface{}
	if err := c.ShouldBindJSON(&notifyData); err != nil {
		h.logger.Error("解析微信通知数据失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "FAIL",
			"message": "解析数据失败",
		})
		return
	}

	h.logger.Info("收到微信支付通知",
		zap.Any("out_trade_no", notifyData["out_trade_no"]),
		zap.Any("trade_state", notifyData["trade_state"]))

	// TODO: 实现签名验证逻辑
	// 验证签名（实际应用中需要验证微信签名）

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

// HandleWechatRefundNotify 处理微信退款异步通知
// @Summary 处理微信退款异步通知
// @Description 接收微信退款异步通知，更新退款状态
// @Tags 微信支付Webhook
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /webhook/wechat/refund [post]
func (h *WechatWebhookHandler) HandleWechatRefundNotify(c *gin.Context) {
	// 读取请求体
	var notifyData map[string]interface{}
	if err := c.ShouldBindJSON(&notifyData); err != nil {
		h.logger.Error("解析微信退款通知数据失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "FAIL",
			"message": "解析数据失败",
		})
		return
	}

	h.logger.Info("收到微信退款通知",
		zap.Any("out_refund_no", notifyData["out_refund_no"]),
		zap.Any("refund_status", notifyData["refund_status"]))

	// TODO: 实现退款通知处理逻辑
	// 这里需要根据实际业务需求来实现

	h.logger.Info("微信退款通知处理成功")

	// 返回成功响应给微信
	c.JSON(http.StatusOK, gin.H{
		"code":    "SUCCESS",
		"message": "成功",
	})
}

