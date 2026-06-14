package proxy

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/auction-system/api-gateway/internal/discovery"
	"github.com/auction-system/api-gateway/internal/observability"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (g *Gateway) WebSocketHandler(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		observability.ActiveWebSockets.Inc()
		defer observability.ActiveWebSockets.Dec()

		base := g.lb[service].Next()
		targetURL := base + c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			targetURL += "?" + c.Request.URL.RawQuery
		}

		u, err := url.Parse(targetURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid upstream"})
			return
		}
		if u.Scheme == "http" {
			u.Scheme = "ws"
		} else if u.Scheme == "https" {
			u.Scheme = "wss"
		}

		dialer := websocket.Dialer{
			HandshakeTimeout: g.client.Timeout,
		}
		hdr := http.Header{}
		for k, vals := range c.Request.Header {
			if strings.EqualFold(k, "Connection") || strings.EqualFold(k, "Upgrade") {
				continue
			}
			for _, v := range vals {
				hdr.Add(k, v)
			}
		}
		if cid := c.GetString("correlation_id"); cid != "" {
			hdr.Set(HeaderCorrelationID, cid)
		}
		if uid := c.GetString("user_id"); uid != "" {
			hdr.Set(HeaderUserID, uid)
			hdr.Set(HeaderUserRole, c.GetString("role"))
			hdr.Set(HeaderGatewayTrust, "true")
		}

		clientConn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer clientConn.Close()

		backendConn, resp, err := dialer.Dial(u.String(), hdr)
		if err != nil {
			observability.GatewayErrors.WithLabelValues(discovery.BidService, "websocket").Inc()
			observability.Log.Warn("websocket dial failed", zap.Error(err))
			clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "upstream unavailable"))
			if resp != nil {
				_ = resp.StatusCode
			}
			return
		}
		defer backendConn.Close()

		errCh := make(chan error, 2)
		go copyWS(clientConn, backendConn, errCh)
		go copyWS(backendConn, clientConn, errCh)
		<-errCh
	}
}

func copyWS(dst, src *websocket.Conn, errCh chan error) {
	for {
		mt, msg, err := src.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if err := dst.WriteMessage(mt, msg); err != nil {
			errCh <- err
			return
		}
	}
}
