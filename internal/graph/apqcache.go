package graph

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/redis/go-redis/v9"
)

// APQCache stores persisted query strings in Redis, so every API replica
// recognises a hash that any other replica has seen.
type APQCache struct {
	client *redis.Client
	ttl    time.Duration
}

var _ graphql.Cache[string] = (*APQCache)(nil)

const apqKeyPrefix = "apq:"

func NewAPQCache(client *redis.Client, ttl time.Duration) *APQCache {
	return &APQCache{client: client, ttl: ttl}
}

// Get treats a Redis failure as a miss: the client then resends the full
// query, so an outage costs bandwidth, not availability.
func (c *APQCache) Get(ctx context.Context, key string) (string, bool) {
	s, err := c.client.Get(ctx, apqKeyPrefix+key).Result()
	if err != nil {
		return "", false
	}
	return s, true
}

func (c *APQCache) Add(ctx context.Context, key, value string) {
	_ = c.client.Set(ctx, apqKeyPrefix+key, value, c.ttl).Err()
}
