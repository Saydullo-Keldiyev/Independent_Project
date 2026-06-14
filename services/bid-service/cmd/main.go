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

	"github.com/auction-system/bid-service/internal/config"
	"github.com/auction-system/bid-service/internal/database"
	"github.com/auction-system/bid-service/internal/handler"
	kafkaPkg "github.com/auction-system/bid-service/internal/kafka"
	"github.com/auction-system/bid-service/internal/middleware"
	"github.com/auction-system/bid-service/internal/observability"
	redisPkg "github.com/auction-system/bid-service/internal/redis"
	"github.com/auction-system/bid-service/internal/utils"
	walletPkg "github.com/auction-system/bid-service/internal/wallet"
	wsPkg "github.com/auction-system/bid-service/internal/websocket"
)

func main() {
	// ── 1. Load config ────────────────────────────────────────────────────
	cfg := config.Load()

	// ── 2. Init structured logger (FIRST — everything else logs) ─────────
	if err := observability.InitLogger(cfg.App.Env); err != nil {
		panic("failed to init logger: " + err.Error())
	}
	defer observability.Sync()

	log := observability.Log
	log.Info("starting bid-service",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	// ── 3. Init distributed tracing ───────────────────────────────────────
	shutdownTracing, err := observability.InitTracing(observability.TracerConfig{
		OTLPEndpoint: cfg.Tracing.OTLPEndpoint,
		Environment:  cfg.App.Env,
		Version:      "1.0.0",
	})
	if err != nil {
		log.Warn("tracing init failed — continuing without tracing", zap.Error(err))
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			shutdownTracing(ctx)
		}()
		log.Info("✅ OpenTelemetry tracing initialized",
			zap.String("endpoint", cfg.Tracing.OTLPEndpoint),
		)
	}

	// ── 4. Connect to PostgreSQL ──────────────────────────────────────────
	if err := database.Connect(cfg.DB.URL); err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer database.Close()
	log.Info("✅ PostgreSQL connected")

	// ── 5. Connect to Redis ───────────────────────────────────────────────
	if err := redisPkg.Connect(redisPkg.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}); err != nil {
		log.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisPkg.Close()
	log.Info("✅ Redis connected")

	// ── 6. Init Kafka producer ────────────────────────────────────────────
	kafkaPkg.InitProducer(kafkaPkg.ProducerConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topic,
	})
	defer kafkaPkg.Close()

	// ── 6.5. Init wallet client (user-service integration) ────────────────
	if cfg.UserServiceURL != "" {
		walletPkg.Init(cfg.UserServiceURL)
		log.Info("✅ Wallet client initialized", zap.String("url", cfg.UserServiceURL))
	} else {
		log.Warn("USER_SERVICE_URL not set — wallet integration disabled")
	}

	// Wire Kafka metrics recorder (avoids import cycle)
	kafkaPkg.SetMetricsRecorder(func(topic string, duration float64, err error) {
		status := "success"
		if err != nil {
			status = "error"
			observability.KafkaPublishTotal.WithLabelValues(topic, status).Inc()
			return
		}
		observability.KafkaPublishTotal.WithLabelValues(topic, status).Inc()
		observability.KafkaPublishDuration.WithLabelValues(topic).Observe(duration)
	})
	log.Info("✅ Kafka producer initialized")

	// ── 7. Register custom validators ────────────────────────────────────
	utils.RegisterCustomValidators()

	// ── 8. Start WebSocket hub ────────────────────────────────────────────
	go wsPkg.H.Run()
	log.Info("✅ WebSocket hub started")

	// ── 9. Setup Gin router ───────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Global middleware — order matters
	r.Use(gin.Recovery())
	r.Use(observability.ObservabilityMiddleware()) // logging + metrics + tracing

	// ── Probe endpoints (no auth, no rate limit) ──────────────────────────
	r.GET("/health", handler.Health)
	r.GET("/ready", handler.Ready)

	// ── Prometheus metrics endpoint ───────────────────────────────────────
	// Protect this in production (e.g. only accessible from internal network)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// ── Public API routes ─────────────────────────────────────────────────
	api := r.Group("/api/v1")
	{
		api.GET("/auctions/:auction_id/bids", handler.GetBidsByAuction)
		api.GET("/auctions/:auction_id/bid-history", handler.GetBidHistory)
		api.GET("/auctions/:auction_id/min-bid", handler.GetMinBid)
	}

	// ── Protected routes (JWT required) ──────────────────────────────────
	authGroup := api.Group("/")
	authGroup.Use(middleware.AuthMiddleware())
	{
		authGroup.POST("/bids", handler.PlaceBid)
		authGroup.GET("/bids/me", handler.GetMyBids)
		authGroup.GET("/ws/:auction_id", handler.ServeWS)
	}

	// ── HTTP server with production timeouts ──────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("🚀 bid-service is running",
			zap.String("port", cfg.App.Port),
			zap.String("env", cfg.App.Env),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", zap.String("signal", sig.String()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", zap.Error(err))
	}

	log.Info("bid-service exited cleanly")
}
