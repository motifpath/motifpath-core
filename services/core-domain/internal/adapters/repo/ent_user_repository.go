package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/language"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/user"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntUserRepository persists User records via ent/Postgres.
type EntUserRepository struct {
	client *ent.Client
}

func NewEntUserRepository(client *ent.Client) *EntUserRepository {
	return &EntUserRepository{client: client}
}

func (r *EntUserRepository) Create(ctx context.Context, u domain.User) error {
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return err
	}
	localeRow, err := r.client.Language.Query().Where(language.Code(u.Locale.Code)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.NewValidationError("locale", "must be a known language code")
		}
		return err
	}
	_, err = r.client.User.Create().
		SetID(id).
		SetClerkUserID(u.ClerkUserID).
		SetRole(user.Role(u.Role)).
		SetDisplayName(u.DisplayName).
		SetLocaleID(localeRow.ID).
		SetNillableEmail(nilIfEmpty(u.Email)).
		SetNillableTimeZone(nilIfEmpty(u.TimeZone)).
		SetEmailProgressReport(u.EmailPreferences.ProgressReport).
		SetEmailReminders(u.EmailPreferences.Reminders).
		SetRegisteredAt(u.RegisteredAt).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *EntUserRepository) GetByClerkUserID(ctx context.Context, clerkUserID string) (domain.User, error) {
	row, err := r.client.User.Query().Where(user.ClerkUserID(clerkUserID)).WithLocale().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, err
	}
	return toDomainUser(row), nil
}

func (r *EntUserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.User{}, domain.ErrNotFound
	}
	row, err := r.client.User.Query().Where(user.ID(parsed)).WithLocale().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, err
	}
	return toDomainUser(row), nil
}

func (r *EntUserRepository) UpdatePreferences(ctx context.Context, id string, update domain.ProfileUpdate) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	mutation := r.client.User.UpdateOneID(parsed)
	if update.Locale != nil {
		localeID, err := r.localeID(ctx, *update.Locale)
		if err != nil {
			return err
		}
		mutation.SetLocaleID(localeID)
	}
	if update.TimeZone != nil {
		mutation.SetTimeZone(*update.TimeZone)
	}
	if update.EmailPreferences != nil {
		setEmailPreferences(mutation, *update.EmailPreferences)
	}
	if _, err := mutation.Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

// localeID returns the id of the Language with code, or a validation error
// on field "locale" when there is none.
func (r *EntUserRepository) localeID(ctx context.Context, code string) (uuid.UUID, error) {
	row, err := r.client.Language.Query().Where(language.Code(code)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return uuid.UUID{}, domain.NewValidationError("locale", "must be a known language code")
		}
		return uuid.UUID{}, err
	}
	return row.ID, nil
}

func setEmailPreferences(mutation *ent.UserUpdateOne, update domain.EmailPreferencesUpdate) {
	if update.ProgressReport != nil {
		mutation.SetEmailProgressReport(*update.ProgressReport)
	}
	if update.Reminders != nil {
		mutation.SetEmailReminders(*update.Reminders)
	}
}

func (r *EntUserRepository) UpdateEmail(ctx context.Context, id, email string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	_, err = r.client.User.UpdateOneID(parsed).SetEmail(email).Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *EntUserRepository) UpdateDisplayName(ctx context.Context, id, displayName string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	_, err = r.client.User.UpdateOneID(parsed).SetDisplayName(displayName).Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

// ListByRole returns every user with role.
func (r *EntUserRepository) ListByRole(ctx context.Context, role domain.Role) ([]domain.User, error) {
	rows, err := r.client.User.Query().Where(user.RoleEQ(user.Role(role))).WithLocale().All(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]domain.User, len(rows))
	for i, row := range rows {
		users[i] = toDomainUser(row)
	}
	return users, nil
}

// GetDisplayNames reads only the id and display_name columns of the
// requested users in one query. An id that isn't a valid UUID can't name a
// user, so it is skipped the same way as a valid id with no row.
func (r *EntUserRepository) GetDisplayNames(ctx context.Context, ids []string) (map[string]string, error) {
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if u, err := uuid.Parse(id); err == nil {
			parsed = append(parsed, u)
		}
	}
	names := make(map[string]string, len(parsed))
	if len(parsed) == 0 {
		return names, nil
	}
	rows, err := r.client.User.Query().
		Where(user.IDIn(parsed...)).
		Select(user.FieldID, user.FieldDisplayName).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.ID.String()] = row.DisplayName
	}
	return names, nil
}

func toDomainUser(row *ent.User) domain.User {
	var locale domain.Language
	if row.Edges.Locale != nil {
		locale = domain.Language{Code: row.Edges.Locale.Code, Name: row.Edges.Locale.Name}
	}
	return domain.User{
		ID:          row.ID.String(),
		ClerkUserID: row.ClerkUserID,
		Role:        domain.Role(row.Role),
		DisplayName: row.DisplayName,
		Locale:      locale,
		Email:       valueOrEmpty(row.Email),
		TimeZone:    valueOrEmpty(row.TimeZone),
		EmailPreferences: domain.EmailPreferences{
			ProgressReport: row.EmailProgressReport,
			Reminders:      row.EmailReminders,
		},
		RegisteredAt: row.RegisteredAt,
	}
}

// nilIfEmpty stores an absent optional text field as null, not as "".
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
