package elastic

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// AuctionMapping defines the Elasticsearch index mapping for auctions.
// Optimized for: full-text search, filtering, sorting, autocomplete.
const AuctionMapping = `{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "autocomplete_tokenizer",
          "filter": ["lowercase"]
        },
        "search_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "synonym_filter"]
        }
      },
      "tokenizer": {
        "autocomplete_tokenizer": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20,
          "token_chars": ["letter", "digit"]
        }
      },
      "filter": {
        "synonym_filter": {
          "type": "synonym",
          "synonyms": [
            "phone,smartphone,mobile",
            "laptop,notebook,computer",
            "car,automobile,vehicle"
          ]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "auction_id":    { "type": "keyword" },
      "title":         { "type": "text", "analyzer": "standard", "fields": { "autocomplete": { "type": "text", "analyzer": "autocomplete_analyzer", "search_analyzer": "standard" }, "keyword": { "type": "keyword" } } },
      "description":   { "type": "text" },
      "category":      { "type": "keyword" },
      "tags":          { "type": "keyword" },
      "seller_id":     { "type": "keyword" },
      "seller_name":   { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
      "start_price":   { "type": "double" },
      "current_price": { "type": "double" },
      "total_bids":    { "type": "integer" },
      "state":         { "type": "keyword" },
      "image_url":     { "type": "keyword", "index": false },
      "start_at":      { "type": "date" },
      "end_at":        { "type": "date" },
      "created_at":    { "type": "date" },
      "updated_at":    { "type": "date" }
    }
  }
}`

// EnsureIndex creates the index if it doesn't exist
func EnsureIndex(indexName string, log *zap.Logger) error {
	res, err := Client.Indices.Exists([]string{indexName})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		log.Info("ES index already exists", zap.String("index", indexName))
		return nil
	}

	// Create index with mapping
	res, err = Client.Indices.Create(
		indexName,
		Client.Indices.Create.WithBody(strings.NewReader(AuctionMapping)),
		Client.Indices.Create.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("create index error: %s", res.String())
	}

	log.Info("ES index created", zap.String("index", indexName))
	return nil
}
