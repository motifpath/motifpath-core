package repo

import (
	"context"
	"encoding/json"

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

	classificationJSON, err := marshalVersionClassification(version.Classification)
	if err != nil {
		return err
	}
	languagesJSON, err := marshalVersionLanguages(version.Languages)
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
		SetClassificationSnapshot(classificationJSON).
		SetLanguagesSnapshot(languagesJSON).
		SetInstrumentIdsSnapshot(version.InstrumentIDsSnapshot).
		SetNillableThumbnailURLSnapshot(version.ThumbnailURLSnapshot).
		SetPublishedBy(publishedBy).
		SetPublishedAt(version.PublishedAt).
		Save(ctx)
	return err
}

func (r *EntContentNodeVersionRepository) ListByContentNodeID(ctx context.Context, contentNodeID string) ([]domain.ContentNodeVersion, error) {
	parsed, err := uuid.Parse(contentNodeID)
	if err != nil {
		return []domain.ContentNodeVersion{}, nil
	}

	rows, err := r.client.ContentNodeVersion.Query().
		Where(contentnodeversion.ContentNodeID(parsed)).
		Order(ent.Desc(contentnodeversion.FieldVersionNumber)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]domain.ContentNodeVersion, len(rows))
	for i, row := range rows {
		versions[i] = toDomainContentNodeVersion(row)
	}
	return versions, nil
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

	return toDomainContentNodeVersion(row), nil
}

func toDomainContentNodeVersion(row *ent.ContentNodeVersion) domain.ContentNodeVersion {
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

		Classification:        unmarshalVersionClassification(row.ClassificationSnapshot),
		Languages:             unmarshalVersionLanguages(row.LanguagesSnapshot),
		InstrumentIDsSnapshot: row.InstrumentIdsSnapshot,
		ThumbnailURLSnapshot:  row.ThumbnailURLSnapshot,
	}
}

// versionKnowledgeNodeJSON is a skill or concept as frozen into a
// version's classification snapshot, so the snapshot never changes if the
// node is later renamed, moved or re-scoped.
type versionKnowledgeNodeJSON struct {
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	Key           string            `json:"key"`
	Names         map[string]string `json:"names"`
	Descriptions  map[string]string `json:"descriptions,omitempty"`
	ParentID      *string           `json:"parent_id,omitempty"`
	InstrumentIDs []string          `json:"instrument_ids,omitempty"`
}

func toVersionKnowledgeNodeJSON(n domain.KnowledgeNode) versionKnowledgeNodeJSON {
	return versionKnowledgeNodeJSON{
		ID: n.ID, Kind: string(n.Kind), Key: n.Key, Names: n.Names, Descriptions: n.Descriptions,
		ParentID: n.ParentID, InstrumentIDs: n.InstrumentIDs,
	}
}

func (n versionKnowledgeNodeJSON) toDomain() domain.KnowledgeNode {
	return domain.KnowledgeNode{
		ID: n.ID, Kind: domain.KnowledgeNodeKind(n.Kind), Key: n.Key, Names: n.Names, Descriptions: n.Descriptions,
		ParentID: n.ParentID, InstrumentIDs: n.InstrumentIDs,
	}
}

type versionClassificationJSON struct {
	Skills          []versionKnowledgeNodeJSON `json:"skills"`
	Concepts        []versionKnowledgeNodeJSON `json:"concepts"`
	DifficultyLevel string                     `json:"difficulty_level"`
	ReviewState     string                     `json:"review_state"`
}

type versionLanguageJSON struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func marshalVersionClassification(c domain.Classification) (string, error) {
	snapshot := versionClassificationJSON{
		Skills:          make([]versionKnowledgeNodeJSON, len(c.Skills)),
		Concepts:        make([]versionKnowledgeNodeJSON, len(c.Concepts)),
		DifficultyLevel: string(c.DifficultyLevel),
		ReviewState:     string(c.ReviewState),
	}
	for i, skill := range c.Skills {
		snapshot.Skills[i] = toVersionKnowledgeNodeJSON(skill)
	}
	for i, concept := range c.Concepts {
		snapshot.Concepts[i] = toVersionKnowledgeNodeJSON(concept)
	}
	data, err := json.Marshal(snapshot)
	return string(data), err
}

// unmarshalVersionClassification returns the zero Classification for a
// version published before snapshots were persisted (nil column) or whose
// JSON is malformed.
func unmarshalVersionClassification(raw *string) domain.Classification {
	if raw == nil {
		return domain.Classification{}
	}
	var snapshot versionClassificationJSON
	if err := json.Unmarshal([]byte(*raw), &snapshot); err != nil {
		return domain.Classification{}
	}
	result := domain.Classification{
		DifficultyLevel: domain.DifficultyLevel(snapshot.DifficultyLevel),
		ReviewState:     domain.ReviewState(snapshot.ReviewState),
	}
	for _, skill := range snapshot.Skills {
		result.Skills = append(result.Skills, skill.toDomain())
	}
	for _, concept := range snapshot.Concepts {
		result.Concepts = append(result.Concepts, concept.toDomain())
	}
	return result
}

func marshalVersionLanguages(languages []domain.Language) (string, error) {
	snapshot := make([]versionLanguageJSON, len(languages))
	for i, l := range languages {
		snapshot[i] = versionLanguageJSON{Code: l.Code, Name: l.Name}
	}
	data, err := json.Marshal(snapshot)
	return string(data), err
}

// unmarshalVersionLanguages returns nil for a version published before
// snapshots were persisted (nil column) or whose JSON is malformed.
func unmarshalVersionLanguages(raw *string) []domain.Language {
	if raw == nil {
		return nil
	}
	var snapshot []versionLanguageJSON
	if err := json.Unmarshal([]byte(*raw), &snapshot); err != nil {
		return nil
	}
	result := make([]domain.Language, len(snapshot))
	for i, l := range snapshot {
		result[i] = domain.Language{Code: l.Code, Name: l.Name}
	}
	return result
}
