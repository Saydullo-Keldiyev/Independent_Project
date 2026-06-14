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
	JWT       JWTConfig
	Scheduler SchedulerConfig
}

type AppConfig struct{ Port, Env string }
type DBConfig struct{ URL string }
type RedisConfig struct{ Addr, Password string; DB int }
type KafkaConfig struct{ Brokers []string; Topic string }
type JWTConfig struct{ Secret string }
type SchedulerConfig struct{ IntervalSeconds, LockTTLSeconds int }

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
		Kafka: KafkaConfig{Brokers: strings.Split(viper.GetString("KAFKA_BROKERS"), ","), Topic: viper.GetString("KAFKA_TOPIC")},
		JWT:   JWTConfig{Secret: viper.GetString("JWT_SECRET")},
		Scheduler: SchedulerConfig{
			IntervalSeconds: viper.GetInt("SCHEDULER_INTERVAL_SECONDS"),
			LockTTLSeconds:  viper.GetInt("LOCK_TTL_SECONDS"),
		},
	}
	if cfg.Scheduler.IntervalSeconds == 0 {
		cfg.Scheduler.IntervalSeconds = 5
	}
	if cfg.Scheduler.LockTTLSeconds == 0 {
		cfg.Scheduler.LockTTLSeconds = 10
	}
	Cfg = cfg
	return cfg
}
