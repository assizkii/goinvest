package invest

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrCacheMiss is a replacement for implementation defined cache miss error of different providers,
	// such as replacement for redis.Nil which looks cryptic.
	ErrCacheMiss = errors.New("invest/cache: key not found")
)

// Cache is an interface which abstracts cache providers.
type Cache interface {

	// Get retrieves value from cache
	// returns ErrCacheMiss (if everything is ok, but value is not in cache)
	// or implementation defined error in case of problem.
	Get(ctx context.Context, key string, ptrValue interface{}) error

	// Set just sets the given key/value in the cache, overwriting any existing value
	// associated with that key.
	Set(ctx context.Context, key string, ptrValue interface{}, expires time.Duration) error
}

// CacheCredentials is an option structure for configuring real cache implementation.
type CacheCredentials struct {
	Active string `yaml:"active"`
	Redis  struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		PoolSize int    `yaml:"poolSize"`
	} `yaml:"redis"`
}
