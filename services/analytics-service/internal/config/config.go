package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig
	DB        DBConfig
	Redis     RedisConfig
	Kafka     KafkaConfig
	Cache     CacheConfig
	Retention RetentionConfig
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
	Topics  []string
	GroupID string
}

type CacheConfig struct {
	TTLSeconds int
}

type RetentionConfig struct {
	RawDays    int
	HourlyDays int
}

var Cfg *Config

func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: .env not found: %v", err)
	}

	cfg := &Config{
		App:   AppConfig{Port: viper.GetString("APP_PORT"), Env: viper.GetString("APP_ENV")},
		DB:    DBConfig{URL: viper.GetString("DB_URL")},
		Redis: RedisConfig{Addr: viper.GetString("REDIS_ADDR"), Password: viper.GetString("REDIS_PASSWORD"), DB: viper.GetInt("REDIS_DB")},
		Kafka: KafkaConfig{
			Brokers: strings.Split(viper.GetString("KAFKA_BROKERS"), ","),
			Topics:  strings.Split(viper.GetString("KAFKA_TOPICS"), ","),
			GroupID: viper.GetString("KAFKA_GROUP_ID"),
		},
		Cache:     CacheConfig{TTLSeconds: viper.GetInt("CACHE_TTL_SECONDS")},
		Retention: RetentionConfig{RawDays: viper.GetInt("RETENTION_RAW_DAYS"), HourlyDays: viper.GetInt("RETENTION_HOURLY_DAYS")},
	}

	if cfg.Cache.TTLSeconds == 0 {
		cfg.Cache.TTLSeconds = 60
	}
	if cfg.Retention.RawDays == 0 {
		cfg.Retention.RawDays = 30
	}
	if cfg.Retention.HourlyDays == 0 {
		cfg.Retention.HourlyDays = 365
	}

	Cfg = cfg
	return cfg
}
