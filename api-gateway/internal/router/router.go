package router

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/auction-system/api-gateway/internal/auth"
	"github.com/auction-system/api-gateway/internal/config"
	"github.com/auction-system/api-gateway/internal/discovery"
	"github.com/auction-system/api-gateway/internal/middleware"
	"github.com/auction-system/api-gateway/internal/observability"
	"github.com/auction-system/api-gateway/internal/proxy"
)

// Setup builds the production API Gateway router — single entry point for all clients.
func Setup(cfg *config.Config, redisClient *redis.Client) *gin.Engine {
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	jwtVal := auth.NewValidator(cfg.JWT.Secret, redisClient)
	reg := discovery.NewRegistry(
		cfg.Services.UserServiceURL,
		cfg.Services.AuctionServiceURL,
		cfg.Services.BidServiceURL,
		cfg.Services.NotificationServiceURL,
	)
	gw := proxy.NewGateway(reg, cfg.Proxy.Timeout, cfg.Proxy.Retries)

	// Sliding window rate limiter with IP blocking, endpoint-specific limits,
	// and automatic fallback to in-memory when Redis is unavailable.
	slidingRL := middleware.NewSlidingWindowRateLimiter(redisClient, observability.Log)

	r.Use(middleware.Recovery())
	r.Use(middleware.CorrelationID())
	r.Use(middleware.AccessLog())
	r.Use(middleware.SecurityHeaders(cfg.App.Env != "production"))
	r.Use(middleware.CORSMiddleware(cfg.CORS.AllowedOrigins))
	r.Use(slidingRL.Middleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "api-gateway"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if err := redisClient.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "redis": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// ── Swagger UI ───────────────────────────────────────────────────────────
	r.GET("/swagger", func(c *gin.Context) {
		// Override CSP for Swagger UI — allow CDN scripts/styles
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data: https://unpkg.com;")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, swaggerHTML)
	})
	r.GET("/swagger/spec.yaml", func(c *gin.Context) {
		// Try multiple paths (works regardless of where exe is launched from)
		paths := []string{
			"docs/swagger.yaml",
			"../docs/swagger.yaml",
			"../../docs/swagger.yaml",
			"d:/Go_Projects/auction-system/docs/swagger.yaml",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				c.File(p)
				return
			}
		}
		c.String(http.StatusNotFound, "swagger.yaml not found")
	})

	v1 := r.Group("/api/v1")

	// ── USER SERVICE — public auth ───────────────────────────────────────────
	authPublic := v1.Group("/auth")
	{
		authPublic.POST("/register", gw.Handler(discovery.UserService))
		authPublic.POST("/login", gw.Handler(discovery.UserService))
		authPublic.POST("/refresh", gw.Handler(discovery.UserService))
	}

	// ── AUCTION SERVICE — public read ────────────────────────────────────────
	auctionPublic := v1.Group("/auctions")
	{
		auctionPublic.GET("", gw.Handler(discovery.AuctionService))
		auctionPublic.GET("/search", gw.Handler(discovery.AuctionService))
		auctionPublic.GET("/category/:id", gw.Handler(discovery.AuctionService))
		auctionPublic.GET("/:id/bids", gw.Handler(discovery.BidService))
		auctionPublic.GET("/:id", gw.Handler(discovery.AuctionService))
	}

	// ── Authenticated routes ─────────────────────────────────────────────────
	authed := v1.Group("")
	authed.Use(middleware.Auth(jwtVal))
	{
		// User service
		authed.POST("/auth/logout", gw.Handler(discovery.UserService))
		authed.GET("/users/me", gw.Handler(discovery.UserService))
		authed.PUT("/users/me", gw.Handler(discovery.UserService))
		authed.GET("/wallet", gw.Handler(discovery.UserService))
		authed.GET("/wallet/history", gw.Handler(discovery.UserService))
		authed.POST("/wallet/deposit", gw.Handler(discovery.UserService))

		// Bid service
		authed.POST("/bids", gw.Handler(discovery.BidService))
		authed.GET("/bids/me", gw.Handler(discovery.BidService))
	}

	// WebSocket — bid service (JWT required)
	ws := v1.Group("")
	ws.Use(middleware.Auth(jwtVal))
	{
		ws.GET("/ws/:auction_id", gw.WebSocketHandler(discovery.BidService))
	}

	// Seller — auction CRUD
	seller := v1.Group("")
	seller.Use(middleware.Auth(jwtVal))
	seller.Use(middleware.RequireSellerOrAdmin())
	{
		seller.POST("/auctions", gw.Handler(discovery.AuctionService))
		seller.PUT("/auctions/:id", gw.Handler(discovery.AuctionService))
		seller.DELETE("/auctions/:id", gw.Handler(discovery.AuctionService))
		seller.GET("/seller/auctions", gw.Handler(discovery.AuctionService))
	}

	// Admin
	admin := v1.Group("/admin")
	admin.Use(middleware.Auth(jwtVal))
	admin.Use(middleware.RequireAdmin())
	{
		admin.DELETE("/auctions/:id", gw.Handler(discovery.AuctionService))
		// User management
		admin.GET("/users", gw.Handler(discovery.UserService))
		admin.PUT("/users/:id/role", gw.Handler(discovery.UserService))
		admin.POST("/users/:id/ban", gw.Handler(discovery.UserService))
		admin.POST("/users/:id/unban", gw.Handler(discovery.UserService))
		admin.DELETE("/users/:id", gw.Handler(discovery.UserService))
	}

	return r
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Auction System API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .topbar { display: none; }
        .swagger-ui .info .title { font-size: 2em; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
    <script>
        SwaggerUIBundle({
            url: '/swagger/spec.yaml',
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.SwaggerUIStandalonePreset
            ],
            layout: "BaseLayout",
            defaultModelsExpandDepth: 1,
            docExpansion: "list",
            filter: true,
            tryItOutEnabled: true
        });
    </script>
</body>
</html>`
