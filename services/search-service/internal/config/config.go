package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App     AppConfig
	Elastic ElasticConfig
	Redis   RedisConfig
	Kafka   KafkaConfig
	Cache   CacheConfig
}

type AppConfig struct {
	Port string
	Env  string
}

type ElasticConfig struct {
	URL   string
	Index string
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
	TTLSeconds         int
	TrendingWindowHours int
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
		App: AppConfig{
			Port: viper.GetString("APP_PORT"),
			Env:  viper.GetString("APP_ENV"),
		},
		Elastic: ElasticConfig{
			URL:   viper.GetString("ELASTICSEARCH_URL"),
			Index: viper.GetString("ELASTICSEARCH_INDEX"),
		},
		Redis: RedisConfig{
			Addr:     viper.GetString("REDIS_ADDR"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		Kafka: KafkaConfig{
			Brokers: strings.Split(viper.GetString("KAFKA_BROKERS"), ","),
			Topics:  strings.Split(viper.GetString("KAFKA_TOPICS"), ","),
			GroupID: viper.GetString("KAFKA_GROUP_ID"),
		},
		Cache: CacheConfig{
			TTLSeconds:          viper.GetInt("CACHE_TTL_SECONDS"),
			TrendingWindowHours: viper.GetInt("TRENDING_WINDOW_HOURS"),
		},
	}

	if cfg.Cache.TTLSeconds == 0 {
		cfg.Cache.TTLSeconds = 300
	}
	if cfg.Cache.TrendingWindowHours == 0 {
		cfg.Cache.TrendingWindowHours = 24
	}
	if cfg.Elastic.Index == "" {
		cfg.Elastic.Index = "auctions"
	}

	Cfg = cfg
	return cfg
}
