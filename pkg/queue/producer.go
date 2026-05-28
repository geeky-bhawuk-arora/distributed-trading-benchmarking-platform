package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const TelemetryTopic = "run.telemetry"

// Producer wraps a kafka.Writer to publish metrics to Redpanda/Kafka topics.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a new Producer connecting to the specified broker addresses.
func NewProducer(brokers []string) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        TelemetryTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,              // Trigger batch write on 100 messages
		BatchTimeout: 10 * time.Millisecond, // Or write every 10ms (asynchronous/non-blocking)
		Async:        true,             // High throughput non-blocking writes
		RequiredAcks: kafka.RequireOne,
	}

	log.Printf("[producer] Initialized Redpanda/Kafka writer targeting topic: %s", TelemetryTopic)
	return &Producer{writer: writer}
}

// PublishTelemetry publishes a TelemetryEvent to the telemetry queue asynchronously.
func (p *Producer) PublishTelemetry(ctx context.Context, event *TelemetryEvent) error {
	payload, err := event.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Use submission_id as the partition key to ensure order of logs per run
	msg := kafka.Message{
		Key:   []byte(event.SubmissionID),
		Value: payload,
	}

	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to write message to Redpanda: %w", err)
	}

	return nil
}

// Close closes the underlying Kafka writer.
func (p *Producer) Close() error {
	log.Println("[producer] Closing Redpanda writer...")
	return p.writer.Close()
}
