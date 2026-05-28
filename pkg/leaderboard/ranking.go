package leaderboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	GlobalLeaderboardKey = "leaderboard:global"
	MetadataKeyPrefix    = "submission:metadata:"
)

// LeaderboardEntry represents a single contestant's rank entry on the leaderboard.
type LeaderboardEntry struct {
	SubmissionID string    `json:"submission_id"`
	ContestantID string    `json:"contestant_id"`
	TPS          float64   `json:"tps"`
	P99Latency   float64   `json:"p99_latency_ms"`
	SuccessRate  float64   `json:"success_rate"`
	Score        float64   `json:"score"`
	Timestamp    time.Time `json:"timestamp"`
}

// UpdateScore calculates the composite score for a submission and saves it to Redis.
func UpdateScore(ctx context.Context, rdb *redis.Client, submissionID string, contestantID string, tps float64, p99Latency float64, successRate float64) (float64, error) {
	// Formula: Score = TPS * (1000 / p99Latency)
	// Penalize failed runs by scaling the score down by the success rate
	score := 0.0
	if successRate > 0 && p99Latency > 0 {
		score = tps * (1000.0 / p99Latency) * (successRate / 100.0)
	}

	// 1. Add to the Global Sorted Set (ZSET)
	err := rdb.ZAdd(ctx, GlobalLeaderboardKey, redis.Z{
		Score:  score,
		Member: submissionID,
	}).Err()
	if err != nil {
		return 0, fmt.Errorf("failed to add ZSET entry: %w", err)
	}

	// 2. Save detailed metadata as a Hash
	metaKey := MetadataKeyPrefix + submissionID
	meta := map[string]interface{}{
		"submission_id": submissionID,
		"contestant_id": contestantID,
		"tps":           strconv.FormatFloat(tps, 'f', 2, 64),
		"p99_latency":   strconv.FormatFloat(p99Latency, 'f', 2, 64),
		"success_rate":  strconv.FormatFloat(successRate, 'f', 2, 64),
		"score":         strconv.FormatFloat(score, 'f', 2, 64),
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	err = rdb.HSet(ctx, metaKey, meta).Err()
	if err != nil {
		return 0, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Set TTL to 7 days so we don't accumulate junk forever
	rdb.Expire(ctx, metaKey, 7*24*time.Hour)

	// 3. Publish an update event to Redis Pub/Sub so active WS clients get real-time refreshes
	eventData, err := json.Marshal(map[string]interface{}{
		"type":          "new_run",
		"submission_id": submissionID,
		"contestant_id": contestantID,
		"score":         score,
	})
	if err == nil {
		rdb.Publish(ctx, "leaderboard:updates", string(eventData))
	}

	return score, nil
}

// GetLeaderboard fetches the top N entries from the Redis ZSET.
func GetLeaderboard(ctx context.Context, rdb *redis.Client, limit int64) ([]LeaderboardEntry, error) {
	// Fetch top members with scores from ZSET in descending order
	zRange, err := rdb.ZRevRangeWithScores(ctx, GlobalLeaderboardKey, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ZSET: %w", err)
	}

	entries := make([]LeaderboardEntry, 0, len(zRange))
	for _, z := range zRange {
		submissionID, ok := z.Member.(string)
		if !ok {
			continue
		}

		metaKey := MetadataKeyPrefix + submissionID
		metaMap, err := rdb.HGetAll(ctx, metaKey).Result()
		if err != nil || len(metaMap) == 0 {
			// If metadata was evicted but ZSET entry exists, create a basic fallback
			entries = append(entries, LeaderboardEntry{
				SubmissionID: submissionID,
				ContestantID: "Unknown",
				Score:        z.Score,
			})
			continue
		}

		tps, _ := strconv.ParseFloat(metaMap["tps"], 64)
		p99, _ := strconv.ParseFloat(metaMap["p99_latency"], 64)
		sr, _ := strconv.ParseFloat(metaMap["success_rate"], 64)
		score, _ := strconv.ParseFloat(metaMap["score"], 64)
		ts, _ := time.Parse(time.RFC3339, metaMap["timestamp"])

		entries = append(entries, LeaderboardEntry{
			SubmissionID: submissionID,
			ContestantID: metaMap["contestant_id"],
			TPS:          tps,
			P99Latency:   p99,
			SuccessRate:  sr,
			Score:        score,
			Timestamp:    ts,
		})
	}

	return entries, nil
}
