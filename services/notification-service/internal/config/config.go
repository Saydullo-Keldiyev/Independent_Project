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
	Email EmailConfig
	SMTP  SMTPConfig
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

type EmailConfig struct {
	FromAddress string
	FromName    string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

var Cfg *Config

func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: .env not found, using env vars: %v", err)
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
			Topics:  strings.Split(viper.GetString("KAFKA_TOPICS"), ","),
			GroupID: viper.GetString("KAFKA_GROUP_ID"),
		},
		Email: EmailConfig{
			FromAddress: viper.GetString("EMAIL_FROM_ADDRESS"),
			FromName:    viper.GetString("EMAIL_FROM_NAME"),
		},
		SMTP: SMTPConfig{
			Host:     viper.GetString("SMTP_HOST"),
			Port:     viper.GetInt("SMTP_PORT"),
			Username: viper.GetString("SMTP_USERNAME"),
			Password: viper.GetString("SMTP_PASSWORD"),
		},
	}

	Cfg = cfg
	return cfg
}
