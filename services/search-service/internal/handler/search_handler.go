package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/search-service/internal/cache"
	"github.com/auction-system/search-service/internal/config"
	"github.com/auction-system/search-service/internal/elastic"
	"github.com/auction-system/search-service/internal/model"
)

// Search handles GET /search?q=...&category=...&min_price=...&sort_by=...
func Search(c *gin.Context) {
	var req model.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	// Record for trending
	cache.RecordSearch(req.Query)

	// Check cache
	cacheKey := cache.CacheKey("main", req.Query, req.Category, req.State, req.SortBy, req.Page)
	if cached, _ := cache.GetCached(cacheKey); cached != nil {
		var resp model.SearchResponse
		if json.Unmarshal(cached, &resp) == nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": resp, "cached": true})
			return
		}
	}

	// Execute search
	resp, err := elastic.Search(c.Request.Context(), config.Cfg.Elastic.Index, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}

	// Cache result
	ttl := time.Duration(config.Cfg.Cache.TTLSeconds) * time.Second
	cache.SetCached(cacheKey, resp, ttl)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// Suggest handles GET /search/suggest?q=...
func Suggest(c *gin.Context) {
	prefix := c.Query("q")
	if prefix == "" || len(prefix) < 2 {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}})
		return
	}

	suggestions, err := elastic.Suggest(c.Request.Context(), config.Cfg.Elastic.Index, prefix, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": model.SuggestResponse{Suggestions: suggestions}})
}

// Trending handles GET /search/trending
func Trending(c *gin.Context) {
	items, err := cache.GetTrendingSearches(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "trending failed"})
		return
	}

	trending := make([]model.TrendingItem, 0, len(items))
	for _, item := range items {
		trending = append(trending, model.TrendingItem{
			Query: item.Member.(string),
			Count: int64(item.Score),
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": model.TrendingResponse{Searches: trending}})
}
