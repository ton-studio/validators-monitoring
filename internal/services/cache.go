package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	clients "validators-health/internal/clients/redis"

	"github.com/go-redis/redis/v8"
)

type CacheService struct {
	RedisClient *redis.Client
}

func NewCacheService() (*CacheService, error) {
	redisClientWrapper, err := clients.GetRedisClient()
	if err != nil {
		return nil, err
	}
	return &CacheService{RedisClient: redisClientWrapper.Client}, nil
}

func (c *CacheService) CacheData(key string, data interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.RedisClient.Set(context.Background(), key, jsonData, ttl).Err()
}

func (c *CacheService) GetCachedData(key string, result interface{}) (bool, error) {
	data, err := c.RedisClient.Get(context.Background(), key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	err = json.Unmarshal([]byte(data), result)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *CacheService) IncrementCounter(key string) (int64, error) {
	result, err := c.RedisClient.Incr(context.Background(), key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}
	return result, nil
}

func (c *CacheService) CacheChunkData(key string, data map[uint32]float64, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.RedisClient.Set(context.Background(), key, jsonData, ttl).Err()
}
