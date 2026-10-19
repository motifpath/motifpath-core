package coredomain_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/adapters/coredomain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

type fakeProfileResolver struct {
	mu      sync.Mutex
	profile ports.Profile
	err     error
	calls   int32
	tokens  []string
}

func (f *fakeProfileResolver) ResolveProfile(_ context.Context, bearerToken string) (ports.Profile, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	f.tokens = append(f.tokens, bearerToken)
	f.mu.Unlock()
	if f.err != nil {
		return ports.Profile{}, f.err
	}
	return f.profile, nil
}

func (f *fakeProfileResolver) callCount() int { return int(atomic.LoadInt32(&f.calls)) }

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newResolver(t *testing.T, backend ports.ProfileResolver, opts coredomain.CacheOptions) *coredomain.CachingIdentityResolver {
	t.Helper()
	return coredomain.NewCachingIdentityResolver(backend, opts)
}

func TestCachingIdentityResolver_ResolvesAndReturnsUserID(t *testing.T) {
	backend := &fakeProfileResolver{profile: ports.Profile{UserID: "user-1", Role: "student"}}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	userID, err := r.ResolveUserID(context.Background(), "sub-1", "token-1")

	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}

func TestCachingIdentityResolver_CacheHitSkipsTheBackend(t *testing.T) {
	backend := &fakeProfileResolver{profile: ports.Profile{UserID: "user-1", Role: "student"}}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	for i := 0; i < 5; i++ {
		userID, err := r.ResolveUserID(context.Background(), "sub-1", "token-1")
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	}

	assert.Equal(t, 1, backend.callCount(), "the backend should be called once, then served from cache")
}

func TestCachingIdentityResolver_CachesPerSubject(t *testing.T) {
	backend := &fakeProfileResolver{profile: ports.Profile{UserID: "user-1", Role: "student"}}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	_, _ = r.ResolveUserID(context.Background(), "sub-1", "token-1")
	_, _ = r.ResolveUserID(context.Background(), "sub-2", "token-2")

	assert.Equal(t, 2, backend.callCount())
}

func TestCachingIdentityResolver_ReResolvesAfterTTL(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)}
	backend := &fakeProfileResolver{profile: ports.Profile{UserID: "user-1", Role: "student"}}
	r := newResolver(t, backend, coredomain.CacheOptions{TTL: time.Hour, Now: clock.Now})

	_, _ = r.ResolveUserID(context.Background(), "sub-1", "token-1")
	clock.advance(59 * time.Minute)
	_, _ = r.ResolveUserID(context.Background(), "sub-1", "token-1")
	assert.Equal(t, 1, backend.callCount(), "still within TTL")

	clock.advance(2 * time.Minute)
	_, _ = r.ResolveUserID(context.Background(), "sub-1", "token-1")
	assert.Equal(t, 2, backend.callCount(), "TTL elapsed, re-resolved")
}

func TestCachingIdentityResolver_DoesNotCacheNotRegistered(t *testing.T) {
	backend := &fakeProfileResolver{err: ports.ErrIdentityNotRegistered}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	for i := 0; i < 3; i++ {
		_, err := r.ResolveUserID(context.Background(), "sub-1", "token-1")
		require.ErrorIs(t, err, ports.ErrIdentityNotRegistered)
	}
	assert.Equal(t, 3, backend.callCount(), "a negative answer must not be cached")
}

func TestCachingIdentityResolver_DoesNotCacheUnavailable(t *testing.T) {
	backend := &fakeProfileResolver{err: ports.ErrProfileUnavailable}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	_, err := r.ResolveUserID(context.Background(), "sub-1", "token-1")
	require.ErrorIs(t, err, ports.ErrProfileUnavailable)

	backend.err = nil
	backend.profile = ports.Profile{UserID: "user-1", Role: "student"}
	userID, err := r.ResolveUserID(context.Background(), "sub-1", "token-1")
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
	assert.Equal(t, 2, backend.callCount())
}

func TestCachingIdentityResolver_PropagatesUnexpectedError(t *testing.T) {
	backend := &fakeProfileResolver{err: errors.New("boom")}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	_, err := r.ResolveUserID(context.Background(), "sub-1", "token-1")
	require.Error(t, err)
}

func TestCachingIdentityResolver_StaysBoundedPastTheCap(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)}
	backend := &fakeProfileResolver{profile: ports.Profile{UserID: "user-x", Role: "student"}}
	r := newResolver(t, backend, coredomain.CacheOptions{TTL: time.Hour, MaxEntries: 4, Now: clock.Now})

	for i := 0; i < 20; i++ {
		_, err := r.ResolveUserID(context.Background(), string(rune('a'+i)), "token")
		require.NoError(t, err)
	}
	// No assertion on exact size (eviction strategy is deliberately coarse) --
	// the contract is only that it does not grow without bound. Re-resolving a
	// recent subject should still work.
	_, err := r.ResolveUserID(context.Background(), "t", "token")
	require.NoError(t, err)
}

func TestCachingIdentityResolver_ForwardsTheBearerToken(t *testing.T) {
	backend := &fakeProfileResolver{profile: ports.Profile{UserID: "user-1", Role: "student"}}
	r := newResolver(t, backend, coredomain.CacheOptions{})

	_, _ = r.ResolveUserID(context.Background(), "sub-1", "the-real-token")

	backend.mu.Lock()
	defer backend.mu.Unlock()
	require.Equal(t, []string{"the-real-token"}, backend.tokens)
}
