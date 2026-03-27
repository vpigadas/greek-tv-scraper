package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vpigadas/greek-tv-scraper/internal/metrics"
	"github.com/vpigadas/greek-tv-scraper/internal/model"
)

const keyPrefix = "greek-tv:"

type Store struct {
	client    *redis.Client
	futureTTL time.Duration
	pastTTL   time.Duration
}

func New(addr, password string, db int, futureTTL, pastTTL time.Duration) *Store {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Store{client: rdb, futureTTL: futureTTL, pastTTL: pastTTL}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// SetSchedule stores programmes for a channel+broadcastDate with the appropriate TTL.
// isPast controls which TTL is used: true = 8 days (historical), false = 8h (fresh).
func (s *Store) SetSchedule(ctx context.Context, channelID, date string, progs []model.Programme, isPast bool) error {
	key := fmt.Sprintf("%sschedule:%s:%s", keyPrefix, channelID, date)
	data, err := json.Marshal(progs)
	if err != nil {
		return err
	}
	ttl := s.futureTTL
	if isPast {
		ttl = s.pastTTL
	}
	start := time.Now()
	err = s.client.Set(ctx, key, data, ttl).Err()
	metrics.RedisLatency.WithLabelValues("set").Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.RedisOperations.WithLabelValues("set", "error").Inc()
	} else {
		metrics.RedisOperations.WithLabelValues("set", "success").Inc()
	}
	return err
}

// GetSchedule retrieves the programme list for a channel+broadcastDate.
func (s *Store) GetSchedule(ctx context.Context, channelID, date string) ([]model.Programme, error) {
	key := fmt.Sprintf("%sschedule:%s:%s", keyPrefix, channelID, date)
	start := time.Now()
	data, err := s.client.Get(ctx, key).Bytes()
	metrics.RedisLatency.WithLabelValues("get").Observe(time.Since(start).Seconds())
	if err == redis.Nil {
		metrics.RedisOperations.WithLabelValues("get", "success").Inc()
		return nil, nil
	}
	if err != nil {
		metrics.RedisOperations.WithLabelValues("get", "error").Inc()
		return nil, err
	}
	metrics.RedisOperations.WithLabelValues("get", "success").Inc()
	var progs []model.Programme
	return progs, json.Unmarshal(data, &progs)
}

// GetScheduleRange fetches multiple days for a channel in a single Redis pipeline round-trip.
func (s *Store) GetScheduleRange(ctx context.Context, channelID string, dates []string) (map[string][]model.Programme, error) {
	pipe := s.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(dates))
	for _, date := range dates {
		key := fmt.Sprintf("%sschedule:%s:%s", keyPrefix, channelID, date)
		cmds[date] = pipe.Get(ctx, key)
	}
	start := time.Now()
	_, err := pipe.Exec(ctx)
	metrics.RedisLatency.WithLabelValues("pipeline").Observe(time.Since(start).Seconds())
	if err != nil && err != redis.Nil {
		metrics.RedisOperations.WithLabelValues("pipeline", "error").Inc()
		return nil, err
	}
	metrics.RedisOperations.WithLabelValues("pipeline", "success").Inc()

	result := make(map[string][]model.Programme, len(dates))
	for date, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			continue // key doesn't exist for this date
		}
		var progs []model.Programme
		if err := json.Unmarshal(data, &progs); err == nil {
			result[date] = progs
		}
	}
	return result, nil
}

// SetNowPlaying stores the pre-computed now-playing data with a short TTL.
func (s *Store) SetNowPlaying(ctx context.Context, playing []model.NowPlaying) error {
	data, err := json.Marshal(playing)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, keyPrefix+"now", data, 90*time.Second).Err()
}

// GetNowPlaying retrieves the pre-computed now-playing data.
func (s *Store) GetNowPlaying(ctx context.Context) ([]model.NowPlaying, error) {
	data, err := s.client.Get(ctx, keyPrefix+"now").Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var playing []model.NowPlaying
	return playing, json.Unmarshal(data, &playing)
}

func (s *Store) SetLastRefresh(ctx context.Context) error {
	key := keyPrefix + "last-refresh"
	return s.client.Set(ctx, key, time.Now().UTC().Format(time.RFC3339), 0).Err()
}

func (s *Store) GetLastRefresh(ctx context.Context) (string, error) {
	key := keyPrefix + "last-refresh"
	v, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}

// BroadcastDate returns the broadcast day for a given time.
// Greek TV broadcast day runs 06:00 → 06:00 next day.
// A show at 02:00 on Tuesday belongs to Monday's broadcast day.
func BroadcastDate(t time.Time, athens *time.Location) string {
	local := t.In(athens)
	if local.Hour() < 6 {
		local = local.Add(-24 * time.Hour)
	}
	return local.Format("2006-01-02")
}
