package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/auction-system/api-gateway/internal/config"
	"github.com/auction-system/api-gateway/internal/observability"
	"github.com/auction-system/api-gateway/internal/pkginit"
	"github.com/auction-system/api-gateway/internal/router"
)

func main() {
	cfg := config.MustLoad()

	if err := observability.InitLogger(cfg.App.Env); err != nil {
		panic(err)
	}
	defer observability.Sync()
	log := observability.Log

	// Initialize shared packages (structured logger, circuit breakers, validation).
	sharedSvc, err := pkginit.Init(cfg.App.Env)
	if err != nil {
		log.Warn("shared packages init failed — continuing with local logger", zap.Error(err))
	} else {
		// Use structured logger from shared pkg for correlation ID propagation.
		sharedSvc.Logger.Info("shared packages ready for api-gateway")
		_ = sharedSvc // Available for middleware and handlers that need pkg/ integrations.
	}

	shutdownTrace, err := observability.InitTracing(
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		cfg.App.Env,
	)
	if err != nil {
		log.Warn("tracing disabled", zap.Error(err))
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownTrace(ctx)
		}()
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("redis connection failed", zap.Error(err))
	}
	cancel()
	defer redisClient.Close()

	r := router.Setup(cfg, redisClient)

	srv := &http.Server{
		Addr:              ":" + cfg.App.Port,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // WebSocket needs no write timeout
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("API Gateway started",
			zap.String("port", cfg.App.Port),
			zap.String("env", cfg.App.Env),
			zap.Int("rate_limit_per_min", cfg.RateLimit.PerMinute),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("API Gateway stopped")
}
