package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App   AppConfig
	DB    DBConfig
	Redis RedisConfig
	Kafka KafkaConfig
	JWT   JWTConfig
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
	Brokers        []string
	ProducerTopic  string
	ConsumerTopics []string
	GroupID        string
}

type JWTConfig struct {
	Secret string
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
		DB: DBConfig{URL: viper.GetString("DB_URL")},
		Redis: RedisConfig{
			Addr:     viper.GetString("REDIS_ADDR"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		Kafka: KafkaConfig{
			Brokers:        strings.Split(viper.GetString("KAFKA_BROKERS"), ","),
			ProducerTopic:  viper.GetString("KAFKA_TOPIC"),
			ConsumerTopics: strings.Split(viper.GetString("KAFKA_CONSUMER_TOPICS"), ","),
			GroupID:        viper.GetString("KAFKA_GROUP_ID"),
		},
		JWT: JWTConfig{Secret: viper.GetString("JWT_SECRET")},
	}

	Cfg = cfg
	return cfg
}
