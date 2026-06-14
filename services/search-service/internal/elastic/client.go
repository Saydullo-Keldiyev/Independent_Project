package elastic

import (
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

var Client *elasticsearch.Client

// Connect initializes the Elasticsearch client
func Connect(url string, log *zap.Logger) error {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Ping to verify connection
	res, err := client.Info()
	if err != nil {
		return fmt.Errorf("ES connection failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES info error: %s", res.String())
	}

	Client = client
	log.Info("✅ Elasticsearch connected", zap.String("url", url))
	return nil
}
