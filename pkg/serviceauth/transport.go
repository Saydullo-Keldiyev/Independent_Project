package serviceauth

import (
	"net/http"

	"go.uber.org/zap"
)

// Transport is an http.RoundTripper that automatically attaches
// service authentication tokens to outbound inter-service HTTP calls.
type Transport struct {
	// Base is the underlying transport. If nil, http.DefaultTransport is used.
	Base http.RoundTripper

	// Issuer creates service tokens for outbound calls.
	Issuer *TokenIssuer

	// HeaderName is the header to set. Defaults to "X-Service-Token".
	HeaderName string

	// Logger for transport errors.
	Logger *zap.Logger
}

// RoundTrip adds the service authentication token to every outgoing request.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	logger := t.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	// Clone the request to avoid mutating the original
	reqClone := req.Clone(req.Context())

	// Issue a fresh token for this request
	token, err := t.Issuer.IssueToken()
	if err != nil {
		logger.Error("failed to issue service token for outbound request",
			zap.String("url", req.URL.String()),
			zap.Error(err),
		)
		// Still send the request (the downstream will reject it), rather than failing here
	} else {
		headerName := t.HeaderName
		if headerName == "" {
			headerName = "X-Service-Token"
		}
		reqClone.Header.Set(headerName, token)
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(reqClone)
}
