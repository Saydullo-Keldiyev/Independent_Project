package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/auction-system/search-service/internal/model"
)

// Search executes a full-text search with filters and sorting
func Search(ctx context.Context, index string, req model.SearchRequest) (*model.SearchResponse, error) {
	query := buildSearchQuery(req)

	body, _ := json.Marshal(query)

	res, err := Client.Search(
		Client.Search.WithContext(ctx),
		Client.Search.WithIndex(index),
		Client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("ES search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES search error: %s", res.String())
	}

	return parseSearchResponse(res.Body, req.Page, req.PageSize)
}

// Suggest returns autocomplete suggestions
func Suggest(ctx context.Context, index, prefix string, limit int) ([]string, error) {
	query := map[string]any{
		"size": 0,
		"suggest": map[string]any{
			"title-suggest": map[string]any{
				"prefix": prefix,
				"completion": map[string]any{
					"field":           "title.autocomplete",
					"size":            limit,
					"skip_duplicates": true,
				},
			},
		},
	}

	// Fallback: use match on autocomplete field
	query = map[string]any{
		"size": limit,
		"_source": []string{"title", "auction_id", "category"},
		"query": map[string]any{
			"match": map[string]any{
				"title.autocomplete": map[string]any{
					"query":    prefix,
					"operator": "and",
				},
			},
		},
	}

	body, _ := json.Marshal(query)

	res, err := Client.Search(
		Client.Search.WithContext(ctx),
		Client.Search.WithIndex(index),
		Client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("suggest error: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Title string `json:"title"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	suggestions := make([]string, 0, len(result.Hits.Hits))
	seen := make(map[string]bool)
	for _, hit := range result.Hits.Hits {
		if !seen[hit.Source.Title] {
			suggestions = append(suggestions, hit.Source.Title)
			seen[hit.Source.Title] = true
		}
	}

	return suggestions, nil
}

// IndexDocument indexes or updates an auction document
func IndexDocument(ctx context.Context, index string, doc model.AuctionDocument) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	res, err := Client.Index(
		index,
		bytes.NewReader(body),
		Client.Index.WithContext(ctx),
		Client.Index.WithDocumentID(doc.AuctionID),
	)
	if err != nil {
		return fmt.Errorf("index document failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index error: %s", res.String())
	}

	return nil
}

// DeleteDocument removes an auction from the index
func DeleteDocument(ctx context.Context, index, auctionID string) error {
	res, err := Client.Delete(
		index,
		auctionID,
		Client.Delete.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// ── Query builder ─────────────────────────────────────────────────────────────

func buildSearchQuery(req model.SearchRequest) map[string]any {
	must := []any{}
	filter := []any{}

	// Full-text search with fuzzy matching
	if req.Query != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query":     req.Query,
				"fields":    []string{"title^3", "description", "category^2", "tags^2"},
				"fuzziness": "AUTO",
				"type":      "best_fields",
			},
		})
	}

	// Filters
	if req.Category != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{"category": req.Category},
		})
	}
	if req.State != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{"state": req.State},
		})
	} else {
		// Default: only show active auctions
		filter = append(filter, map[string]any{
			"term": map[string]any{"state": "active"},
		})
	}
	if req.MinPrice > 0 {
		filter = append(filter, map[string]any{
			"range": map[string]any{"current_price": map[string]any{"gte": req.MinPrice}},
		})
	}
	if req.MaxPrice > 0 {
		filter = append(filter, map[string]any{
			"range": map[string]any{"current_price": map[string]any{"lte": req.MaxPrice}},
		})
	}

	// If no query, match all
	if len(must) == 0 {
		must = append(must, map[string]any{"match_all": map[string]any{}})
	}

	// Sorting
	sort := buildSort(req.SortBy)

	// Pagination
	from := 0
	size := req.PageSize
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if req.Page > 1 {
		from = (req.Page - 1) * size
	}

	return map[string]any{
		"from": from,
		"size": size,
		"sort": sort,
		"query": map[string]any{
			"bool": map[string]any{
				"must":   must,
				"filter": filter,
			},
		},
	}
}

func buildSort(sortBy string) []any {
	switch sortBy {
	case "price_asc":
		return []any{map[string]any{"current_price": "asc"}}
	case "price_desc":
		return []any{map[string]any{"current_price": "desc"}}
	case "newest":
		return []any{map[string]any{"created_at": "desc"}}
	case "ending_soon":
		return []any{map[string]any{"end_at": "asc"}}
	case "most_bids":
		return []any{map[string]any{"total_bids": "desc"}}
	default:
		// Relevance score + recency
		return []any{"_score", map[string]any{"created_at": "desc"}}
	}
}

// ── Response parser ───────────────────────────────────────────────────────────

func parseSearchResponse(body io.Reader, page, pageSize int) (*model.SearchResponse, error) {
	var raw struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source model.AuctionDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	hits := make([]model.AuctionDocument, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		hits = append(hits, h.Source)
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	return &model.SearchResponse{
		Hits:     hits,
		Total:    raw.Hits.Total.Value,
		Page:     page,
		PageSize: pageSize,
		Took:     raw.Took,
	}, nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

// needed for mappings.go
func init() {
	_ = time.Now // avoid unused import
}
