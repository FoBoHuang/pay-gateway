package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"pay-gateway/internal/cache"
	"pay-gateway/internal/config"
	"pay-gateway/internal/database"
	"pay-gateway/internal/mq"
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

	// 初始化支付宝服务（传入 Redis 用于 Webhook 分布式锁）
	alipayService, err := services.NewAlipayService(db.GetDB(), &cfg.Alipay, redis)
	if err != nil {
		logger.Fatal("初始化支付宝服务失败", zap.Error(err))
	}

	// 初始化支付宝对账服务（可选，失败时对账接口返回 503）
	var alipayReconciliationService *services.AlipayReconciliationService
	if svc, err := services.NewAlipayReconciliationService(db.GetDB(), &cfg.Alipay, logger); err != nil {
		logger.Warn("初始化支付宝对账服务失败，对账功能将不可用", zap.Error(err))
	} else {
		alipayReconciliationService = svc
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

	// 初始化 RocketMQ（订单超时自动取消）
	var mqClient *mq.Client
	var orderDelayCancelConsumer *mq.OrderDelayCancelConsumer
	if cfg.RocketMQ.Enabled {
		var err error
		mqClient, err = mq.NewClient(&cfg.RocketMQ, logger)
		if err != nil {
			logger.Warn("初始化 RocketMQ Producer 失败，延迟取消将降级为定时任务", zap.Error(err))
		} else {
			orderProducer := mq.NewOrderDelayCancelProducer(mqClient, &cfg.RocketMQ, logger)
			// 注入到各支付服务
			paymentService.SetOrderDelayCancelProducer(orderProducer)
			if alipayService != nil {
				alipayService.SetOrderDelayCancelProducer(orderProducer)
			}
			if wechatService != nil {
				wechatService.SetOrderDelayCancelProducer(orderProducer)
			}

			// 启动消费者
			orderDelayCancelConsumer, err = mq.NewOrderDelayCancelConsumer(&cfg.RocketMQ, db.GetDB(), logger)
			if err != nil {
				logger.Warn("初始化 RocketMQ 订单取消消费者失败", zap.Error(err))
			} else {
				orderDelayCancelConsumer.Start()
			}

			logger.Info("RocketMQ 订单超时取消服务已启动",
				zap.String("endpoint", cfg.RocketMQ.Endpoint),
				zap.String("topic", cfg.RocketMQ.OrderDelayTopic),
				zap.Duration("order_timeout", cfg.RocketMQ.OrderTimeout))
		}
	} else {
		logger.Info("RocketMQ 未启用，订单超时取消将使用定时任务轮询")
	}

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 创建Gin引擎
	router := gin.New()

	// 设置中间件
	routes.SetupMiddleware(router, logger)

	// 设置路由
	routes.SetupRoutes(router, paymentService, googleService, alipayService, alipayReconciliationService, appleService, wechatService, db.GetDB(), cfg, logger)

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

	// 启动订单超时取消定时任务（每分钟执行一次）
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if count, err := paymentService.CancelExpiredOrders(context.Background()); err != nil {
				logger.Error("定时取消过期订单失败", zap.Error(err))
			} else if count > 0 {
				logger.Info("定时任务已取消过期订单", zap.Int64("count", count))
			}
		}
	}()

	// 启动支付宝主动查询兜底定时任务（每2分钟执行一次，补漏 Webhook 未成功通知的订单）
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if count, err := alipayService.SyncPendingOrders(context.Background()); err != nil {
				logger.Error("支付宝主动查询兜底失败", zap.Error(err))
			} else if count > 0 {
				logger.Info("支付宝主动查询兜底完成", zap.Int("queried", count))
			}
		}
	}()

	// 启动支付宝每日对账定时任务（可选）
	if alipayReconciliationService != nil && cfg.Alipay.ReconciliationCronEnable {
		cronTime := cfg.Alipay.ReconciliationCronTime
		if cronTime == "" {
			cronTime = "02:00"
		}
		go runReconciliationCron(alipayReconciliationService, cronTime, logger)
		logger.Info("已启用支付宝每日对账定时任务", zap.String("cron_time", cronTime))
	}

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

	// 关闭 RocketMQ
	if orderDelayCancelConsumer != nil {
		if err := orderDelayCancelConsumer.Stop(); err != nil {
			logger.Error("关闭 RocketMQ 消费者失败", zap.Error(err))
		}
	}
	if mqClient != nil {
		if err := mqClient.Close(); err != nil {
			logger.Error("关闭 RocketMQ Producer 失败", zap.Error(err))
		}
	}

	logger.Info("服务器已关闭")
}

// runReconciliationCron 每日对账定时任务，在指定时间执行前一日对账
func runReconciliationCron(svc *services.AlipayReconciliationService, cronTime string, logger *zap.Logger) {
	parts := strings.Split(cronTime, ":")
	hour, min := 2, 0
	if len(parts) >= 1 && parts[0] != "" {
		if h, err := strconv.Atoi(parts[0]); err == nil && h >= 0 && h <= 23 {
			hour = h
		}
	}
	if len(parts) >= 2 && parts[1] != "" {
		if m, err := strconv.Atoi(parts[1]); err == nil && m >= 0 && m <= 59 {
			min = m
		}
	}
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		sleep := time.Until(next)
		logger.Info("对账定时任务将于下次执行", zap.Time("next_run", next), zap.Duration("sleep", sleep))
		time.Sleep(sleep)
		billDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		if _, err := svc.RunReconciliation(context.Background(), billDate); err != nil {
			logger.Error("定时对账执行失败", zap.String("bill_date", billDate), zap.Error(err))
		} else {
			logger.Info("定时对账执行完成", zap.String("bill_date", billDate))
		}
	}
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
