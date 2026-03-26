package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vpigadas/greek-tv-scraper/internal/model"
)

const keyPrefix = "greek-tv:"

type Store struct {
	client *redis.Client
	ttl    time.Duration
}

func New(addr, password string, db int, ttl time.Duration) *Store {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Store{client: rdb, ttl: ttl}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) SetSchedule(ctx context.Context, channelID, date string, progs []model.Programme) error {
	key := fmt.Sprintf("%sschedule:%s:%s", keyPrefix, channelID, date)
	data, err := json.Marshal(progs)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, data, s.ttl).Err()
}

func (s *Store) GetSchedule(ctx context.Context, channelID, date string) ([]model.Programme, error) {
	key := fmt.Sprintf("%sschedule:%s:%s", keyPrefix, channelID, date)
	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var progs []model.Programme
	return progs, json.Unmarshal(data, &progs)
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

func (s *Store) ListScheduleKeys(ctx context.Context, channelID string) ([]string, error) {
	pattern := fmt.Sprintf("%sschedule:%s:*", keyPrefix, channelID)
	return s.client.Keys(ctx, pattern).Result()
}
