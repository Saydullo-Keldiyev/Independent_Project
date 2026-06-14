package config

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig
	JWT       JWTConfig
	Redis     RedisConfig
	Services  ServicesConfig
	RateLimit RateLimitConfig
	CORS      CORSConfig
	Proxy     ProxyConfig
}

type AppConfig struct {
	Port string
	Env  string
}

type JWTConfig struct {
	Secret string
}

type RedisConfig struct {
	Addr, Password string
	DB             int
}

type ServicesConfig struct {
	UserServiceURL         string
	AuctionServiceURL      string
	BidServiceURL          string
	NotificationServiceURL string
}

type RateLimitConfig struct {
	PerMinute int // 100 req/min per IP or user
}

type CORSConfig struct {
	AllowedOrigins []string
}

type ProxyConfig struct {
	Timeout time.Duration
	Retries int
}

var Cfg *Config

func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	perMin := viper.GetInt("RATE_LIMIT_PER_MINUTE")
	if perMin == 0 {
		perMin = viper.GetInt("RATE_LIMIT_RPS") * 60
	}
	if perMin == 0 {
		perMin = 100
	}

	timeoutSec := viper.GetInt("PROXY_TIMEOUT_SEC")
	if timeoutSec == 0 {
		timeoutSec = 30
	}

	cfg := &Config{
		App: AppConfig{
			Port: viper.GetString("APP_PORT"),
			Env:  viper.GetString("APP_ENV"),
		},
		JWT:   JWTConfig{Secret: viper.GetString("JWT_SECRET")},
		Redis: RedisConfig{
			Addr: viper.GetString("REDIS_ADDR"), Password: viper.GetString("REDIS_PASSWORD"),
			DB: viper.GetInt("REDIS_DB"),
		},
		Services: ServicesConfig{
			UserServiceURL:         viper.GetString("USER_SERVICE_URL"),
			AuctionServiceURL:      viper.GetString("AUCTION_SERVICE_URL"),
			BidServiceURL:          viper.GetString("BID_SERVICE_URL"),
			NotificationServiceURL: viper.GetString("NOTIFICATION_SERVICE_URL"),
		},
		RateLimit: RateLimitConfig{PerMinute: perMin},
		CORS: CORSConfig{
			AllowedOrigins: strings.Split(viper.GetString("CORS_ALLOWED_ORIGINS"), ","),
		},
		Proxy: ProxyConfig{
			Timeout: time.Duration(timeoutSec) * time.Second,
			Retries: viper.GetInt("PROXY_RETRIES"),
		},
	}

	if cfg.App.Port == "" {
		cfg.App.Port = "8080"
	}
	if cfg.Proxy.Retries == 0 {
		cfg.Proxy.Retries = 2
	}
	if cfg.Services.UserServiceURL == "" {
		cfg.Services.UserServiceURL = "http://user-service:8081"
	}
	if cfg.Services.AuctionServiceURL == "" {
		cfg.Services.AuctionServiceURL = "http://auction-service:8083"
	}
	if cfg.Services.BidServiceURL == "" {
		cfg.Services.BidServiceURL = "http://bid-service:8082"
	}

	Cfg = cfg
	return cfg
}

func MustLoad() *Config {
	cfg := Load()
	if cfg.JWT.Secret == "" || len(cfg.JWT.Secret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}
	return cfg
}
