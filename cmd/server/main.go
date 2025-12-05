package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"pay-gateway/internal/cache"
	"pay-gateway/internal/config"
	"pay-gateway/internal/database"
	"pay-gateway/internal/routes"
	"pay-gateway/internal/services"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志
	logger, err := initLogger(cfg.Server.Mode)
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()

	logger.Info("启动支付网关服务",
		zap.String("mode", cfg.Server.Mode),
		zap.String("port", cfg.Server.Port))

	// 初始化数据库
	db, err := database.NewDatabase(cfg, logger)
	if err != nil {
		logger.Fatal("初始化数据库失败", zap.Error(err))
	}
	defer db.Close()

	// 自动迁移数据库
	if err := db.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}

	// 初始化Redis
	redis, err := cache.NewRedis(cfg, logger)
	if err != nil {
		logger.Fatal("初始化Redis失败", zap.Error(err))
	}
	defer redis.Close()

	// 初始化Google Play服务
	googleService, err := services.NewGooglePlayService(cfg, logger)
	if err != nil {
		logger.Fatal("初始化Google Play服务失败", zap.Error(err))
	}

	// 初始化支付宝服务
	alipayService, err := services.NewAlipayService(db.GetDB(), &cfg.Alipay)
	if err != nil {
		logger.Fatal("初始化支付宝服务失败", zap.Error(err))
	}

	// 初始化Apple服务
	appleService, err := services.NewAppleService(cfg, logger, db.GetDB())
	if err != nil {
		logger.Fatal("初始化Apple服务失败", zap.Error(err))
	}

	// 初始化微信支付服务
	wechatService, err := services.NewWechatService(db.GetDB(), &cfg.Wechat, logger)
	if err != nil {
		logger.Warn("初始化微信支付服务失败，微信支付功能将不可用", zap.Error(err))
		// 微信服务初始化失败不影响其他服务
		wechatService = nil
	}

	// 初始化支付服务
	paymentService := services.NewPaymentService(db.GetDB(), cfg, logger, googleService, alipayService, appleService)

	// 初始化订阅服务
	subscriptionService := services.NewSubscriptionService(db.GetDB(), cfg, logger, googleService, paymentService)

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 创建Gin引擎
	router := gin.New()

	// 设置中间件
	routes.SetupMiddleware(router, logger)

	// 设置路由
	routes.SetupRoutes(router, paymentService, subscriptionService, googleService, alipayService, appleService, wechatService, db.GetDB(), logger)

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 启动服务器
	go func() {
		logger.Info("HTTP服务器启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP服务器启动失败", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("服务器强制关闭", zap.Error(err))
	}

	logger.Info("服务器已关闭")
}

// initLogger 初始化日志
func initLogger(mode string) (*zap.Logger, error) {
	var config zap.Config

	if mode == "debug" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	// 设置日志级别
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	// 设置日志格式
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder

	return config.Build()
}
