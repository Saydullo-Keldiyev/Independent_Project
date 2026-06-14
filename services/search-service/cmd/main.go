package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/auction-system/search-service/internal/cache"
	"github.com/auction-system/search-service/internal/config"
	"github.com/auction-system/search-service/internal/elastic"
	"github.com/auction-system/search-service/internal/handler"
	"github.com/auction-system/search-service/internal/kafka"
)

func main() {
	cfg := config.Load()

	var log *zap.Logger
	if cfg.App.Env == "production" {
		log, _ = zap.NewProduction()
	} else {
		log, _ = zap.NewDevelopment()
	}
	defer log.Sync()

	log.Info("starting search-service", zap.String("port", cfg.App.Port))

	// ── Elasticsearch ─────────────────────────────────────────────────────
	if err := elastic.Connect(cfg.Elastic.URL, log); err != nil {
		log.Fatal("elasticsearch connection failed", zap.Error(err))
	}

	// Ensure index exists with proper mapping
	if err := elastic.EnsureIndex(cfg.Elastic.Index, log); err != nil {
		log.Warn("ensure index failed (may already exist)", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	if err := cache.Connect(cache.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}); err != nil {
		log.Warn("redis connection failed — caching disabled", zap.Error(err))
	} else {
		defer cache.Close()
		log.Info("✅ Redis connected")
	}

	// ── Kafka consumer (indexing) ─────────────────────────────────────────
	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		indexConsumer := kafka.NewIndexConsumer(
			cfg.Kafka.Brokers,
			cfg.Kafka.Topics,
			cfg.Kafka.GroupID,
			cfg.Elastic.Index,
			log,
		)
		defer indexConsumer.Close()
		go func() {
			if err := indexConsumer.Run(ctx); err != nil {
				log.Error("index consumer error", zap.Error(err))
			}
		}()
		log.Info("✅ Kafka index consumer started")
	}

	// ── HTTP Router ───────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "search-service"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if elastic.Client == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Search API — public (rate limited at gateway level)
	api := r.Group("/api/v1")
	{
		api.GET("/search", handler.Search)
		api.GET("/search/suggest", handler.Suggest)
		api.GET("/search/trending", handler.Trending)
	}

	// ── Server ────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("🚀 search-service running", zap.String("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down search-service...")
	cancelAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	log.Info("search-service exited cleanly")
}
