package queue

import (
	"encoding/json"
	"time"
)

// TelemetryEvent is the Go data model matching the schema defined in proto/events.proto.
type TelemetryEvent struct {
	SubmissionID string    `json:"submission_id"`
	ContestantID string    `json:"contestant_id"`
	TPS          float64   `json:"tps"`
	P99LatencyMS float64   `json:"p99_latency_ms"`
	SuccessRate  float64   `json:"success_rate"`
	Timestamp    time.Time `json:"timestamp"`
}

// Serialize encodes the telemetry event into JSON binary bytes.
func (e *TelemetryEvent) Serialize() ([]byte, error) {
	return json.Marshal(e)
}

// DeserializeTelemetryEvent decodes JSON binary bytes into a TelemetryEvent struct.
func DeserializeTelemetryEvent(data []byte) (*TelemetryEvent, error) {
	var event TelemetryEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
