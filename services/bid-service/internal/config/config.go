package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App            AppConfig
	DB             DBConfig
	Redis          RedisConfig
	Kafka          KafkaConfig
	JWT            JWTConfig
	Lock           LockConfig
	Tracing        TracingConfig
	UserServiceURL string
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

type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

type JWTConfig struct {
	Secret string
}

type LockConfig struct {
	TTLSeconds int
}

type TracingConfig struct {
	OTLPEndpoint string // e.g. "localhost:4318"
}

var Cfg *Config

func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables: %v", err)
	}

	cfg := &Config{
		App: AppConfig{
			Port: viper.GetString("APP_PORT"),
			Env:  viper.GetString("APP_ENV"),
		},
		DB: DBConfig{
			URL: viper.GetString("DB_URL"),
		},
		Redis: RedisConfig{
			Addr:     viper.GetString("REDIS_ADDR"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		Kafka: KafkaConfig{
			Brokers: strings.Split(viper.GetString("KAFKA_BROKERS"), ","),
			Topic:   viper.GetString("KAFKA_TOPIC"),
			GroupID: viper.GetString("KAFKA_GROUP_ID"),
		},
		JWT: JWTConfig{
			Secret: viper.GetString("JWT_SECRET"),
		},
		Lock: LockConfig{
			TTLSeconds: viper.GetInt("LOCK_TTL_SECONDS"),
		},
		Tracing: TracingConfig{
			OTLPEndpoint: viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),
		},
		UserServiceURL: viper.GetString("USER_SERVICE_URL"),
	}

	Cfg = cfg
	return cfg
}
