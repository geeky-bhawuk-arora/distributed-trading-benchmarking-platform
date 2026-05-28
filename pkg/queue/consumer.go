package queue

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

// ConsumerGroupListener wraps a kafka.Reader to consume events from Redpanda.
type ConsumerGroupListener struct {
	reader *kafka.Reader
}

// NewConsumer creates a new consumer group listener for telemetry topics.
func NewConsumer(brokers []string, groupID string) *ConsumerGroupListener {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    TelemetryTopic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	log.Printf("[consumer] Initialized Redpanda/Kafka reader for topic %s (Group: %s)", TelemetryTopic, groupID)
	return &ConsumerGroupListener{reader: reader}
}

// StartListening starts consuming messages in a blocking loop and triggers the handler callback.
func (c *ConsumerGroupListener) StartListening(ctx context.Context, handler func(event *TelemetryEvent) error) {
	log.Println("[consumer] Started listening for telemetry events...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[consumer] Stopping telemetry event listener (context cancelled)")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				// Don't log error if context was closed
				select {
				case <-ctx.Done():
					return
				default:
					log.Printf("[consumer] Error reading message: %v", err)
					continue
				}
			}

			event, err := DeserializeTelemetryEvent(msg.Value)
			if err != nil {
				log.Printf("[consumer] Error deserializing message value: %v", err)
				// Here is where a Dead Letter Queue (DLQ) message could be produced for fault tolerance
				continue
			}

			// Forward to custom handler (e.g. database saver or leaderboard updater)
			if err := handler(event); err != nil {
				log.Printf("[consumer] Error processing telemetry event: %v", err)
			}
		}
	}
}

// Close closes the underlying Kafka reader.
func (c *ConsumerGroupListener) Close() error {
	log.Println("[consumer] Closing Redpanda reader...")
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("failed to close Redpanda reader: %w", err)
	}
	return nil
}
