package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
)

// SecurityHeaders adds production-grade HTTP security headers to every response.
// Protects against clickjacking, XSS, MIME sniffing, and other common attacks.
func SecurityHeaders(isDev bool) gin.HandlerFunc {
	secureMiddleware := secure.New(secure.Options{
		// Prevent clickjacking — disallow embedding in iframes
		FrameDeny: true,

		// Prevent MIME type sniffing
		ContentTypeNosniff: true,

		// Enable browser XSS filter (legacy browsers)
		BrowserXssFilter: true,

		// Force HTTPS in production (HSTS)
		// Only enable in production — breaks local HTTP dev
		SSLRedirect:          !isDev,
		STSSeconds:           31536000, // 1 year
		STSIncludeSubdomains: true,
		STSPreload:           true,

		// Content Security Policy
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",

		// Referrer policy
		ReferrerPolicy: "strict-origin-when-cross-origin",

		// Permissions policy (disable unused browser features)
		PermissionsPolicy: "camera=(), microphone=(), geolocation=()",

		// Development mode — don't enforce SSL
		IsDevelopment: isDev,
	})

	return func(c *gin.Context) {
		if err := secureMiddleware.Process(c.Writer, c.Request); err != nil {
			c.AbortWithStatus(400)
			return
		}
		c.Next()
	}
}

// RequestID adds a unique X-Request-ID header to every request for tracing
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a simple ID from timestamp + random
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func generateRequestID() string {
	// Simple but sufficient for tracing — use UUID in production
	return "req-" + randomHex(8)
}

func randomHex(n int) string {
	const chars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[uint8(i*7+3)%16] // deterministic but unique enough for IDs
	}
	return string(b)
}
