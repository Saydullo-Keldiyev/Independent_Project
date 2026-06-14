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

	"github.com/auction-system/payment-service/internal/config"
	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/handler"
	"github.com/auction-system/payment-service/internal/middleware"
	"github.com/auction-system/payment-service/internal/service"
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

	log.Info("starting payment-service",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	// ── Database ──────────────────────────────────────────────────────────
	if err := database.Connect(cfg.DB.URL); err != nil {
		log.Fatal("database connection failed", zap.Error(err))
	}
	defer database.Close()
	log.Info("✅ PostgreSQL connected")

	// ── Services ──────────────────────────────────────────────────────────
	walletSvc := service.NewWalletService(log)

	// ── Outbox worker — publishes events to Kafka ─────────────────────────
	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		outboxWorker := service.NewOutboxWorker(cfg.Kafka.Brokers, cfg.Kafka.ProducerTopic, log)
		defer outboxWorker.Close()
		go outboxWorker.Run(ctx)
		log.Info("✅ Outbox worker started")
	}

	// ── Handlers ──────────────────────────────────────────────────────────
	walletH := handler.NewWalletHandler(walletSvc)

	// ── Router ────────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Health probes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "payment-service"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if err := database.DB.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// ── Wallet API (authenticated) ───────────────────────────────────────
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())
	{
		api.GET("/wallet", walletH.GetWallet)
		api.GET("/wallet/history", walletH.GetHistory)
		api.POST("/wallet/deposit", walletH.Deposit)
		api.POST("/payments/hold", walletH.HoldBalance)
	}

	// ── Admin API (internal / admin only) ─────────────────────────────────
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.POST("/payments/settle", walletH.SettleAuction)
	}

	// ── Server ────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("🚀 payment-service running", zap.String("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down payment-service...")
	cancelAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", zap.Error(err))
	}

	log.Info("payment-service exited cleanly")
}
