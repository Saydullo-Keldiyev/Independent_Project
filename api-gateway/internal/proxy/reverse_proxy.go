package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/api-gateway/internal/discovery"
	"github.com/auction-system/api-gateway/internal/observability"
)

const (
	HeaderUserID        = "X-User-ID"
	HeaderUserRole      = "X-User-Role"
	HeaderUserEmail     = "X-User-Email"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderGatewayTrust  = "X-Gateway-Trust"
)

type Gateway struct {
	registry *discovery.Registry
	lb       map[string]*RoundRobin
	cb       map[string]*CircuitBreaker
	client   *http.Client
	retries  int
}

func NewGateway(reg *discovery.Registry, timeout time.Duration, retries int) *Gateway {
	g := &Gateway{
		registry: reg,
		lb:       make(map[string]*RoundRobin),
		cb:       make(map[string]*CircuitBreaker),
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		retries: retries,
	}
	for _, name := range []string{discovery.UserService, discovery.AuctionService, discovery.BidService, discovery.NotificationService} {
		g.lb[name] = NewRoundRobin(reg.UpstreamURLs(name))
		g.cb[name] = NewCircuitBreaker(5, 30*time.Second)
	}
	return g
}

func (g *Gateway) Handler(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		g.Forward(c, service)
	}
}

func (g *Gateway) Forward(c *gin.Context, service string) {
	start := time.Now()
	cb := g.cb[service]

	if err := cb.Allow(); err != nil {
		observability.GatewayErrors.WithLabelValues(service, "circuit_open").Inc()
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "service temporarily unavailable",
		})
		return
	}

	base := g.lb[service].Next()
	url := base + c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		url += "?" + c.Request.URL.RawQuery
	}

	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= g.retries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
		}
		resp, err = g.doOnce(c, url, bodyBytes)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if err != nil {
		cb.RecordFailure()
		observability.GatewayErrors.WithLabelValues(service, "upstream").Inc()
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   fmt.Sprintf("upstream unavailable: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	cb.RecordSuccess()
	observability.GatewayLatency.WithLabelValues(service, c.Request.Method).Observe(time.Since(start).Seconds())
	observability.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), fmt.Sprintf("%d", resp.StatusCode), service).Inc()

	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func (g *Gateway) doOnce(c *gin.Context, url string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), g.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, c.Request.Method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, vals := range c.Request.Header {
		if k == "Host" {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	if cid := c.GetString("correlation_id"); cid != "" {
		req.Header.Set(HeaderCorrelationID, cid)
	}
	if uid := c.GetString("user_id"); uid != "" {
		req.Header.Set(HeaderUserID, uid)
		req.Header.Set(HeaderUserRole, c.GetString("role"))
		req.Header.Set(HeaderUserEmail, c.GetString("email"))
		req.Header.Set(HeaderGatewayTrust, "true")
	}

	return g.client.Do(req)
}
