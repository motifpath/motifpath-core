package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ConfigKey is one setting the service knows: its key, and how a value for it is checked before it
// is stored. Only registered keys exist; there is no free-form setting.
type ConfigKey interface {
	Key() string
	// Normalize checks raw against the key's schema and its data rules and returns the value to
	// store. A value that fails comes back as a *domain.ValidationError listing every problem.
	Normalize(ctx context.Context, raw []byte) ([]byte, error)
}

// AppConfigSetting is a known key with its stored entry; Entry is nil while the key was never set.
type AppConfigSetting struct {
	Key   string
	Entry *domain.AppConfigEntry
}

// AppConfigService reads and replaces the app's settings for admins. Clients other than admins
// never read a setting by key: each use reads its setting through its own service.
type AppConfigService struct {
	repo ports.AppConfigRepository
	now  func() time.Time
	keys map[string]ConfigKey
}

func NewAppConfigService(repo ports.AppConfigRepository, now func() time.Time, keys ...ConfigKey) *AppConfigService {
	registry := make(map[string]ConfigKey, len(keys))
	for _, k := range keys {
		registry[k.Key()] = k
	}
	return &AppConfigService{repo: repo, now: now, keys: registry}
}

// List returns every known key, set or not, ordered by key.
func (s *AppConfigService) List(ctx context.Context, caller domain.User) ([]AppConfigSetting, error) {
	if caller.Role != domain.RoleAdmin {
		return nil, domain.ErrForbidden
	}
	entries, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]domain.AppConfigEntry, len(entries))
	for _, e := range entries {
		byKey[e.Key] = e
	}
	settings := make([]AppConfigSetting, 0, len(s.keys))
	for key := range s.keys {
		setting := AppConfigSetting{Key: key}
		if e, ok := byKey[key]; ok {
			setting.Entry = &e
		}
		settings = append(settings, setting)
	}
	sort.Slice(settings, func(i, j int) bool { return settings[i].Key < settings[j].Key })
	return settings, nil
}

// Get returns one known key's setting; an unknown key is domain.ErrNotFound.
func (s *AppConfigService) Get(ctx context.Context, caller domain.User, key string) (AppConfigSetting, error) {
	if caller.Role != domain.RoleAdmin {
		return AppConfigSetting{}, domain.ErrForbidden
	}
	if _, ok := s.keys[key]; !ok {
		return AppConfigSetting{}, domain.ErrNotFound
	}
	return s.read(ctx, key)
}

// Set replaces a known key's whole value once the key accepts it, recording caller as the last to
// change it. A value the key refuses stores nothing.
func (s *AppConfigService) Set(ctx context.Context, caller domain.User, key string, raw []byte) (AppConfigSetting, error) {
	if caller.Role != domain.RoleAdmin {
		return AppConfigSetting{}, domain.ErrForbidden
	}
	k, ok := s.keys[key]
	if !ok {
		return AppConfigSetting{}, domain.ErrNotFound
	}
	value, err := k.Normalize(ctx, raw)
	if err != nil {
		return AppConfigSetting{}, err
	}
	entry := domain.AppConfigEntry{Key: key, Value: value, UpdatedAt: s.now(), UpdatedBy: caller.ID}
	if err := s.repo.Put(ctx, entry); err != nil {
		return AppConfigSetting{}, err
	}
	return AppConfigSetting{Key: key, Entry: &entry}, nil
}

func (s *AppConfigService) read(ctx context.Context, key string) (AppConfigSetting, error) {
	entry, err := s.repo.Get(ctx, key)
	if errors.Is(err, domain.ErrNotFound) {
		return AppConfigSetting{Key: key}, nil
	}
	if err != nil {
		return AppConfigSetting{}, err
	}
	return AppConfigSetting{Key: key, Entry: &entry}, nil
}
