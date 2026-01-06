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
	googleService *services.GooglePlayService,
	alipayService *services.AlipayService,
	appleService *services.AppleService,
	wechatService *services.WechatService,
	db *gorm.DB,
	logger *zap.Logger,
) {
	// ==================== 创建处理器 ====================

	// 通用处理器（订单、支付等通用接口）
	commonHandler := handlers.NewCommonHandler(paymentService, logger)

	// Google Play处理器
	googleHandler := handlers.NewGoogleHandler(googleService, paymentService, logger)
	googleWebhookHandler := handlers.NewGoogleWebhookHandler(db, googleService, paymentService, logger)

	// 支付宝处理器
	alipayHandler := handlers.NewAlipayHandler(alipayService, paymentService, logger)
	alipayWebhookHandler := handlers.NewAlipayWebhookHandler(alipayService, logger)

	// Apple处理器
	appleHandler := handlers.NewAppleHandler(appleService, paymentService, nil, logger)
	appleWebhookHandler := handlers.NewAppleWebhookHandler(db, appleService, paymentService, nil, logger)

	// 微信支付处理器
	var wechatHandler *handlers.WechatHandler
	var wechatWebhookHandler *handlers.WechatWebhookHandler
	if wechatService != nil {
		wechatHandler = handlers.NewWechatHandler(wechatService, logger)
		wechatWebhookHandler = handlers.NewWechatWebhookHandler(wechatService, logger)
	}

	// ==================== API路由 ====================

	v1 := router.Group("/api/v1")
	{
		// ---------- 通用订单路由 ----------
		orders := v1.Group("/orders")
		{
			orders.POST("", commonHandler.CreateOrder)                   // 创建订单
			orders.GET("/:id", commonHandler.GetOrder)                   // 获取订单详情
			orders.GET("/no/:order_no", commonHandler.GetOrderByOrderNo) // 根据订单号获取订单
			orders.POST("/:id/cancel", commonHandler.CancelOrder)        // 取消订单
		}

		// ---------- 用户相关路由 ----------
		users := v1.Group("/users")
		{
			users.GET("/:user_id/orders", commonHandler.GetUserOrders) // 获取用户订单
		}

		// ---------- Google Play路由 ----------
		google := v1.Group("/google")
		{
			// 购买验证
			google.POST("/verify-purchase", googleHandler.VerifyPurchase)         // 验证购买
			google.POST("/verify-subscription", googleHandler.VerifySubscription) // 验证订阅

			// 确认购买
			google.POST("/acknowledge-purchase", googleHandler.AcknowledgePurchase)         // 确认购买
			google.POST("/acknowledge-subscription", googleHandler.AcknowledgeSubscription) // 确认订阅

			// 消费购买
			google.POST("/consume-purchase", googleHandler.ConsumePurchase) // 消费购买

			// 订阅管理
			google.POST("/subscriptions", googleHandler.CreateSubscription)                 // 创建订阅订单
			google.GET("/subscriptions/status", googleHandler.GetSubscriptionStatus)        // 获取订阅状态
			google.GET("/users/:user_id/subscriptions", googleHandler.GetUserSubscriptions) // 获取用户订阅
		}

		// ---------- 支付宝路由 ----------
		alipay := v1.Group("/alipay")
		{
			// 支付
			alipay.POST("/orders", alipayHandler.CreateAlipayOrder)     // 创建支付宝订单
			alipay.POST("/payments", alipayHandler.CreateAlipayPayment) // 创建支付宝支付
			alipay.GET("/orders/query", alipayHandler.QueryAlipayOrder) // 查询支付宝订单
			alipay.POST("/refunds", alipayHandler.AlipayRefund)         // 支付宝退款

			// 周期扣款（订阅）
			alipay.POST("/subscriptions", alipayHandler.CreateAlipaySubscription)        // 创建周期扣款
			alipay.GET("/subscriptions/query", alipayHandler.QueryAlipaySubscription)    // 查询周期扣款
			alipay.POST("/subscriptions/cancel", alipayHandler.CancelAlipaySubscription) // 取消周期扣款
		}

		// ---------- Apple路由 ----------
		apple := v1.Group("/apple")
		{
			// 收据验证
			apple.POST("/verify-receipt", appleHandler.VerifyReceipt)         // 验证收据
			apple.POST("/verify-transaction", appleHandler.VerifyTransaction) // 验证交易
			apple.POST("/validate-receipt", appleHandler.ValidateReceipt)     // 验证收据（简化版本）

			// 交易历史
			apple.GET("/transactions/:original_transaction_id/history", appleHandler.GetTransactionHistory) // 获取交易历史

			// 订阅管理
			apple.GET("/subscriptions/:original_transaction_id/status", appleHandler.GetSubscriptionStatus) // 获取订阅状态
		}

		// ---------- 微信支付路由 ----------
		if wechatHandler != nil {
			wechat := v1.Group("/wechat")
			{
				// 订单管理
				wechat.POST("/orders", wechatHandler.CreateOrder)                // 创建微信订单
				wechat.GET("/orders/:order_no", wechatHandler.QueryOrder)        // 查询订单状态
				wechat.POST("/orders/:order_no/close", wechatHandler.CloseOrder) // 关闭订单

				// 支付
				wechat.POST("/payments/jsapi/:order_no", wechatHandler.CreateJSAPIPayment)   // 创建JSAPI支付
				wechat.POST("/payments/native/:order_no", wechatHandler.CreateNativePayment) // 创建Native支付
				wechat.POST("/payments/app/:order_no", wechatHandler.CreateAPPPayment)       // 创建APP支付
				wechat.POST("/payments/h5/:order_no", wechatHandler.CreateH5Payment)         // 创建H5支付

				// 退款
				wechat.POST("/refunds", wechatHandler.Refund) // 退款
			}
		}
	}

	// ==================== Webhook路由 ====================

	webhooks := router.Group("/webhook")
	{
		// Google Play Webhook
		webhooks.POST("/google", googleWebhookHandler.HandleGooglePlayWebhook)

		// 支付宝 Webhook
		webhooks.POST("/alipay/notify", alipayWebhookHandler.HandleAlipayNotify)                   // 支付通知
		webhooks.POST("/alipay/subscription", alipayWebhookHandler.HandleAlipaySubscriptionNotify) // 签约通知
		webhooks.POST("/alipay/deduct", alipayWebhookHandler.HandleAlipayDeductNotify)             // 扣款通知

		// Apple Webhook
		webhooks.POST("/apple", appleWebhookHandler.HandleAppleWebhook)

		// 微信支付 Webhook
		if wechatWebhookHandler != nil {
			webhooks.POST("/wechat/notify", wechatWebhookHandler.HandleWechatNotify)       // 支付通知
			webhooks.POST("/wechat/refund", wechatWebhookHandler.HandleWechatRefundNotify) // 退款通知
		}
	}

	// ==================== 系统路由 ====================

	router.GET("/health", commonHandler.HealthCheck) // 健康检查
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
