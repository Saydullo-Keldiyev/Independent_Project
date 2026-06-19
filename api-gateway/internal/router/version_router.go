package router

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// VersionConfig holds configuration for a specific API version.
type VersionConfig struct {
	// Deprecated indicates whether this version is deprecated.
	Deprecated bool
	// SunsetDate is the date after which this version will no longer be available.
	// Must be set when Deprecated is true.
	SunsetDate time.Time
}

// VersionRouter manages API versioning with support for 2-3 concurrent versions,
// deprecation headers, and sunset enforcement.
type VersionRouter struct {
	mu          sync.RWMutex
	versions    map[string]*gin.RouterGroup // registered version route groups
	configs     map[string]VersionConfig    // version configuration (deprecation, sunset)
	maxVersions int                         // maximum concurrent versions (default: 3)
	engine      *gin.Engine
}

// NewVersionRouter creates a new VersionRouter.
// maxVersions controls how many concurrent versions can be active (2-3).
func NewVersionRouter(engine *gin.Engine, maxVersions int) *VersionRouter {
	if maxVersions < 2 {
		maxVersions = 2
	}
	if maxVersions > 3 {
		maxVersions = 3
	}
	return &VersionRouter{
		versions:    make(map[string]*gin.RouterGroup),
		configs:     make(map[string]VersionConfig),
		maxVersions: maxVersions,
		engine:      engine,
	}
}

// RegisterVersion registers a new API version and returns the router group for that version.
// The version string should be like "v1", "v2", etc.
// Returns an error if the maximum number of versions would be exceeded.
func (vr *VersionRouter) RegisterVersion(version string, config VersionConfig) (*gin.RouterGroup, error) {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	// Count active (non-sunset) versions
	activeCount := 0
	for v, cfg := range vr.configs {
		if v == version {
			continue // don't count if re-registering same version
		}
		if !cfg.Deprecated || cfg.SunsetDate.IsZero() || time.Now().Before(cfg.SunsetDate) {
			activeCount++
		}
	}

	if _, exists := vr.versions[version]; !exists && activeCount >= vr.maxVersions {
		return nil, fmt.Errorf("maximum of %d concurrent versions reached", vr.maxVersions)
	}

	prefix := "/api/" + version
	group := vr.engine.Group(prefix)
	group.Use(vr.versionMiddleware(version))

	vr.versions[version] = group
	vr.configs[version] = config

	return group, nil
}

// versionMiddleware returns a Gin middleware that handles deprecation headers
// and sunset enforcement for a specific version.
func (vr *VersionRouter) versionMiddleware(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		vr.mu.RLock()
		cfg, exists := vr.configs[version]
		vr.mu.RUnlock()

		if !exists {
			c.Next()
			return
		}

		now := time.Now()

		// Check if version is past sunset date → return 410 Gone
		if cfg.Deprecated && !cfg.SunsetDate.IsZero() && now.After(cfg.SunsetDate) {
			supportedVersions := vr.getSupportedVersions()
			c.AbortWithStatusJSON(http.StatusGone, gin.H{
				"success": false,
				"error": gin.H{
					"code":               "VERSION_GONE",
					"message":            fmt.Sprintf("API version %s has been removed", version),
					"supported_versions": supportedVersions,
				},
			})
			return
		}

		// Add Deprecation header for deprecated versions
		if cfg.Deprecated && !cfg.SunsetDate.IsZero() {
			// RFC 7231 date format: Mon, 02 Jan 2006 15:04:05 GMT
			c.Header("Deprecation", cfg.SunsetDate.UTC().Format(http.TimeFormat))
			c.Header("Sunset", cfg.SunsetDate.UTC().Format(http.TimeFormat))
		}

		c.Next()
	}
}

// VersionNotFoundHandler returns a Gin handler that responds with HTTP 404
// and a list of supported versions when an unrecognized version prefix is hit.
func (vr *VersionRouter) VersionNotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Only intercept paths that look like versioned API paths
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// Extract version from path: /api/v3/... → v3
		parts := strings.SplitN(strings.TrimPrefix(path, "/api/"), "/", 2)
		if len(parts) == 0 {
			c.Next()
			return
		}

		requestedVersion := parts[0]
		if requestedVersion == "" {
			c.Next()
			return
		}

		// Check if this is a registered version
		vr.mu.RLock()
		_, exists := vr.versions[requestedVersion]
		vr.mu.RUnlock()

		if exists {
			// Version exists, let normal routing handle it
			c.Next()
			return
		}

		// Unrecognized version — return 404 with supported versions
		supportedVersions := vr.getSupportedVersions()
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":               "VERSION_NOT_FOUND",
				"message":            fmt.Sprintf("API version %s is not available", requestedVersion),
				"supported_versions": supportedVersions,
			},
		})
	}
}

// getSupportedVersions returns a list of currently supported (non-sunset) versions.
func (vr *VersionRouter) getSupportedVersions() []string {
	vr.mu.RLock()
	defer vr.mu.RUnlock()

	now := time.Now()
	supported := make([]string, 0, len(vr.versions))

	for version, cfg := range vr.configs {
		// Include version if it's not past sunset
		if !cfg.Deprecated || cfg.SunsetDate.IsZero() || now.Before(cfg.SunsetDate) {
			supported = append(supported, "/api/"+version)
		}
	}

	return supported
}

// GetVersionGroup returns the router group for a registered version, or nil if not found.
func (vr *VersionRouter) GetVersionGroup(version string) *gin.RouterGroup {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	return vr.versions[version]
}

// UpdateVersionConfig updates the configuration for an existing version.
func (vr *VersionRouter) UpdateVersionConfig(version string, config VersionConfig) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	if _, exists := vr.versions[version]; !exists {
		return fmt.Errorf("version %s is not registered", version)
	}

	vr.configs[version] = config
	return nil
}

// SupportedVersions returns the list of currently active version strings.
func (vr *VersionRouter) SupportedVersions() []string {
	return vr.getSupportedVersions()
}
