package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"google-play-billing/internal/handlers"
	"google-play-billing/internal/middleware"
	"google-play-billing/internal/services"
)

// SetupRoutes 设置路由
func SetupRoutes(
	router *gin.Engine,
	paymentService services.PaymentService,
	subscriptionService services.SubscriptionService,
	googleService *services.GooglePlayService,
	logger *zap.Logger,
) {
	// 创建处理器
	handler := handlers.NewHandler(paymentService, subscriptionService, googleService, logger)

	// 注意：这里需要传入数据库连接，实际使用时应该从依赖注入容器中获取
	// 为了简化，这里暂时使用nil，实际使用时需要修改
	webhookHandler := handlers.NewWebhookHandler(
		nil, // 需要传入数据库连接
		googleService,
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
