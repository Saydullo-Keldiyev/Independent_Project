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

	"github.com/auction-system/auction-service/internal/config"
	"github.com/auction-system/auction-service/internal/database"
	"github.com/auction-system/auction-service/internal/handler"
	kafkaPkg "github.com/auction-system/auction-service/internal/kafka"
	"github.com/auction-system/auction-service/internal/middleware"
	"github.com/auction-system/auction-service/internal/pkginit"
	redisPkg "github.com/auction-system/auction-service/internal/redis"
	"github.com/auction-system/auction-service/internal/scheduler"
	"github.com/auction-system/auction-service/internal/service"
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

	log.Info("starting auction-service", zap.String("port", cfg.App.Port))

	// ── Database ──────────────────────────────────────────────────────────
	if err := database.Connect(cfg.DB.URL); err != nil {
		log.Fatal("database failed", zap.Error(err))
	}
	defer database.Close()
	log.Info("✅ PostgreSQL connected")

	// ── Redis ─────────────────────────────────────────────────────────────
	if err := redisPkg.Connect(redisPkg.Options{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	}); err != nil {
		log.Warn("redis failed — caching disabled", zap.Error(err))
	} else {
		defer redisPkg.Close()
		log.Info("✅ Redis connected")
	}

	// ── Kafka ─────────────────────────────────────────────────────────────
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		kafkaPkg.InitProducer(kafkaPkg.ProducerConfig{
			Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.Topic,
		})
		defer kafkaPkg.Close()
		log.Info("✅ Kafka producer initialized")
	}

	// ── Shared packages (structured logger, circuit breakers, validation) ──
	sharedSvc, sharedErr := pkginit.Init(pkginit.Config{
		Environment:  cfg.App.Env,
		KafkaBrokers: cfg.Kafka.Brokers,
		KafkaTopic:   cfg.Kafka.Topic,
	})
	if sharedErr != nil {
		log.Warn("shared packages init failed — continuing with local implementations", zap.Error(sharedErr))
	} else {
		defer sharedSvc.Close()
		sharedSvc.Logger.Info("shared packages ready for auction-service")
		_ = sharedSvc
	}

	// ── Services ──────────────────────────────────────────────────────────
	auctionSvc := service.New(log)
	auctionH := handler.NewAuctionHandler(auctionSvc)

	// ── Scheduler (background worker) ─────────────────────────────────────
	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

	sched := scheduler.New(cfg.Scheduler.IntervalSeconds, cfg.Scheduler.LockTTLSeconds, log)
	go sched.Run(ctx)
	log.Info("✅ Auction scheduler started",
		zap.Int("interval_sec", cfg.Scheduler.IntervalSeconds),
	)

	// ── Router ────────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "auction-service"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if err := database.DB.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public
	api := r.Group("/api/v1")
	{
		api.GET("/auctions", auctionH.List)
		api.GET("/auctions/:id", auctionH.GetByID)
		api.GET("/auctions/:id/bids", auctionH.GetAuctionBids)
		api.GET("/categories", auctionH.GetCategories)
	}

	// Protected (seller/bidder)
	auth := api.Group("/")
	auth.Use(middleware.AuthMiddleware())
	{
		auth.POST("/auctions", middleware.RequireRole("seller", "admin"), auctionH.Create)
		auth.PUT("/auctions/:id", auctionH.Update)
		auth.DELETE("/auctions/:id", auctionH.Delete)
		auth.GET("/seller/auctions", auctionH.GetSellerAuctions)

		// Auction state transitions
		auth.POST("/auctions/:id/publish", middleware.RequireRole("seller", "admin"), auctionH.Publish)
		auth.POST("/auctions/:id/cancel", auctionH.Cancel)

		// Auction images
		auth.POST("/auctions/:id/images", middleware.RequireRole("seller", "admin"), auctionH.AddImage)
		auth.DELETE("/auctions/:id/images/:imageId", auctionH.DeleteImage)

		// Watchlist
		auth.POST("/watchlist/:auctionId", auctionH.AddToWatchlist)
		auth.DELETE("/watchlist/:auctionId", auctionH.RemoveFromWatchlist)
		auth.GET("/watchlist", auctionH.GetWatchlist)
	}

	// Admin
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.DELETE("/auctions/:id", auctionH.Delete)
		admin.GET("/auctions", auctionH.AdminListAll)
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
		log.Info("🚀 auction-service running", zap.String("port", cfg.App.Port))
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
	log.Info("auction-service exited cleanly")
}
