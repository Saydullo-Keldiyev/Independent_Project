package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App     AppConfig
	DB      DBConfig
	Redis   RedisConfig
	JWT     JWTConfig
	Kafka   KafkaConfig
	Tracing TracingConfig
	Security SecurityConfig
}

type AppConfig struct {
	Port string
	Env  string
}

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  int // minutes — default 15
	RefreshTokenTTL int // days — default 7
}

type KafkaConfig struct {
	Brokers      []string
	Topic        string
	AuctionTopic string
}

type TracingConfig struct {
	OTLPEndpoint string
}

type SecurityConfig struct {
	BcryptCost  int
	MaxSessions int
}

var Cfg *Config

func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	cfg := &Config{
		App: AppConfig{
			Port: viper.GetString("APP_PORT"),
			Env:  viper.GetString("APP_ENV"),
		},
		DB: DBConfig{URL: viper.GetString("DB_URL")},
		Redis: RedisConfig{
			Addr:     viper.GetString("REDIS_ADDR"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:          viper.GetString("JWT_SECRET"),
			AccessTokenTTL:  viper.GetInt("JWT_ACCESS_TTL_MINUTES"),
			RefreshTokenTTL: viper.GetInt("JWT_REFRESH_TTL_DAYS"),
		},
		Kafka: KafkaConfig{
			Brokers:      strings.Split(viper.GetString("KAFKA_BROKERS"), ","),
			Topic:        viper.GetString("KAFKA_TOPIC"),
			AuctionTopic: viper.GetString("KAFKA_AUCTION_TOPIC"),
		},
		Tracing: TracingConfig{
			OTLPEndpoint: viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),
		},
		Security: SecurityConfig{
			BcryptCost:  viper.GetInt("BCRYPT_COST"),
			MaxSessions: viper.GetInt("MAX_ACTIVE_SESSIONS"),
		},
	}

	if cfg.App.Port == "" {
		cfg.App.Port = "8081"
	}
	if cfg.JWT.AccessTokenTTL == 0 {
		cfg.JWT.AccessTokenTTL = 15
	}
	if cfg.JWT.RefreshTokenTTL == 0 {
		cfg.JWT.RefreshTokenTTL = 7
	}
	if cfg.Security.BcryptCost == 0 {
		cfg.Security.BcryptCost = 14
	}
	if cfg.Security.MaxSessions == 0 {
		cfg.Security.MaxSessions = 5
	}
	if cfg.Kafka.Topic == "" {
		cfg.Kafka.Topic = "user-events"
	}

	Cfg = cfg
	return cfg
}

func MustLoad() *Config {
	cfg := Load()
	if cfg.JWT.Secret == "" || len(cfg.JWT.Secret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}
	if cfg.DB.URL == "" {
		log.Fatal("DB_URL is required")
	}
	return cfg
}
