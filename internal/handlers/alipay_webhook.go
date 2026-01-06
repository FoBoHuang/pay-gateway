package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/services"
)

// AlipayWebhookHandler 支付宝Webhook处理器
type AlipayWebhookHandler struct {
	alipayService *services.AlipayService
	logger        *zap.Logger
}

// NewAlipayWebhookHandler 创建支付宝Webhook处理器
func NewAlipayWebhookHandler(
	alipayService *services.AlipayService,
	logger *zap.Logger,
) *AlipayWebhookHandler {
	return &AlipayWebhookHandler{
		alipayService: alipayService,
		logger:        logger,
	}
}

// HandleAlipayNotify 处理支付宝异步通知
// @Summary 处理支付宝异步通知
// @Description 接收并处理支付宝的异步通知（支付结果通知）
// @Tags 支付宝Webhook
// @Accept application/x-www-form-urlencoded
// @Produce text/plain
// @Param notify_data formData string true "支付宝通知数据"
// @Success 200 {string} string "success"
// @Failure 400 {string} string "fail"
// @Router /webhook/alipay/notify [post]
func (h *AlipayWebhookHandler) HandleAlipayNotify(c *gin.Context) {
	// 解析表单数据
	if err := c.Request.ParseForm(); err != nil {
		h.logger.Error("解析支付宝通知表单失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	notifyData := make(map[string]string)
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			notifyData[key] = values[0]
		}
	}

	h.logger.Info("收到支付宝支付通知",
		zap.String("out_trade_no", notifyData["out_trade_no"]),
		zap.String("trade_status", notifyData["trade_status"]),
		zap.String("trade_no", notifyData["trade_no"]))

	// 处理通知
	if err := h.alipayService.HandleNotify(c.Request.Context(), notifyData); err != nil {
		h.logger.Error("处理支付宝支付通知失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	// 必须返回 success，否则支付宝会一直重试
	c.String(http.StatusOK, "success")
}

// HandleAlipaySubscriptionNotify 处理支付宝周期扣款签约通知
// @Summary 处理支付宝周期扣款签约通知
// @Description 接收并处理支付宝的周期扣款签约/解约通知
// @Tags 支付宝Webhook
// @Accept application/x-www-form-urlencoded
// @Produce text/plain
// @Param notify_data formData string true "支付宝通知数据"
// @Success 200 {string} string "success"
// @Failure 400 {string} string "fail"
// @Router /webhook/alipay/subscription [post]
func (h *AlipayWebhookHandler) HandleAlipaySubscriptionNotify(c *gin.Context) {
	// 解析表单数据
	if err := c.Request.ParseForm(); err != nil {
		h.logger.Error("解析支付宝订阅通知表单失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	notifyData := make(map[string]string)
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			notifyData[key] = values[0]
		}
	}

	h.logger.Info("收到支付宝周期扣款签约通知",
		zap.String("agreement_no", notifyData["agreement_no"]),
		zap.String("out_request_no", notifyData["out_request_no"]),
		zap.String("status", notifyData["status"]))

	// 处理订阅通知
	if err := h.alipayService.HandleSubscriptionNotify(c.Request.Context(), notifyData); err != nil {
		h.logger.Error("处理支付宝订阅通知失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// HandleAlipayDeductNotify 处理支付宝周期扣款扣款通知
// @Summary 处理支付宝周期扣款扣款通知
// @Description 接收并处理支付宝的周期扣款扣款结果通知
// @Tags 支付宝Webhook
// @Accept application/x-www-form-urlencoded
// @Produce text/plain
// @Param notify_data formData string true "支付宝通知数据"
// @Success 200 {string} string "success"
// @Failure 400 {string} string "fail"
// @Router /webhook/alipay/deduct [post]
func (h *AlipayWebhookHandler) HandleAlipayDeductNotify(c *gin.Context) {
	// 解析表单数据
	if err := c.Request.ParseForm(); err != nil {
		h.logger.Error("解析支付宝扣款通知表单失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	notifyData := make(map[string]string)
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			notifyData[key] = values[0]
		}
	}

	h.logger.Info("收到支付宝周期扣款扣款通知",
		zap.String("agreement_no", notifyData["agreement_no"]),
		zap.String("out_trade_no", notifyData["out_trade_no"]),
		zap.String("status", notifyData["status"]))

	// 处理扣款通知
	if err := h.alipayService.HandleDeductNotify(c.Request.Context(), notifyData); err != nil {
		h.logger.Error("处理支付宝扣款通知失败", zap.Error(err))
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

