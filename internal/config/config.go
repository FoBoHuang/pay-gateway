package config

import (
	"os"
	"strconv"
	"time"
)

// Config 应用配置总结构，包含所有模块的配置信息
type Config struct {
	Server   ServerConfig   // 服务器配置
	Database DatabaseConfig // 数据库配置
	Redis    RedisConfig    // Redis缓存配置
	Google   GoogleConfig   // Google Play配置
	JWT      JWTConfig      // JWT认证配置
	Alipay   AlipayConfig   // 支付宝配置
}

// ServerConfig 服务器配置参数
// 包含端口、运行模式和超时设置
type ServerConfig struct {
	Port         string        // 服务器监听端口，默认8080
	Mode         string        // 运行模式：debug/release，默认release
	ReadTimeout  time.Duration // 读取超时时间，默认15秒
	WriteTimeout time.Duration // 写入超时时间，默认15秒
}

// DatabaseConfig 数据库连接配置
// 使用PostgreSQL数据库
type DatabaseConfig struct {
	Host         string // 数据库主机地址，默认localhost
	Port         string // 数据库端口，默认5432
	User         string // 数据库用户名，默认postgres
	Password     string // 数据库密码
	DBName       string // 数据库名称，默认billing
	SSLMode      string // SSL模式，默认disable
	MaxIdleConns int    // 最大空闲连接数，默认10
	MaxOpenConns int    // 最大打开连接数，默认100
}

// RedisConfig Redis缓存配置
// 用于会话存储和缓存
type RedisConfig struct {
	Host         string // Redis主机地址，默认localhost
	Port         string // Redis端口，默认6379
	Password     string // Redis密码
	DB           int    // Redis数据库编号，默认0
	PoolSize     int    // 连接池大小，默认10
	MinIdleConns int    // 最小空闲连接数，默认5
}

// GoogleConfig Google Play相关配置
// 包含服务账号和应用包名等关键信息
type GoogleConfig struct {
	ServiceAccountFile string // Google服务账号JSON文件路径
	PackageName        string // Android应用包名，如com.example.app
	WebhookSecret      string // Webhook密钥，用于验证Google Play通知
}

// JWTConfig JWT认证配置
// 用于用户认证和授权
type JWTConfig struct {
	Secret     string        // JWT密钥
	ExpireTime time.Duration // Token过期时间，默认24小时
}

// AlipayConfig 支付宝配置
type AlipayConfig struct {
	AppID          string // 支付宝应用ID
	PrivateKey     string // 应用私钥
	IsProduction   bool   // 是否为生产环境
	NotifyURL      string // 异步通知URL
	ReturnURL      string // 同步返回URL
	CertMode       bool   // 是否使用证书模式
	AppCertPath    string // 应用公钥证书路径
	RootCertPath   string // 支付宝根证书路径
	AlipayCertPath string // 支付宝公钥证书路径
}

// Load 从环境变量加载配置
// 支持默认值，确保配置完整性
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Mode:         getEnv("SERVER_MODE", "release"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnv("DB_PORT", "5432"),
			User:         getEnv("DB_USER", "postgres"),
			Password:     getEnv("DB_PASSWORD", ""),
			DBName:       getEnv("DB_NAME", "billing"),
			SSLMode:      getEnv("DB_SSLMODE", "disable"),
			MaxIdleConns: getInt("DB_MAX_IDLE_CONNS", 10),
			MaxOpenConns: getInt("DB_MAX_OPEN_CONNS", 100),
		},
		Redis: RedisConfig{
			Host:         getEnv("REDIS_HOST", "localhost"),
			Port:         getEnv("REDIS_PORT", "6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getInt("REDIS_DB", 0),
			PoolSize:     getInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getInt("REDIS_MIN_IDLE_CONNS", 5),
		},
		Google: GoogleConfig{
			ServiceAccountFile: getEnv("GOOGLE_SERVICE_ACCOUNT_FILE", "service-account.json"),
			PackageName:        getEnv("GOOGLE_PACKAGE_NAME", "com.example.app"),
			WebhookSecret:      getEnv("GOOGLE_WEBHOOK_SECRET", ""),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "your-secret-key"),
			ExpireTime: getDuration("JWT_EXPIRE_TIME", 24*time.Hour),
		},
		Alipay: AlipayConfig{
			AppID:          getEnv("ALIPAY_APP_ID", ""),
			PrivateKey:     getEnv("ALIPAY_PRIVATE_KEY", ""),
			IsProduction:   getEnv("ALIPAY_IS_PRODUCTION", "false") == "true",
			NotifyURL:      getEnv("ALIPAY_NOTIFY_URL", "https://your-domain.com/api/alipay/notify"),
			ReturnURL:      getEnv("ALIPAY_RETURN_URL", "https://your-domain.com/payment/return"),
			CertMode:       getEnv("ALIPAY_CERT_MODE", "false") == "true",
			AppCertPath:    getEnv("ALIPAY_APP_CERT_PATH", "certs/appCertPublicKey.crt"),
			RootCertPath:   getEnv("ALIPAY_ROOT_CERT_PATH", "certs/alipayRootCert.crt"),
			AlipayCertPath: getEnv("ALIPAY_PUBLIC_CERT_PATH", "certs/alipayCertPublicKey_RSA2.crt"),
		},
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
// key: 环境变量名
// defaultValue: 默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getInt 获取环境变量并转换为整数，如果转换失败或不存在则返回默认值
// key: 环境变量名
// defaultValue: 默认值
func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDuration 获取环境变量并转换为时间间隔，如果转换失败或不存在则返回默认值
// key: 环境变量名
// defaultValue: 默认值
func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
