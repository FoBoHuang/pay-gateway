package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/handlers"
	"pay-gateway/internal/middleware"
	"pay-gateway/internal/services"
)

// SetupRoutes 设置路由
func SetupRoutes(
	router *gin.Engine,
	paymentService services.PaymentService,
	subscriptionService services.SubscriptionService,
	googleService *services.GooglePlayService,
	alipayService *services.AlipayService,
	appleService *services.AppleService,
	db *gorm.DB,
	logger *zap.Logger,
) {
	// 创建处理器
	handler := handlers.NewHandler(paymentService, subscriptionService, googleService, logger)
	alipayHandler := handlers.NewAlipayHandler(alipayService, paymentService, logger)
	appleHandler := handlers.NewAppleHandler(appleService, paymentService, subscriptionService, logger)

	// 创建Webhook处理器
	webhookHandler := handlers.NewWebhookHandler(
		db,
		googleService,
		alipayService,
		paymentService,
		subscriptionService,
		logger,
	)
	appleWebhookHandler := handlers.NewAppleWebhookHandler(
		db,
		appleService,
		paymentService,
		subscriptionService,
		logger,
	)

	// API版本组
	v1 := router.Group("/api/v1")
	{
		// 订单相关路由
		orders := v1.Group("/orders")
		{
			orders.POST("", handler.CreateOrder)                   // 创建订单
			orders.GET("/:id", handler.GetOrder)                   // 获取订单详情
			orders.GET("/no/:order_no", handler.GetOrderByOrderNo) // 根据订单号获取订单
			orders.POST("/:id/cancel", handler.CancelOrder)        // 取消订单
		}

		// 支付相关路由
		payments := v1.Group("/payments")
		{
			payments.POST("/process", handler.ProcessPayment) // 处理支付
		}

		// 支付宝相关路由
		alipay := v1.Group("/alipay")
		{
			alipay.POST("/orders", alipayHandler.CreateAlipayOrder)     // 创建支付宝订单
			alipay.POST("/payments", alipayHandler.CreateAlipayPayment) // 创建支付宝支付
			alipay.GET("/orders/query", alipayHandler.QueryAlipayOrder) // 查询支付宝订单
			alipay.POST("/refunds", alipayHandler.AlipayRefund)         // 支付宝退款
		}

		// Apple相关路由
		apple := v1.Group("/apple")
		{
			apple.POST("/verify-receipt", appleHandler.VerifyReceipt)                                       // 验证Apple收据
			apple.POST("/verify-transaction", appleHandler.VerifyTransaction)                               // 验证Apple交易
			apple.POST("/validate-receipt", appleHandler.ValidateReceipt)                                   // 验证Apple收据（简化版本）
			apple.GET("/transactions/:original_transaction_id/history", appleHandler.GetTransactionHistory) // 获取交易历史
			apple.GET("/subscriptions/:original_transaction_id/status", appleHandler.GetSubscriptionStatus) // 获取订阅状态
		}

		// 订阅相关路由
		subscriptions := v1.Group("/subscriptions")
		{
			subscriptions.POST("", handler.CreateSubscription)               // 创建订阅
			subscriptions.GET("/:id", handler.GetSubscription)               // 获取订阅详情
			subscriptions.GET("/:id/validate", handler.ValidateSubscription) // 验证订阅
			subscriptions.POST("/:id/cancel", handler.CancelSubscription)    // 取消订阅
		}

		// 用户相关路由
		users := v1.Group("/users")
		{
			users.GET("/:user_id/orders", handler.GetUserOrders)                     // 获取用户订单
			users.GET("/:user_id/subscriptions", handler.GetUserSubscriptions)       // 获取用户订阅
			users.GET("/:user_id/subscriptions/stats", handler.GetSubscriptionStats) // 获取用户订阅统计
		}
	}

	// Webhook路由
	webhooks := router.Group("/webhook")
	{
		webhooks.POST("/google-play", webhookHandler.HandleGooglePlayWebhook) // Google Play Webhook
		webhooks.POST("/alipay", webhookHandler.HandleAlipayWebhook)          // 支付宝 Webhook
		webhooks.POST("/apple", appleWebhookHandler.HandleAppleWebhook)       // Apple Webhook
	}

	// 系统路由
	router.GET("/health", handler.HealthCheck) // 健康检查
}

// SetupMiddleware 设置中间件
func SetupMiddleware(router *gin.Engine, logger *zap.Logger) {
	// 基础中间件
	router.Use(middleware.LoggerMiddleware(logger))
	router.Use(middleware.RecoveryMiddleware(logger))
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.SecurityMiddleware())
	router.Use(middleware.TimeoutMiddleware(30 * time.Second))
	router.Use(middleware.RateLimitMiddleware())
}
