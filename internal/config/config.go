package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// Config 应用配置总结构，包含所有模块的配置信息
type Config struct {
	Server   ServerConfig   // 服务器配置
	Database DatabaseConfig // 数据库配置
	Redis    RedisConfig    // Redis缓存配置
	Google   GoogleConfig   // Google Play配置
	JWT      JWTConfig      // JWT认证配置
	Alipay   AlipayConfig   // 支付宝配置
	Apple    AppleConfig    // Apple Store配置
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

// AppleConfig Apple Store配置
type AppleConfig struct {
	KeyID          string // Apple私钥ID，从App Store Connect获取
	IssuerID       string // Apple发行者ID，从App Store Connect获取
	BundleID       string // iOS应用Bundle ID
	PrivateKey     string // Apple私钥内容（.p8文件内容）
	PrivateKeyPath string // Apple私钥文件路径（如果私钥内容为空，则从文件读取）
	Sandbox        bool   // 是否使用沙盒环境
	WebhookSecret  string // Apple Webhook密钥，用于验证通知
}

// Load 从配置文件加载配置，支持环境变量覆盖
// 首先尝试加载config.toml文件，然后环境变量可以覆盖文件中的配置
func Load() *Config {
	config := &Config{}

	// 尝试从配置文件加载
	if err := loadFromFile(config); err != nil {
		log.Printf("配置文件加载失败: %v, 使用默认配置", err)
		config = loadDefaults()
	}

	// 环境变量可以覆盖配置文件中的设置
	config.applyEnvOverrides()

	return config
}

// loadFromFile 从config.toml文件加载配置
func loadFromFile(config *Config) error {
	// 尝试多个路径加载配置文件
	configPaths := []string{
		"configs/config.toml",          // 项目configs目录
		"config.toml",                  // 项目根目录（向后兼容）
		"../configs/config.toml",       // 上一级configs目录
		"../config.toml",               // 上一级目录
		"../../configs/config.toml",    // 上两级configs目录
		"../../config.toml",            // 上两级目录
		"/etc/pay-gateway/config.toml", // 系统配置目录
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, config); err != nil {
				return err
			}
			log.Printf("配置文件加载成功: %s", path)
			return nil
		}
	}

	return os.ErrNotExist
}

// loadDefaults 加载默认配置
func loadDefaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "8080",
			Mode:         "release",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			Host:         "localhost",
			Port:         "5432",
			User:         "postgres",
			Password:     "",
			DBName:       "billing",
			SSLMode:      "disable",
			MaxIdleConns: 10,
			MaxOpenConns: 100,
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         "6379",
			Password:     "",
			DB:           0,
			PoolSize:     10,
			MinIdleConns: 5,
		},
		Google: GoogleConfig{
			ServiceAccountFile: "service-account.json",
			PackageName:        "com.example.app",
			WebhookSecret:      "",
		},
		JWT: JWTConfig{
			Secret:     "your-secret-key",
			ExpireTime: 24 * time.Hour,
		},
		Alipay: AlipayConfig{
			AppID:          "",
			PrivateKey:     "",
			IsProduction:   false,
			NotifyURL:      "https://your-domain.com/api/alipay/notify",
			ReturnURL:      "https://your-domain.com/payment/return",
			CertMode:       false,
			AppCertPath:    "certs/appCertPublicKey.crt",
			RootCertPath:   "certs/alipayRootCert.crt",
			AlipayCertPath: "certs/alipayCertPublicKey_RSA2.crt",
		},
		Apple: AppleConfig{
			KeyID:          "",
			IssuerID:       "",
			BundleID:       "com.example.app",
			PrivateKey:     "",
			PrivateKeyPath: "",
			Sandbox:        false,
			WebhookSecret:  "",
		},
	}
}

// applyEnvOverrides 应用环境变量覆盖
func (c *Config) applyEnvOverrides() {
	// 服务器配置覆盖
	if port := os.Getenv("SERVER_PORT"); port != "" {
		c.Server.Port = port
	}
	if mode := os.Getenv("SERVER_MODE"); mode != "" {
		c.Server.Mode = mode
	}
	if readTimeout := getDuration("SERVER_READ_TIMEOUT", 0); readTimeout > 0 {
		c.Server.ReadTimeout = readTimeout
	}
	if writeTimeout := getDuration("SERVER_WRITE_TIMEOUT", 0); writeTimeout > 0 {
		c.Server.WriteTimeout = writeTimeout
	}

	// 数据库配置覆盖
	if host := os.Getenv("DB_HOST"); host != "" {
		c.Database.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		c.Database.Port = port
	}
	if user := os.Getenv("DB_USER"); user != "" {
		c.Database.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		c.Database.Password = password
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		c.Database.DBName = dbName
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		c.Database.SSLMode = sslMode
	}
	if maxIdleConns := getInt("DB_MAX_IDLE_CONNS", 0); maxIdleConns > 0 {
		c.Database.MaxIdleConns = maxIdleConns
	}
	if maxOpenConns := getInt("DB_MAX_OPEN_CONNS", 0); maxOpenConns > 0 {
		c.Database.MaxOpenConns = maxOpenConns
	}

	// Redis配置覆盖
	if host := os.Getenv("REDIS_HOST"); host != "" {
		c.Redis.Host = host
	}
	if port := os.Getenv("REDIS_PORT"); port != "" {
		c.Redis.Port = port
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		c.Redis.Password = password
	}
	if db := getInt("REDIS_DB", -1); db >= 0 {
		c.Redis.DB = db
	}
	if poolSize := getInt("REDIS_POOL_SIZE", 0); poolSize > 0 {
		c.Redis.PoolSize = poolSize
	}
	if minIdleConns := getInt("REDIS_MIN_IDLE_CONNS", 0); minIdleConns > 0 {
		c.Redis.MinIdleConns = minIdleConns
	}

	// Google配置覆盖
	if serviceAccountFile := os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE"); serviceAccountFile != "" {
		c.Google.ServiceAccountFile = serviceAccountFile
	}
	if packageName := os.Getenv("GOOGLE_PACKAGE_NAME"); packageName != "" {
		c.Google.PackageName = packageName
	}
	if webhookSecret := os.Getenv("GOOGLE_WEBHOOK_SECRET"); webhookSecret != "" {
		c.Google.WebhookSecret = webhookSecret
	}

	// JWT配置覆盖
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		c.JWT.Secret = secret
	}
	if expireTime := getDuration("JWT_EXPIRE_TIME", 0); expireTime > 0 {
		c.JWT.ExpireTime = expireTime
	}

	// 支付宝配置覆盖
	if appID := os.Getenv("ALIPAY_APP_ID"); appID != "" {
		c.Alipay.AppID = appID
	}
	if privateKey := os.Getenv("ALIPAY_PRIVATE_KEY"); privateKey != "" {
		c.Alipay.PrivateKey = privateKey
	}
	if isProduction := os.Getenv("ALIPAY_IS_PRODUCTION"); isProduction != "" {
		c.Alipay.IsProduction = isProduction == "true"
	}
	if notifyURL := os.Getenv("ALIPAY_NOTIFY_URL"); notifyURL != "" {
		c.Alipay.NotifyURL = notifyURL
	}
	if returnURL := os.Getenv("ALIPAY_RETURN_URL"); returnURL != "" {
		c.Alipay.ReturnURL = returnURL
	}
	if certMode := os.Getenv("ALIPAY_CERT_MODE"); certMode != "" {
		c.Alipay.CertMode = certMode == "true"
	}
	if appCertPath := os.Getenv("ALIPAY_APP_CERT_PATH"); appCertPath != "" {
		c.Alipay.AppCertPath = appCertPath
	}
	if rootCertPath := os.Getenv("ALIPAY_ROOT_CERT_PATH"); rootCertPath != "" {
		c.Alipay.RootCertPath = rootCertPath
	}
	if alipayCertPath := os.Getenv("ALIPAY_PUBLIC_CERT_PATH"); alipayCertPath != "" {
		c.Alipay.AlipayCertPath = alipayCertPath
	}

	// Apple配置覆盖
	if keyID := os.Getenv("APPLE_KEY_ID"); keyID != "" {
		c.Apple.KeyID = keyID
	}
	if issuerID := os.Getenv("APPLE_ISSUER_ID"); issuerID != "" {
		c.Apple.IssuerID = issuerID
	}
	if bundleID := os.Getenv("APPLE_BUNDLE_ID"); bundleID != "" {
		c.Apple.BundleID = bundleID
	}
	if privateKey := os.Getenv("APPLE_PRIVATE_KEY"); privateKey != "" {
		c.Apple.PrivateKey = privateKey
	}
	if privateKeyPath := os.Getenv("APPLE_PRIVATE_KEY_PATH"); privateKeyPath != "" {
		c.Apple.PrivateKeyPath = privateKeyPath
	}
	if sandbox := os.Getenv("APPLE_SANDBOX"); sandbox != "" {
		c.Apple.Sandbox = sandbox == "true"
	}
	if webhookSecret := os.Getenv("APPLE_WEBHOOK_SECRET"); webhookSecret != "" {
		c.Apple.WebhookSecret = webhookSecret
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
