// Package cache stores task list responses in Redis.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/example/task-management/backend/internal/domain"
)

// TaskCache is the cache contract the service depends on.
type TaskCache interface {
	// Version returns the current list version (0 if never bumped).
	Version(ctx context.Context) (int64, error)
	GetList(ctx context.Context, key string) (*domain.TaskPage, bool, error)
	SetList(ctx context.Context, key string, page *domain.TaskPage) error
	// Invalidate bumps the version, orphaning every list key built from an
	// older version. Orphans die on their own TTL; nothing is scanned or deleted.
	Invalidate(ctx context.Context) error
}

const VersionKey = "tasks:version"

// ListKey builds a deterministic key from the version and normalised params.
// url.Values.Encode sorts keys, so field order never changes the key.
func ListKey(version int64, p domain.ListParams) string {
	dir := "asc"
	if p.SortDesc {
		dir = "desc"
	}
	v := url.Values{}
	v.Set("status", string(p.Status))
	v.Set("keyword", p.Keyword)
	v.Set("assignee", p.Assignee)
	v.Set("page", strconv.Itoa(p.Page))
	v.Set("limit", strconv.Itoa(p.Limit))
	v.Set("sort", p.SortBy+":"+dir)
	return "tasks:list:v" + strconv.FormatInt(version, 10) + ":" + v.Encode()
}

type RedisTaskCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisTaskCache(rdb *redis.Client, ttl time.Duration) *RedisTaskCache {
	return &RedisTaskCache{rdb: rdb, ttl: ttl}
}

func (c *RedisTaskCache) Version(ctx context.Context) (int64, error) {
	v, err := c.rdb.Get(ctx, VersionKey).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return v, err
}

func (c *RedisTaskCache) GetList(ctx context.Context, key string) (*domain.TaskPage, bool, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var page domain.TaskPage
	if err := json.Unmarshal(b, &page); err != nil {
		return nil, false, err
	}
	return &page, true, nil
}

func (c *RedisTaskCache) SetList(ctx context.Context, key string, page *domain.TaskPage) error {
	b, err := json.Marshal(page)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, c.ttl).Err()
}

func (c *RedisTaskCache) Invalidate(ctx context.Context) error {
	return c.rdb.Incr(ctx, VersionKey).Err()
}
