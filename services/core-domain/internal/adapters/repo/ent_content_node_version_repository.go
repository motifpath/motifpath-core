package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnodeversion"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntContentNodeVersionRepository persists ContentNodeVersion snapshots via
// ent/Postgres.
type EntContentNodeVersionRepository struct {
	client *ent.Client
}

func NewEntContentNodeVersionRepository(client *ent.Client) *EntContentNodeVersionRepository {
	return &EntContentNodeVersionRepository{client: client}
}

func (r *EntContentNodeVersionRepository) Create(ctx context.Context, version domain.ContentNodeVersion) error {
	id, err := uuid.Parse(version.ID)
	if err != nil {
		return err
	}
	contentNodeID, err := uuid.Parse(version.ContentNodeID)
	if err != nil {
		return err
	}
	publishedBy, err := uuid.Parse(version.PublishedBy)
	if err != nil {
		return err
	}

	richContentJSON, err := marshalRichContent(version.RichContent)
	if err != nil {
		return err
	}

	_, err = r.client.ContentNodeVersion.Create().
		SetID(id).
		SetContentNodeID(contentNodeID).
		SetVersionNumber(version.VersionNumber).
		SetTitle(version.Title).
		SetContentType(contentnodeversion.ContentType(version.ContentType)).
		SetNillableMediaURL(version.MediaURL).
		SetNillableRichContent(richContentJSON).
		SetPublishedBy(publishedBy).
		SetPublishedAt(version.PublishedAt).
		Save(ctx)
	return err
}

func (r *EntContentNodeVersionRepository) GetLatestByContentNodeID(ctx context.Context, contentNodeID string) (domain.ContentNodeVersion, error) {
	parsed, err := uuid.Parse(contentNodeID)
	if err != nil {
		return domain.ContentNodeVersion{}, domain.ErrNotFound
	}

	row, err := r.client.ContentNodeVersion.Query().
		Where(contentnodeversion.ContentNodeID(parsed)).
		Order(ent.Desc(contentnodeversion.FieldVersionNumber)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ContentNodeVersion{}, domain.ErrNotFound
		}
		return domain.ContentNodeVersion{}, err
	}

	return domain.ContentNodeVersion{
		ID:            row.ID.String(),
		ContentNodeID: row.ContentNodeID.String(),
		VersionNumber: row.VersionNumber,
		Title:         row.Title,
		ContentType:   domain.ContentType(row.ContentType),
		MediaURL:      row.MediaURL,
		RichContent:   unmarshalRichContent(row.RichContent),
		PublishedBy:   row.PublishedBy.String(),
		PublishedAt:   row.PublishedAt,
	}, nil
}
