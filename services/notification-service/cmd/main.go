package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/auction-system/notification-service/internal/config"
	"github.com/auction-system/notification-service/internal/consumer"
	"github.com/auction-system/notification-service/internal/database"
	"github.com/auction-system/notification-service/internal/email"
	"github.com/auction-system/notification-service/internal/kafka"
	redisPkg "github.com/auction-system/notification-service/internal/redis"
	"github.com/auction-system/notification-service/internal/repository"
	"github.com/auction-system/notification-service/internal/service"
	wsPkg "github.com/auction-system/notification-service/internal/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func main() {
	cfg := config.Load()

	// ── Logger ────────────────────────────────────────────────────────────
	var log *zap.Logger
	if cfg.App.Env == "production" {
		log, _ = zap.NewProduction()
	} else {
		log, _ = zap.NewDevelopment()
	}
	defer log.Sync()

	log.Info("starting notification-service",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	// ── Database ──────────────────────────────────────────────────────────
	if cfg.DB.URL != "" {
		if err := database.Connect(cfg.DB.URL); err != nil {
			log.Warn("database connection failed — persistence disabled", zap.Error(err))
		} else {
			defer database.Close()
			log.Info("✅ PostgreSQL connected")
		}
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	if err := redisPkg.Connect(redisPkg.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}); err != nil {
		log.Warn("redis connection failed — WS scaling disabled", zap.Error(err))
	} else {
		defer redisPkg.Close()
		log.Info("✅ Redis connected")
	}

	// ── Email sender ──────────────────────────────────────────────────────
	var emailSender *email.Sender
	if cfg.SMTP.Host != "" {
		emailSender = email.NewSender(email.Config{
			Host:        cfg.SMTP.Host,
			Port:        cfg.SMTP.Port,
			Username:    cfg.SMTP.Username,
			Password:    cfg.SMTP.Password,
			FromAddress: cfg.Email.FromAddress,
			FromName:    cfg.Email.FromName,
		})
		log.Info("✅ Email sender initialized")
	} else {
		log.Warn("SMTP not configured — email disabled")
	}

	// ── WebSocket Hub ─────────────────────────────────────────────────────
	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

	hub := wsPkg.NewHub(log)
	go hub.Run(ctx)
	log.Info("✅ WebSocket hub started")

	// ── Notification service ──────────────────────────────────────────────
	notifService := service.New(emailSender, log)

	// ── Dead Letter Queue ─────────────────────────────────────────────────
	var dlq *kafka.DeadLetterQueue
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		dlq = kafka.NewDeadLetterQueue(cfg.Kafka.Brokers, log)
		defer dlq.Close()
	}

	// ── Kafka consumer ────────────────────────────────────────────────────
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		kafkaConsumer := consumer.New(
			cfg.Kafka.Brokers,
			cfg.Kafka.Topics,
			cfg.Kafka.GroupID,
			notifService,
			dlq,
			log,
		)
		defer kafkaConsumer.Close()

		go func() {
			if err := kafkaConsumer.Run(ctx); err != nil {
				log.Error("kafka consumer error", zap.Error(err))
			}
		}()
		log.Info("✅ Kafka consumer started", zap.Strings("topics", cfg.Kafka.Topics))
	}

	// ── HTTP server ───────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Health probes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "notification-service"})
	})
	r.GET("/ready", func(c *gin.Context) {
		checks := gin.H{}
		status := http.StatusOK

		if database.DB != nil {
			if err := database.DB.Ping(c.Request.Context()); err != nil {
				checks["postgres"] = "unhealthy"
				status = http.StatusServiceUnavailable
			} else {
				checks["postgres"] = "ok"
			}
		}

		if redisPkg.Client != nil {
			if err := redisPkg.Client.Ping(c.Request.Context()).Err(); err != nil {
				checks["redis"] = "unhealthy"
				status = http.StatusServiceUnavailable
			} else {
				checks["redis"] = "ok"
			}
		}

		c.JSON(status, gin.H{"status": "ready", "checks": checks})
	})

	// Metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// WebSocket endpoint — user connects here for real-time notifications
	r.GET("/ws", func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID == "" {
			userID = c.GetHeader("X-User-ID")
		}
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id required"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error("ws upgrade failed", zap.Error(err))
			return
		}

		client := wsPkg.NewClient(hub, conn, userID, log)
		hub.Register <- client

		go client.WritePump()
		go client.ReadPump()
	})

	// API — notification history
	api := r.Group("/api/v1/notifications")
	{
		api.GET("", func(c *gin.Context) {
			userID := c.GetHeader("X-User-ID")
			if userID == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}

			notifs, err := repository.GetByUser(c.Request.Context(), userID, 50, 0)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get notifications"})
				return
			}

			unread, _ := repository.UnreadCount(c.Request.Context(), userID)

			c.JSON(http.StatusOK, gin.H{
				"notifications": notifs,
				"unread_count":  unread,
			})
		})

		api.POST("/:id/read", func(c *gin.Context) {
			userID := c.GetHeader("X-User-ID")
			id := c.Param("id")
			if err := repository.MarkAsRead(c.Request.Context(), id, userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		api.POST("/read-all", func(c *gin.Context) {
			userID := c.GetHeader("X-User-ID")
			if err := repository.MarkAllRead(c.Request.Context(), userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true})
		})
	}

	// ── Start server ──────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second, // longer for WebSocket
	}

	go func() {
		log.Info("🚀 notification-service running", zap.String("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", zap.String("signal", sig.String()))
	cancelAll() // stop WS hub + kafka consumer

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", zap.Error(err))
	}

	log.Info("notification-service exited cleanly")
}
