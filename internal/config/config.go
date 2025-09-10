package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Google   GoogleConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Port         string
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxIdleConns int
	MaxOpenConns int
}

type RedisConfig struct {
	Host         string
	Port         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

type GoogleConfig struct {
	ServiceAccountFile string
	PackageName        string
	WebhookSecret      string
}

type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
}

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
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
