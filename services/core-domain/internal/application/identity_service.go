package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// defaultLocale is the locale a new registration falls back to when the
// caller's Accept-Language header names no known, non-"any" Language.
const defaultLocale = "en"

// IdentityService maps Clerk identities to MotifPath User records.
// ResolveCaller is the "resolve caller" lookup every other application
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
// identity. claims are what the caller's identity token carries, never
// client-supplied values: a blank name is rejected on field "name", and an
// email that isn't a valid address on field "email", so every new user can
// be shown by name and reached by email.
func (s *IdentityService) RegisterUser(ctx context.Context, clerkUserID string, role domain.Role, acceptLanguageCandidate string, claims domain.IdentityClaims) (domain.User, error) {
	locale := s.resolveRegistrationLocale(ctx, acceptLanguageCandidate)

	user, err := domain.NewUser(s.newID(), clerkUserID, role, claims.Name, locale, s.now())
	email, emailOK := domain.NormalizeEmail(claims.Email)
	if err != nil || !emailOK {
		return domain.User{}, withEmailError(err, emailOK)
	}
	user.Email = email
	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// withEmailError adds the invalid-email field error to err's validation
// errors, so a registration with several bad claims reports all of them.
// err is nil or the error domain.NewUser returned.
func withEmailError(err error, emailOK bool) error {
	if emailOK {
		return err
	}
	emailErr := domain.FieldError{Field: "email", Reason: "must be a valid email address"}
	var valErr *domain.ValidationError
	if errors.As(err, &valErr) {
		return &domain.ValidationError{Fields: append(valErr.Fields, emailErr)}
	}
	if err != nil {
		return err
	}
	return &domain.ValidationError{Fields: []domain.FieldError{emailErr}}
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

// ResolveCaller returns the user registered for clerkUserID, first bringing
// their stored name and email address up to date with claims — what the
// identity token of the request being served carries. The identity
// provider owns both, so a change there reaches MotifPath on the user's
// next request. A blank or invalid claim is ignored rather than clearing
// the stored value, and an unchanged one writes nothing. Returns
// domain.ErrNotFound if the identity has never registered.
func (s *IdentityService) ResolveCaller(ctx context.Context, clerkUserID string, claims domain.IdentityClaims) (domain.User, error) {
	user, err := s.users.GetByClerkUserID(ctx, clerkUserID)
	if err != nil {
		return domain.User{}, err
	}
	if user, err = s.refreshDisplayName(ctx, user, claims.Name); err != nil {
		return domain.User{}, err
	}
	return s.refreshEmail(ctx, user, claims.Email)
}

func (s *IdentityService) refreshDisplayName(ctx context.Context, user domain.User, claimed string) (domain.User, error) {
	name, ok := domain.NormalizeDisplayName(claimed)
	if !ok || name == user.DisplayName {
		return user, nil
	}
	if err := s.users.UpdateDisplayName(ctx, user.ID, name); err != nil {
		return domain.User{}, err
	}
	user.DisplayName = name
	return user, nil
}

func (s *IdentityService) refreshEmail(ctx context.Context, user domain.User, claimed string) (domain.User, error) {
	email, ok := domain.NormalizeEmail(claimed)
	if !ok || email == user.Email {
		return user, nil
	}
	if err := s.users.UpdateEmail(ctx, user.ID, email); err != nil {
		return domain.User{}, err
	}
	user.Email = email
	return user, nil
}

// DisplayNames returns the current display name of every user in ids, keyed
// by user_id, in one lookup however many times an id repeats. Every id is
// expected to name a real user — anything that points at a user was
// recorded from an authenticated caller — so a missing one is reported as
// domain.ErrNotFound rather than shown without a name.
func (s *IdentityService) DisplayNames(ctx context.Context, ids []string) (map[string]string, error) {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return map[string]string{}, nil
	}
	names, err := s.users.GetDisplayNames(ctx, unique)
	if err != nil {
		return nil, err
	}
	for _, id := range unique {
		if _, ok := names[id]; !ok {
			return nil, fmt.Errorf("display name for user %s: %w", id, domain.ErrNotFound)
		}
	}
	return names, nil
}

// UpdateProfile applies update to the authenticated user's locale, time
// zone and email preferences, in one write, and returns the updated user.
// A field left out of update keeps its value. Returns a
// *domain.ValidationError on field "body" when update changes nothing, on
// "locale" for a locale that isn't a known, non-"any" language code, on
// "time_zone" for an unknown IANA time zone, and on "email_preferences"
// when it names no switch; domain.ErrNotFound if the caller has never
// registered.
func (s *IdentityService) UpdateProfile(ctx context.Context, clerkUserID string, update domain.ProfileUpdate) (domain.User, error) {
	if update.IsEmpty() {
		return domain.User{}, domain.NewValidationError("body", "must change at least one of locale, time_zone and email_preferences")
	}
	locale, err := s.validateProfileUpdate(ctx, update)
	if err != nil {
		return domain.User{}, err
	}

	user, err := s.users.GetByClerkUserID(ctx, clerkUserID)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.users.UpdatePreferences(ctx, user.ID, update); err != nil {
		return domain.User{}, err
	}

	if update.Locale != nil {
		user.Locale = locale
	}
	if update.TimeZone != nil {
		user.TimeZone = *update.TimeZone
	}
	if update.EmailPreferences != nil {
		user.EmailPreferences = update.EmailPreferences.Apply(user.EmailPreferences)
	}
	return user, nil
}

// validateProfileUpdate checks every field update sets and returns the
// Language its locale names, if it names one.
func (s *IdentityService) validateProfileUpdate(ctx context.Context, update domain.ProfileUpdate) (domain.Language, error) {
	var locale domain.Language
	if update.Locale != nil {
		lang, err := s.resolveLocale(ctx, *update.Locale)
		if err != nil {
			return domain.Language{}, err
		}
		locale = lang
	}
	if update.TimeZone != nil {
		if err := domain.ValidateTimeZone(*update.TimeZone); err != nil {
			return domain.Language{}, err
		}
	}
	if update.EmailPreferences != nil && update.EmailPreferences.IsEmpty() {
		return domain.Language{}, domain.NewValidationError("email_preferences", "must name at least one known switch")
	}
	return locale, nil
}

// resolveLocale returns the Language a user's locale may be set to: an
// existing one, never "any".
func (s *IdentityService) resolveLocale(ctx context.Context, code string) (domain.Language, error) {
	if code == "" || code == domain.LanguageCodeAny {
		return domain.Language{}, domain.NewValidationError("locale", "must be a known language code")
	}
	lang, err := s.languages.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Language{}, domain.NewValidationError("locale", "must be a known language code")
		}
		return domain.Language{}, err
	}
	return lang, nil
}
