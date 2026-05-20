// Package cache provides a generic TTL cache with freshness tagging and
// singleflight request coalescing.
package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

// FreshnessTag describes how fresh a cached value is.
type FreshnessTag string

const (
	// FreshnessLive means the value was fetched from the upstream on this call.
	FreshnessLive FreshnessTag = "live"
	// FreshnessCached means the value came from an unexpired cache entry.
	FreshnessCached FreshnessTag = "cached"
	// FreshnessStaleFallback means the upstream call failed and an expired-but-
	// within-StaleWindow entry was served instead.
	FreshnessStaleFallback FreshnessTag = "stale_fallback"
)

// Response wraps a cached value with metadata about its freshness.
type Response[T any] struct {
	Data      T
	Freshness FreshnessTag
	FetchedAt time.Time
	SourceTTL time.Duration
}

// Config controls TTL and stale-serving behaviour.
type Config struct {
	// TTL is how long an entry is considered fresh.
	TTL time.Duration
	// StaleWindow is how long past TTL a failed upstream fetch may still be
	// served from the stale entry.
	StaleWindow time.Duration
}

// entry is an internal cache record.
type entry[T any] struct {
	value     T
	fetchedAt time.Time
	expiresAt time.Time
}

// fetchResult is the value type shared through singleflight.
type fetchResult[T any] struct {
	data      T
	fetchedAt time.Time
	freshness FreshnessTag
}

// Cache is a generic TTL cache. Construct with New.
//
// There is no background eviction goroutine: entries remain in memory until
// they are invalidated or overwritten. For this server's bounded reference
// space (one entry per DSV shipment ID) that is an acceptable trade-off in v1.
type Cache[T any] struct {
	cfg    Config
	logger *slog.Logger

	mu    sync.RWMutex
	items map[string]entry[T]

	sf singleflight.Group
}

// New constructs a Cache with the given Config and logger.
func New[T any](cfg Config, logger *slog.Logger) *Cache[T] {
	return &Cache[T]{
		cfg:    cfg,
		logger: logger,
		items:  make(map[string]entry[T]),
	}
}

// Fetch returns a Response[T] tagged with freshness.
//
//   - On a cache hit (within TTL): returns FreshnessCached without calling fetcher.
//   - On a miss or stale entry: calls fetcher. On success, caches the result and
//     returns FreshnessLive.
//   - On fetcher error with a stale-but-within-StaleWindow entry: returns the
//     stale value tagged FreshnessStaleFallback and logs a warning.
//   - On fetcher error with no usable stale entry: returns the error.
//
// Concurrent Fetch calls for the same key are coalesced via singleflight.
func (c *Cache[T]) Fetch(ctx context.Context, key string, fetcher func(context.Context) (T, error)) (Response[T], error) {
	now := time.Now()

	// Fast path: return a fresh (within-TTL) cached entry.
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if ok && now.Before(e.expiresAt) {
		return Response[T]{
			Data:      e.value,
			Freshness: FreshnessCached,
			FetchedAt: e.fetchedAt,
			SourceTTL: c.cfg.TTL,
		}, nil
	}

	// Slow path: coalesce concurrent fetches via singleflight.
	ch := c.sf.DoChan(key, func() (any, error) {
		val, fetchErr := fetcher(ctx)
		if fetchErr == nil {
			ts := time.Now()
			c.mu.Lock()
			c.items[key] = entry[T]{value: val, fetchedAt: ts, expiresAt: ts.Add(c.cfg.TTL)}
			c.mu.Unlock()
			return fetchResult[T]{data: val, fetchedAt: ts, freshness: FreshnessLive}, nil
		}

		// Fetcher failed — try to serve a stale entry within the stale window.
		staleNow := time.Now()
		c.mu.RLock()
		stale, hasStale := c.items[key]
		c.mu.RUnlock()
		if hasStale && staleNow.After(stale.expiresAt) && staleNow.Before(stale.expiresAt.Add(c.cfg.StaleWindow)) {
			c.logger.Warn("upstream fetch failed; serving stale cache entry",
				"key", key,
				"request_id", obs.RequestIDFromContext(ctx),
				"error", fetchErr,
			)
			return fetchResult[T]{data: stale.value, fetchedAt: stale.fetchedAt, freshness: FreshnessStaleFallback}, nil
		}

		var zero T
		return fetchResult[T]{data: zero}, fetchErr
	})

	select {
	case <-ctx.Done():
		return Response[T]{}, ctx.Err()
	case sfr := <-ch:
		if sfr.Err != nil {
			return Response[T]{}, sfr.Err
		}
		fr := sfr.Val.(fetchResult[T])
		return Response[T]{
			Data:      fr.data,
			Freshness: fr.freshness,
			FetchedAt: fr.fetchedAt,
			SourceTTL: c.cfg.TTL,
		}, nil
	}
}

// SetWithTTL stores value under key with a caller-supplied TTL, overriding
// any TTL stored in Config. Use this when different entries should expire at
// different rates — for example, a Delivered shipment (immutable) can be
// cached for 24 h while an in-transit shipment uses the default 30 s TTL.
//
// SetWithTTL always overwrites any existing entry for the key.
func (c *Cache[T]) SetWithTTL(key string, value T, ttl time.Duration) {
	now := time.Now()
	c.mu.Lock()
	c.items[key] = entry[T]{value: value, fetchedAt: now, expiresAt: now.Add(ttl)}
	c.mu.Unlock()
}

// Invalidate removes the entry for key. Safe for concurrent use.
func (c *Cache[T]) Invalidate(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}
