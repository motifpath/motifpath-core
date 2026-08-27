package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
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
	_, err = r.client.User.Create().
		SetID(id).
		SetClerkUserID(u.ClerkUserID).
		SetRole(user.Role(u.Role)).
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
	row, err := r.client.User.Query().Where(user.ClerkUserID(clerkUserID)).Only(ctx)
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
	row, err := r.client.User.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, err
	}
	return toDomainUser(row), nil
}

func toDomainUser(row *ent.User) domain.User {
	return domain.User{
		ID:           row.ID.String(),
		ClerkUserID:  row.ClerkUserID,
		Role:         domain.Role(row.Role),
		RegisteredAt: row.RegisteredAt,
	}
}
