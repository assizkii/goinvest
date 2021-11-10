package redis

import (
	"context"
	"fmt"
	"goinvest/internal/invest"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

const (
	redisProvider              = "redis"
	defaultReconnectionTimeout = 5
	defaultIdleTimeout         = 55 * time.Second
	defaultIdleCheckFrequency  = 170 * time.Second
)

// ConnectLoop takes config and depending on cache section of config wraps actual cache implementation
// It trying to connect to cache in a loop because at least at dev environment service can be ready before cache is up.
func ConnectLoop(ctx context.Context, config invest.CacheCredentials, logger *zap.Logger) (cache invest.Cache, closeFunc func() error, err error) {
	switch activeCache := config.Active; activeCache {
	case redisProvider:
		return openRedisClient(ctx, config, logger)
	default:
		return openRedisClient(ctx, config, logger)
	}
}

// Cache wraps *redis.Client to meet swappable Cache interface.
type Cache struct {
	client *redis.Client
}

// newRedisCache accept config and returns ready for usage cache among with its closer.
func openRedisClient(ctx context.Context, config invest.CacheCredentials, logger *zap.Logger) (redisCache *Cache, closeFunc func() error, err error) {

	client := redis.NewClient(&redis.Options{
		Addr:               config.Redis.Address,
		Password:           config.Redis.Password,
		PoolSize:           config.Redis.PoolSize,
		IdleTimeout:        defaultIdleTimeout,
		IdleCheckFrequency: defaultIdleCheckFrequency,
	})

	err = client.WithContext(ctx).Ping(ctx).Err()
	if nil == err {
		redisCache := &Cache{client: client}
		return redisCache, client.Close, nil
	}

	logger.Error("error when starting redis server", zap.Error(err))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	timeoutExceeded := time.After(time.Second * time.Duration(defaultReconnectionTimeout))

	for {

		select {

		case <-timeoutExceeded:
			return nil, nil, fmt.Errorf("cache connection failed after %d timeout", defaultReconnectionTimeout)

		case <-ticker.C:
			err := client.Ping(ctx).Err()
			if nil == err {
				redisCache := &Cache{client: client}
				return redisCache, redisCache.client.Close, nil
			}
			logger.Error("error when starting redis server", zap.Error(err))

		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
}

// Get retrieves value from Redis and serializes to pointer value.
func (c *Cache) Get(ctx context.Context, key string, ptrValue interface{}) error {
	b, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return invest.ErrCacheMiss
		}
		return fmt.Errorf("problem while trying to get value from cache: %w", err)
	}
	return Deserialize(b, ptrValue)
}

// Set takes key and value as input and setting Redis cache with this value.
func (c *Cache) Set(ctx context.Context, key string, ptrValue interface{}, expires time.Duration) error {

	b, err := Serialize(ptrValue)
	if err != nil {
		return fmt.Errorf("problem while trying to serialize value while setting in cache: %w", err)
	}

	if err := c.client.Set(ctx, key, b, expires).Err(); err != nil {
		return fmt.Errorf("problem while trying to set value in cache: %w", err)
	}

	return nil
}
