package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// defaultLocale is the locale a new registration falls back to when the
// caller's Accept-Language header names no known, non-"any" Language.
const defaultLocale = "en"

// IdentityService maps Clerk identities to MotifPath User records.
// GetProfile doubles as the "resolve caller" lookup every other application
// service depends on: the HTTP handler layer calls it once per request to
// turn the JWT's Clerk sub claim into a domain.User carrying the real
// user_id and role, then passes that resolved User into whichever service
// handles the request. This is the first real implementation of User.role
// in the codebase — event-ingestion's admin endpoints currently fake role
// via a Clerk JWT custom claim specifically because this service didn't
// exist yet (ADR-012 Part 3 flags that as reconciliation debt).
type IdentityService struct {
	users     ports.UserRepository
	languages ports.LanguageRepository
	newID     func() string
	now       func() time.Time
}

func NewIdentityService(users ports.UserRepository, languages ports.LanguageRepository, newID func() string, now func() time.Time) *IdentityService {
	return &IdentityService{users: users, languages: languages, newID: newID, now: now}
}

// RegisterUser creates a new user for clerkUserID. acceptLanguageCandidate is
// a normalized locale candidate derived from the caller's Accept-Language
// header (see domain.NormalizeAcceptLanguage) — used as the user's initial
// locale when it names a known, non-"any" Language; falls back to
// defaultLocale otherwise. Returns domain.ErrAlreadyExists if a user record
// already exists for that Clerk identity — registration is callable once per
// identity.
func (s *IdentityService) RegisterUser(ctx context.Context, clerkUserID string, role domain.Role, acceptLanguageCandidate string) (domain.User, error) {
	locale := s.resolveRegistrationLocale(ctx, acceptLanguageCandidate)

	user, err := domain.NewUser(s.newID(), clerkUserID, role, locale, s.now())
	if err != nil {
		return domain.User{}, err
	}
	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// resolveRegistrationLocale honors a valid Accept-Language-derived candidate
// when it names a known, non-"any" Language, and falls back to defaultLocale
// otherwise — including when candidate is empty or the lookup fails for any
// reason. A malformed or unrecognized header must never block registration.
func (s *IdentityService) resolveRegistrationLocale(ctx context.Context, candidate string) domain.Language {
	if candidate != "" && candidate != domain.LanguageCodeAny {
		if lang, err := s.languages.GetByCode(ctx, candidate); err == nil {
			return lang
		}
	}
	if lang, err := s.languages.GetByCode(ctx, defaultLocale); err == nil {
		return lang
	}
	// defaultLocale is a system row seeded by the Atlas migration and must
	// always exist; this fallback only guards against that invariant being
	// violated rather than a path expected to run in practice.
	return domain.Language{Code: defaultLocale}
}

// GetProfile returns the user registered for clerkUserID. Returns
// domain.ErrNotFound if the identity has never registered.
func (s *IdentityService) GetProfile(ctx context.Context, clerkUserID string) (domain.User, error) {
	return s.users.GetByClerkUserID(ctx, clerkUserID)
}

// UpdateLocale sets the authenticated user's locale preference to locale.
// Returns a *domain.ValidationError on field "locale" if locale does not
// name an existing, non-"any" Language; domain.ErrNotFound if the caller has
// never registered.
func (s *IdentityService) UpdateLocale(ctx context.Context, clerkUserID, locale string) (domain.User, error) {
	if locale == "" || locale == domain.LanguageCodeAny {
		return domain.User{}, domain.NewValidationError("locale", "must be a known language code")
	}
	lang, err := s.languages.GetByCode(ctx, locale)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, domain.NewValidationError("locale", "must be a known language code")
		}
		return domain.User{}, err
	}

	user, err := s.users.GetByClerkUserID(ctx, clerkUserID)
	if err != nil {
		return domain.User{}, err
	}

	if err := s.users.UpdateLocale(ctx, user.ID, locale); err != nil {
		return domain.User{}, err
	}
	user.Locale = lang
	return user, nil
}
