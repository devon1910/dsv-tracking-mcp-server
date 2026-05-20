package cache_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fmt"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/cache"
)

// noopLogger returns a logger that discards all output, keeping test noise low.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(noopWriter{}, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

type noopWriter struct{}

func (noopWriter) Write(b []byte) (int, error) { return len(b), nil }

// newCache is a shortcut for creating test caches.
func newCache(ttl, stale time.Duration) *cache.Cache[string] {
	return cache.New[string](cache.Config{TTL: ttl, StaleWindow: stale}, noopLogger())
}

// okFetcher returns a fetcher that always succeeds with the given value,
// incrementing a counter on each call.
func okFetcher(val string, n *int32) func(context.Context) (string, error) {
	return func(_ context.Context) (string, error) {
		atomic.AddInt32(n, 1)
		return val, nil
	}
}

// errFetcher returns a fetcher that always fails.
func errFetcher(err error) func(context.Context) (string, error) {
	return func(_ context.Context) (string, error) { return "", err }
}

// TestCacheHit verifies that a cached (within-TTL) entry is served without
// invoking the fetcher.
func TestCacheHit(t *testing.T) {
	c := newCache(time.Minute, time.Minute)
	ctx := context.Background()

	var calls int32
	f := okFetcher("hello", &calls)

	// Prime the cache.
	r1, err := c.Fetch(ctx, "k", f)
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if r1.Freshness != cache.FreshnessLive {
		t.Errorf("first Fetch freshness = %q, want live", r1.Freshness)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 fetcher call, got %d", calls)
	}

	// Second fetch must hit the cache.
	r2, err := c.Fetch(ctx, "k", f)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if r2.Freshness != cache.FreshnessCached {
		t.Errorf("second Fetch freshness = %q, want cached", r2.Freshness)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("fetcher called again on cache hit, want exactly 1 call total")
	}
	if r2.Data != "hello" {
		t.Errorf("Data = %q, want %q", r2.Data, "hello")
	}
}

// TestCacheMiss verifies that a cold cache calls the fetcher and returns FreshnessLive.
func TestCacheMiss(t *testing.T) {
	c := newCache(time.Minute, time.Minute)
	ctx := context.Background()

	var calls int32
	r, err := c.Fetch(ctx, "k", okFetcher("world", &calls))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if r.Freshness != cache.FreshnessLive {
		t.Errorf("freshness = %q, want live", r.Freshness)
	}
	if r.Data != "world" {
		t.Errorf("Data = %q, want world", r.Data)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("fetcher calls = %d, want 1", calls)
	}
}

// TestExpiredEntryFetcherSuccess verifies that an expired entry triggers a
// fresh fetch, replaces the entry, and returns FreshnessLive.
func TestExpiredEntryFetcherSuccess(t *testing.T) {
	const ttl = 20 * time.Millisecond
	c := newCache(ttl, time.Second)
	ctx := context.Background()

	var calls int32
	f := okFetcher("v1", &calls)

	if _, err := c.Fetch(ctx, "k", f); err != nil {
		t.Fatal(err)
	}

	time.Sleep(ttl + 10*time.Millisecond) // let the entry expire

	r, err := c.Fetch(ctx, "k", okFetcher("v2", &calls))
	if err != nil {
		t.Fatalf("Fetch after expiry: %v", err)
	}
	if r.Freshness != cache.FreshnessLive {
		t.Errorf("freshness = %q, want live", r.Freshness)
	}
	if r.Data != "v2" {
		t.Errorf("Data = %q, want v2", r.Data)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("fetcher calls = %d, want 2", calls)
	}
}

// TestStaleFallbackWithinWindow verifies that when the entry is expired and
// within the stale window, a failed fetch returns the stale value tagged
// FreshnessStaleFallback.
func TestStaleFallbackWithinWindow(t *testing.T) {
	const ttl = 20 * time.Millisecond
	const staleWindow = 200 * time.Millisecond
	c := newCache(ttl, staleWindow)
	ctx := context.Background()

	// Prime with a known value.
	if _, err := c.Fetch(ctx, "k", okFetcher("stale-value", new(int32))); err != nil {
		t.Fatal(err)
	}

	time.Sleep(ttl + 10*time.Millisecond) // expire the entry

	sentinelErr := errors.New("upstream down")
	r, err := c.Fetch(ctx, "k", errFetcher(sentinelErr))
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if r.Freshness != cache.FreshnessStaleFallback {
		t.Errorf("freshness = %q, want stale_fallback", r.Freshness)
	}
	if r.Data != "stale-value" {
		t.Errorf("Data = %q, want stale-value", r.Data)
	}
}

// TestStaleFallbackBeyondWindow verifies that when the entry has been stale
// longer than StaleWindow, the fetcher's error is returned.
func TestStaleFallbackBeyondWindow(t *testing.T) {
	const ttl = 10 * time.Millisecond
	const staleWindow = 10 * time.Millisecond
	c := newCache(ttl, staleWindow)
	ctx := context.Background()

	if _, err := c.Fetch(ctx, "k", okFetcher("old-value", new(int32))); err != nil {
		t.Fatal(err)
	}

	// Sleep past both TTL and stale window.
	time.Sleep(ttl + staleWindow + 20*time.Millisecond)

	sentinelErr := errors.New("upstream down")
	_, err := c.Fetch(ctx, "k", errFetcher(sentinelErr))
	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// TestSingleflight verifies that 100 concurrent Fetch calls for the same key
// invoke the fetcher exactly once.
func TestSingleflight(t *testing.T) {
	c := newCache(time.Minute, time.Minute)
	ctx := context.Background()

	var callCount int32
	// unblock is closed by the test after all goroutines have had time to join
	// the singleflight group, ensuring only one fetcher goroutine runs.
	unblock := make(chan struct{})

	fetcher := func(_ context.Context) (string, error) {
		atomic.AddInt32(&callCount, 1)
		<-unblock
		return "shared", nil
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			r, err := c.Fetch(ctx, "sf-key", fetcher)
			if err != nil {
				t.Errorf("Fetch error: %v", err)
			}
			if r.Data != "shared" {
				t.Errorf("Data = %q, want shared", r.Data)
			}
		}()
	}

	// Give goroutines time to block inside DoChan before releasing the fetcher.
	time.Sleep(30 * time.Millisecond)
	close(unblock)
	wg.Wait()

	if n := atomic.LoadInt32(&callCount); n != 1 {
		t.Errorf("fetcher called %d times, want 1", n)
	}
}

// TestErrorChain_NoStaleEntry verifies that errors.Is works on errors returned
// by Fetch when there is no usable stale entry. The cache must not wrap the
// fetcher's error — callers rely on errors.Is to discriminate sentinel values
// (e.g. domain.ErrShipmentNotFound) without string-matching.
func TestErrorChain_NoStaleEntry(t *testing.T) {
	// Use a tiny stale window so both TTL AND stale window expire quickly.
	const ttl = 10 * time.Millisecond
	const staleWindow = 5 * time.Millisecond
	c := newCache(ttl, staleWindow)
	ctx := context.Background()

	// Prime the cache.
	if _, err := c.Fetch(ctx, "k", func(context.Context) (string, error) { return "v", nil }); err != nil {
		t.Fatal(err)
	}

	// Wait past both TTL and stale window.
	time.Sleep(ttl + staleWindow + 20*time.Millisecond)

	// Simulate *domain.UpstreamError wrapping domain.ErrShipmentNotFound by
	// using fmt.Errorf("%w") to build a chain that errors.Is can walk.
	sentinel := errors.New("not found sentinel")
	wrappedErr := fmt.Errorf("upstream detail: %w", sentinel)

	_, fetchErr := c.Fetch(ctx, "k", func(context.Context) (string, error) {
		return "", wrappedErr
	})

	if fetchErr == nil {
		t.Fatal("expected error, got nil (stale window should have expired)")
	}
	if !errors.Is(fetchErr, sentinel) {
		t.Errorf("errors.Is(sentinel) = false; cache must not re-wrap fetcher errors.\ngot: %v (type %T)", fetchErr, fetchErr)
	}
}

// TestErrorChain_StaleEntryWithinWindow verifies that when a stale entry is
// within the stale window, Fetch returns the stale data with nil error even
// when the fetcher fails — the fetcher's error is deliberately discarded so
// the caller receives a useful stale response rather than an error.
func TestErrorChain_StaleEntryWithinWindow(t *testing.T) {
	const ttl = 10 * time.Millisecond
	const staleWindow = 200 * time.Millisecond
	c := newCache(ttl, staleWindow)
	ctx := context.Background()

	// Prime then let the TTL expire (but not the stale window).
	if _, _ = c.Fetch(ctx, "k", func(context.Context) (string, error) { return "stale", nil }); false {
	}
	time.Sleep(ttl + 5*time.Millisecond)

	sentinel := errors.New("upstream down")
	resp, err := c.Fetch(ctx, "k", func(context.Context) (string, error) {
		return "", sentinel
	})

	// In the stale-fallback path the cache swallows the fetcher error and
	// returns the stale value. There is no error to check errors.Is against.
	if err != nil {
		t.Errorf("expected nil error in stale-fallback path, got: %v", err)
	}
	if resp.Freshness != cache.FreshnessStaleFallback {
		t.Errorf("Freshness = %q, want stale_fallback", resp.Freshness)
	}
	if resp.Data != "stale" {
		t.Errorf("Data = %q, want stale", resp.Data)
	}
}

// TestContextCancellation verifies that a cancelled context propagates to the
// caller while the underlying singleflight fetch completes independently.
func TestContextCancellation(t *testing.T) {
	c := newCache(time.Minute, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())

	// Fetcher respects context cancellation.
	fetcher := func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			return "late", nil
		}
	}

	cancel() // cancel before calling Fetch
	_, err := c.Fetch(ctx, "k", fetcher)

	// Either the select in Fetch picked ctx.Done() or the fetcher returned
	// ctx.Err() — either way the error must be context.Canceled.
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
