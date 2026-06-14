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

	"github.com/auction-system/user-service/internal/config"
	"github.com/auction-system/user-service/internal/consumer"
	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/handler"
	kafkaPkg "github.com/auction-system/user-service/internal/kafka"
	"github.com/auction-system/user-service/internal/middleware"
	"github.com/auction-system/user-service/internal/model"
	"github.com/auction-system/user-service/internal/observability"
	"github.com/auction-system/user-service/internal/redis"
	"github.com/auction-system/user-service/internal/repository"
	"github.com/auction-system/user-service/internal/service"
)

func main() {
	cfg := config.MustLoad()

	if err := observability.InitLogger(cfg.App.Env); err != nil {
		panic(err)
	}
	defer observability.Sync()
	log := observability.Log

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	shutdownTracing, err := observability.InitTracing(observability.TracerConfig{
		OTLPEndpoint: cfg.Tracing.OTLPEndpoint,
		Environment:  cfg.App.Env,
		Version:      "1.0.0",
	})
	if err != nil {
		log.Warn("tracing disabled", zap.Error(err))
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownTracing(ctx)
		}()
	}

	if err := database.Connect(cfg.DB.URL); err != nil {
		log.Fatal("database connection failed", zap.Error(err))
	}
	defer database.Close()

	if err := redis.Connect(redis.Options{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	}); err != nil {
		log.Fatal("redis connection failed", zap.Error(err))
	}
	defer func() { _ = redis.Close() }()

	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		kafkaPkg.InitProducer(kafkaPkg.ProducerConfig{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.Topic})
		defer func() { _ = kafkaPkg.Close() }()
	}

	go refreshSessionGauge(repository.NewSessionRepository())

	authSvc := service.NewAuthService(cfg)
	userSvc := service.NewUserService()
	walletSvc := service.NewWalletService()

	// ── Start auction Kafka consumer for wallet settlement ────────────────
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		auctionTopic := "auction-events"
		if cfg.Kafka.AuctionTopic != "" {
			auctionTopic = cfg.Kafka.AuctionTopic
		}
		auctionConsumer := consumer.NewAuctionConsumer(
			cfg.Kafka.Brokers,
			auctionTopic,
			"user-service-wallet",
			walletSvc,
			log,
		)
		defer auctionConsumer.Close()
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := auctionConsumer.Run(ctx); err != nil {
				log.Error("auction consumer error", zap.Error(err))
			}
		}()
		log.Info("✅ Auction wallet consumer started", zap.String("topic", auctionTopic))
	}

	authH := handler.NewAuthHandler(authSvc)
	userH := handler.NewUserHandler(userSvc)
	walletH := handler.NewWalletHandler(walletSvc)
	healthH := handler.NewHealthHandler()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.MetricsMiddleware())

	r.GET("/health", healthH.Health)
	r.GET("/ready", healthH.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api/v1")
	auth := api.Group("/auth")
	auth.Use(middleware.RateLimit(30, time.Minute))
	{
		auth.POST("/register", authH.Register)
		auth.POST("/login", authH.Login)
		auth.POST("/refresh", authH.Refresh)
		auth.POST("/logout", middleware.AuthRequired(authSvc), authH.Logout)
		auth.POST("/forgot-password", authH.ForgotPassword)
		auth.POST("/reset-password", authH.ResetPassword)
		auth.POST("/verify-email", authH.VerifyEmail)
		auth.POST("/change-password", middleware.AuthRequired(authSvc), authH.ChangePassword)
	}

	// Sessions
	sessions := api.Group("/sessions")
	sessions.Use(middleware.AuthRequired(authSvc))
	{
		sessions.GET("", authH.GetSessions)
		sessions.DELETE("/:id", authH.DeleteSession)
		sessions.DELETE("", authH.DeleteAllSessions)
	}

	users := api.Group("/users")
	users.Use(middleware.AuthRequired(authSvc))
	{
		users.GET("/me", userH.GetMe)
		users.PUT("/me", userH.UpdateMe)
		users.DELETE("/me", userH.DeleteMe)
		users.POST("/avatar", userH.UploadAvatar)
		users.DELETE("/avatar", userH.DeleteAvatar)
	}

	// Public user profile (no auth)
	api.GET("/users/:id", userH.GetPublicProfile)

	wallet := api.Group("/wallet")
	wallet.Use(middleware.AuthRequired(authSvc))
	{
		wallet.GET("", walletH.GetWallet)
		wallet.GET("/history", walletH.History)
		wallet.POST("/deposit", walletH.Deposit)
		wallet.POST("/withdraw", walletH.Withdraw)
	}

	// Internal wallet API (called by other services — no user auth, use service key)
	internal := api.Group("/internal/wallet")
	{
		internal.POST("/hold", walletH.Hold)
		internal.POST("/release", walletH.Release)
		internal.POST("/settle", walletH.Settle)
		internal.POST("/credit", walletH.CreditSeller)
	}

	// Admin routes (admin role required)
	adminH := handler.NewAdminHandler()
	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.AuthRequired(authSvc))
	adminGroup.Use(middleware.RequireRole(model.RoleAdmin))
	{
		adminGroup.GET("/users", adminH.ListUsers)
		adminGroup.PUT("/users/:id/role", adminH.UpdateRole)
		adminGroup.POST("/users/:id/ban", adminH.BanUser)
		adminGroup.POST("/users/:id/unban", adminH.UnbanUser)
		adminGroup.DELETE("/users/:id", adminH.DeleteUser)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.App.Port,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("user-service listening", zap.String("port", cfg.App.Port), zap.String("env", cfg.App.Env))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("user-service stopped")
}

func refreshSessionGauge(sessions *repository.SessionRepository) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		n, err := sessions.ActiveCount(context.Background())
		if err == nil {
			observability.ActiveSessions.Set(float64(n))
		}
	}
}
