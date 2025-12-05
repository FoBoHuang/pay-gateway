package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// Database 数据库连接
type Database struct {
	DB     *gorm.DB
	config *config.Config
	logger *zap.Logger
}

// NewDatabase 创建数据库连接
func NewDatabase(cfg *config.Config, logger *zap.Logger) (*Database, error) {
	// 构建DSN
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	// 配置GORM日志
	var logLevel gormlogger.LogLevel
	if cfg.Server.Mode == "debug" {
		logLevel = gormlogger.Info
	} else {
		logLevel = gormlogger.Silent
	}

	gormLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			LogLevel: logLevel,
		},
	)

	// 连接数据库
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层sql.DB对象进行连接池配置
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	logger.Info("数据库连接成功",
		zap.String("host", cfg.Database.Host),
		zap.String("port", cfg.Database.Port),
		zap.String("database", cfg.Database.DBName))

	return &Database{
		DB:     db,
		config: cfg,
		logger: logger,
	}, nil
}

// AutoMigrate 自动迁移数据库表结构
func (d *Database) AutoMigrate() error {
	d.logger.Info("开始数据库迁移")

	// 迁移所有模型
	err := d.DB.AutoMigrate(
		// 基础模型
		&models.User{},
		&models.WebhookEvent{},

		// 统一订单模型
		&models.Order{},
		&models.PaymentTransaction{},
		&models.UserBalance{},

		// 各支付方式的详情模型
		&models.GooglePayment{},
		&models.AlipayPayment{},
		&models.AlipayRefund{},
		&models.AlipaySubscription{},
		&models.ApplePayment{},
		&models.AppleRefund{},
		&models.WechatPayment{},
		&models.WechatRefund{},
	)
	if err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	d.logger.Info("数据库迁移完成")
	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// HealthCheck 健康检查
func (d *Database) HealthCheck() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Transaction 执行事务
func (d *Database) Transaction(fn func(*gorm.DB) error) error {
	return d.DB.Transaction(fn)
}

// GetDB 获取数据库连接
func (d *Database) GetDB() *gorm.DB {
	return d.DB
}
