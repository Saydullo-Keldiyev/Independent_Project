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

	"github.com/auction-system/analytics-service/internal/cache"
	"github.com/auction-system/analytics-service/internal/config"
	"github.com/auction-system/analytics-service/internal/consumer"
	"github.com/auction-system/analytics-service/internal/database"
	"github.com/auction-system/analytics-service/internal/handler"
	"github.com/auction-system/analytics-service/internal/repository"
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

	log.Info("starting analytics-service", zap.String("port", cfg.App.Port))

	// ── Database ──────────────────────────────────────────────────────────
	if err := database.Connect(cfg.DB.URL); err != nil {
		log.Fatal("database failed", zap.Error(err))
	}
	defer database.Close()
	log.Info("✅ PostgreSQL connected")

	// ── Redis ─────────────────────────────────────────────────────────────
	if err := cache.Connect(cache.Options{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	}); err != nil {
		log.Warn("redis failed — realtime counters disabled", zap.Error(err))
	} else {
		defer cache.Close()
		log.Info("✅ Redis connected")
	}

	// ── Repository & Handlers ─────────────────────────────────────────────
	repo := repository.New()
	dashH := handler.NewDashboardHandler(repo)

	// ── Kafka consumer ────────────────────────────────────────────────────
	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		c := consumer.New(cfg.Kafka.Brokers, cfg.Kafka.Topics, cfg.Kafka.GroupID, repo, log)
		defer c.Close()
		go func() {
			if err := c.Run(ctx); err != nil {
				log.Error("consumer error", zap.Error(err))
			}
		}()
		log.Info("✅ Kafka consumer started")
	}

	// ── Router ────────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "analytics-service"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if err := database.DB.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api/v1/analytics")
	{
		api.GET("/admin/dashboard", dashH.AdminDashboard)
		api.GET("/revenue", dashH.Revenue)
		api.GET("/seller/:id", dashH.SellerDashboard)
		api.GET("/auction/:id", dashH.AuctionAnalytics)
		api.GET("/trending", dashH.Trending)
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
		log.Info("🚀 analytics-service running", zap.String("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancelAll()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	log.Info("analytics-service exited cleanly")
}
