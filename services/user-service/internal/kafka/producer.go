package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

var Writer *kafka.Writer

type ProducerConfig struct {
	Brokers []string
	Topic   string
}

func InitProducer(cfg ProducerConfig) {
	Writer = &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		Async:        false,
	}
}

func PublishEvent(event UserEvent) error {
	if Writer == nil {
		return fmt.Errorf("kafka writer not initialized")
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return Writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.UserID),
		Value: data,
		Time:  time.Now(),
	})
}

func Close() error {
	if Writer != nil {
		return Writer.Close()
	}
	return nil
}
