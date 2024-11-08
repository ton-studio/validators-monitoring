package clients

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	Client *redis.Client
}

var (
	redisInstance *RedisClient
	redisOnce     sync.Once
	redisInitErr  error
)

func GetRedisClient() (*RedisClient, error) {
	redisOnce.Do(func() {
		maxRetries := 5
		initialBackoff := 1 * time.Second
		maxBackoff := 30 * time.Second

		var client *redis.Client
		var err error
		backoff := initialBackoff

		for attempt := 1; attempt <= maxRetries; attempt++ {
			client = redis.NewClient(&redis.Options{
				Addr:     os.Getenv("REDIS_ADDR"),
				Password: os.Getenv("REDIS_PASSWORD"),
				DB:       0,
				PoolSize: 10,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = client.Ping(ctx).Err()
			if err == nil {

				redisInstance = &RedisClient{Client: client}
				log.Println("Connected to Redis successfully.")
				return
			}

			log.Printf("Try connect to Redis %d/%d failed: %v", attempt, maxRetries, err)

			err := client.Close()
			if err != nil {
				log.Printf("Failed to close Redis connection: %v", err)
				return
			}

			if attempt == maxRetries {
				redisInitErr = err
				log.Printf("Coulnt't connect to redis, tries: %d error: %v", maxRetries, err)
				return
			}

			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	})

	return redisInstance, redisInitErr
}

func (r *RedisClient) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}
