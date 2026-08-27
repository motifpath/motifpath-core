package coredomain

import (
	"context"
	"sync"
	"time"

	"github.com/motifpath/event-ingestion/internal/ports"
)

const (
	defaultIdentityTTL        = time.Hour
	defaultIdentityMaxEntries = 10_000
)

// CacheOptions tunes a CachingIdentityResolver. Zero values fall back to the
// package defaults; Now defaults to time.Now.
type CacheOptions struct {
	TTL        time.Duration
	MaxEntries int
	Now        func() time.Time
}

// CachingIdentityResolver resolves a caller's MotifPath user id through a
// ports.ProfileResolver, caching the result keyed on the token subject.
//
// Per ADR-014 the sub -> user_id mapping is a registration-time invariant, so a
// cache hit can never be stale. The TTL and size cap exist only to bound
// memory. Errors are never cached: a "not registered" or "unavailable" answer
// is re-derived on the next request.
type CachingIdentityResolver struct {
	resolver   ports.ProfileResolver
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]identityCacheEntry
}

type identityCacheEntry struct {
	userID    string
	expiresAt time.Time
}

var _ ports.IdentityResolver = (*CachingIdentityResolver)(nil)

func NewCachingIdentityResolver(resolver ports.ProfileResolver, opts CacheOptions) *CachingIdentityResolver {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultIdentityTTL
	}
	maxEntries := opts.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultIdentityMaxEntries
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &CachingIdentityResolver{
		resolver:   resolver,
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        now,
		entries:    make(map[string]identityCacheEntry),
	}
}

// ResolveUserID returns the MotifPath user id for the caller whose token
// subject is sub, calling the underlying resolver only on a cache miss.
func (c *CachingIdentityResolver) ResolveUserID(ctx context.Context, sub, bearerToken string) (string, error) {
	if userID, ok := c.lookup(sub); ok {
		return userID, nil
	}

	profile, err := c.resolver.ResolveProfile(ctx, bearerToken)
	if err != nil {
		return "", err
	}

	c.store(sub, profile.UserID)
	return profile.UserID, nil
}

func (c *CachingIdentityResolver) lookup(sub string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[sub]
	if !ok || !entry.expiresAt.After(c.now()) {
		return "", false
	}
	return entry.userID, true
}

func (c *CachingIdentityResolver) store(sub, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxEntries {
		c.evictExpiredLocked()
		if len(c.entries) >= c.maxEntries {
			// Still full of live entries: reset rather than grow unbounded.
			// Unreachable at MVP scale (>10k distinct callers per instance
			// within one TTL window); the cost of a reset is just re-warming.
			c.entries = make(map[string]identityCacheEntry)
		}
	}

	c.entries[sub] = identityCacheEntry{userID: userID, expiresAt: c.now().Add(c.ttl)}
}

func (c *CachingIdentityResolver) evictExpiredLocked() {
	now := c.now()
	for sub, entry := range c.entries {
		if !entry.expiresAt.After(now) {
			delete(c.entries, sub)
		}
	}
}
