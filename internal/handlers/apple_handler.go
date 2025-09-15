package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pay-gateway/internal/services"
)

// AppleHandler Apple支付处理器
type AppleHandler struct {
	appleService        *services.AppleService
	paymentService      services.PaymentService
	subscriptionService services.SubscriptionService
	logger              *zap.Logger
}

// NewAppleHandler 创建Apple支付处理器
func NewAppleHandler(
	appleService *services.AppleService,
	paymentService services.PaymentService,
	subscriptionService services.SubscriptionService,
	logger *zap.Logger,
) *AppleHandler {
	return &AppleHandler{
		appleService:        appleService,
		paymentService:      paymentService,
		subscriptionService: subscriptionService,
		logger:              logger,
	}
}

// ApplePurchaseRequest Apple购买验证请求
type ApplePurchaseRequest struct {
	ReceiptData string `json:"receipt_data" binding:"required"`
	OrderID     uint   `json:"order_id" binding:"required"`
	IsSandbox   bool   `json:"is_sandbox"`
}

// AppleTransactionRequest Apple交易验证请求
type AppleTransactionRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	OrderID       uint   `json:"order_id" binding:"required"`
}

// AppleVerifyReceipt 验证Apple收据
// @Summary 验证Apple收据
// @Description 验证Apple应用内购买收据
// @Tags Apple
// @Accept json
// @Produce json
// @Param request body ApplePurchaseRequest true "购买验证请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/apple/verify-receipt [post]
func (h *AppleHandler) VerifyReceipt(c *gin.Context) {
	ctx := context.Background()

	var request ApplePurchaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("invalid request",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// 验证收据
	response, err := h.appleService.VerifyPurchase(ctx, request.ReceiptData, request.OrderID)
	if err != nil {
		h.logger.Error("failed to verify Apple receipt",
			zap.Error(err),
			zap.Uint("order_id", request.OrderID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to verify receipt",
		})
		return
	}

	// 保存支付信息
	if err := h.appleService.SaveApplePayment(ctx, request.OrderID, response); err != nil {
		h.logger.Error("failed to save Apple payment",
			zap.Error(err),
			zap.Uint("order_id", request.OrderID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save payment information",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Receipt verified successfully",
		"data":    response,
	})
}

// AppleVerifyTransaction 验证Apple交易
// @Summary 验证Apple交易
// @Description 使用App Store Server API验证Apple交易
// @Tags Apple
// @Accept json
// @Produce json
// @Param request body AppleTransactionRequest true "交易验证请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/apple/verify-transaction [post]
func (h *AppleHandler) VerifyTransaction(c *gin.Context) {
	ctx := context.Background()

	var request AppleTransactionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("invalid request",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// 验证交易
	response, err := h.appleService.VerifyTransaction(ctx, request.TransactionID)
	if err != nil {
		h.logger.Error("failed to verify Apple transaction",
			zap.Error(err),
			zap.String("transaction_id", request.TransactionID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to verify transaction",
		})
		return
	}

	// 保存支付信息
	if err := h.appleService.SaveApplePayment(ctx, request.OrderID, response); err != nil {
		h.logger.Error("failed to save Apple payment",
			zap.Error(err),
			zap.Uint("order_id", request.OrderID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save payment information",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaction verified successfully",
		"data":    response,
	})
}

// AppleGetTransactionHistory 获取交易历史
// @Summary 获取Apple交易历史
// @Description 获取Apple交易历史记录
// @Tags Apple
// @Accept json
// @Produce json
// @Param original_transaction_id path string true "原始交易ID"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/apple/transactions/{original_transaction_id}/history [get]
func (h *AppleHandler) GetTransactionHistory(c *gin.Context) {
	ctx := context.Background()

	originalTransactionID := c.Param("original_transaction_id")
	if originalTransactionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Original transaction ID is required",
		})
		return
	}

	// 获取交易历史
	transactions, err := h.appleService.GetTransactionHistory(ctx, originalTransactionID)
	if err != nil {
		h.logger.Error("failed to get Apple transaction history",
			zap.Error(err),
			zap.String("original_transaction_id", originalTransactionID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get transaction history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaction history retrieved successfully",
		"data":    transactions,
	})
}

// AppleGetSubscriptionStatus 获取订阅状态
// @Summary 获取Apple订阅状态
// @Description 获取Apple订阅状态信息
// @Tags Apple
// @Accept json
// @Produce json
// @Param original_transaction_id path string true "原始交易ID"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/apple/subscriptions/{original_transaction_id}/status [get]
func (h *AppleHandler) GetSubscriptionStatus(c *gin.Context) {
	ctx := context.Background()

	originalTransactionID := c.Param("original_transaction_id")
	if originalTransactionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Original transaction ID is required",
		})
		return
	}

	// 获取交易历史
	transactions, err := h.appleService.GetTransactionHistory(ctx, originalTransactionID)
	if err != nil {
		h.logger.Error("failed to get Apple subscription status",
			zap.Error(err),
			zap.String("original_transaction_id", originalTransactionID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get subscription status",
		})
		return
	}

	if len(transactions) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No subscription found",
		})
		return
	}

	// 获取最新的交易信息
	latestTransaction := transactions[len(transactions)-1]

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Subscription status retrieved successfully",
		"data":    latestTransaction,
	})
}

// AppleValidateReceipt 验证Apple收据（简化版本）
// @Summary 验证Apple收据（简化版本）
// @Description 验证Apple应用内购买收据并返回验证结果
// @Tags Apple
// @Accept json
// @Produce json
// @Param request body ApplePurchaseRequest true "购买验证请求"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/apple/validate-receipt [post]
func (h *AppleHandler) ValidateReceipt(c *gin.Context) {
	ctx := context.Background()

	var request ApplePurchaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("invalid request",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// 验证收据
	response, err := h.appleService.VerifyPurchase(ctx, request.ReceiptData, request.OrderID)
	if err != nil {
		h.logger.Error("failed to verify Apple receipt",
			zap.Error(err),
			zap.Uint("order_id", request.OrderID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to verify receipt",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Receipt validated successfully",
		"data": gin.H{
			"valid":    true,
			"verified": response,
		},
	})
}
