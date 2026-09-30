package repo

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/studentpath"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/studentpathitem"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntStudentPathRepository persists StudentPath and StudentPathItem records
// via ent/Postgres. A student may hold many StudentPaths at once.
type EntStudentPathRepository struct {
	client *ent.Client
}

func NewEntStudentPathRepository(client *ent.Client) *EntStudentPathRepository {
	return &EntStudentPathRepository{client: client}
}

func (r *EntStudentPathRepository) Create(ctx context.Context, path domain.StudentPath) error {
	id, err := uuid.Parse(path.ID)
	if err != nil {
		return err
	}
	studentID, err := uuid.Parse(path.StudentID)
	if err != nil {
		return err
	}
	sourceTemplateID, err := uuid.Parse(path.SourceTemplateID)
	if err != nil {
		return err
	}
	assignedBy, err := uuid.Parse(path.AssignedBy)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	create := tx.StudentPath.Create().
		SetID(id).
		SetStudentID(studentID).
		SetSourceTemplateID(sourceTemplateID).
		SetTitle(path.Title).
		SetAssignedBy(assignedBy).
		SetAssignedAt(path.AssignedAt).
		SetNillableArchivedAt(path.ArchivedAt).
		SetNillableCourseCheckpointPosition(path.CourseCheckpointPosition)
	create, err = withOptionalFields(create, path)
	if err != nil {
		return rollback(tx, err)
	}
	if _, err := create.Save(ctx); err != nil {
		// The only unique constraint a new copy can break is the one active
		// standalone copy per student and template.
		if ent.IsConstraintError(err) {
			return rollback(tx, domain.ErrAlreadyExists)
		}
		return rollback(tx, err)
	}

	itemBuilders, err := studentPathItemBuilders(tx, id, path.Items)
	if err != nil {
		return rollback(tx, err)
	}
	if len(itemBuilders) > 0 {
		if _, err := tx.StudentPathItem.CreateBulk(itemBuilders...).Save(ctx); err != nil {
			return rollback(tx, err)
		}
	}

	return tx.Commit()
}

// withOptionalFields sets a copy's course enrollment, when it is a course
// checkpoint, and the template presentation it recorded — split out to keep
// Create's own cognitive complexity down.
func withOptionalFields(create *ent.StudentPathCreate, path domain.StudentPath) (*ent.StudentPathCreate, error) {
	if path.SourceCourseEnrollmentID != nil {
		enrollmentID, err := uuid.Parse(*path.SourceCourseEnrollmentID)
		if err != nil {
			return nil, err
		}
		create = create.SetSourceCourseEnrollmentID(enrollmentID)
	}
	create = create.
		SetNillableSummarySnapshot(path.SummarySnapshot).
		SetNillableThumbnailURLSnapshot(path.ThumbnailURLSnapshot)
	if path.LevelSnapshot != nil {
		create = create.SetLevelSnapshot(studentpath.LevelSnapshot(*path.LevelSnapshot))
	}
	if path.CreatedBySnapshot != nil {
		creator, err := uuid.Parse(*path.CreatedBySnapshot)
		if err != nil {
			return nil, err
		}
		create = create.SetCreatedBySnapshot(creator)
	}
	return create, nil
}

// studentPathItemBuilders resolves items into the ent create builders
// Create bulk-inserts, split out to keep Create's own cognitive complexity
// down.
func studentPathItemBuilders(tx *ent.Tx, studentPathID uuid.UUID, items []domain.StudentPathItemRecord) ([]*ent.StudentPathItemCreate, error) {
	builders := make([]*ent.StudentPathItemCreate, len(items))
	for i, item := range items {
		contentNodeID, err := uuid.Parse(item.ContentNodeID)
		if err != nil {
			return nil, err
		}
		contentNodeVersionID, err := uuid.Parse(item.ContentNodeVersionID)
		if err != nil {
			return nil, err
		}
		builders[i] = tx.StudentPathItem.Create().
			SetID(uuid.New()).
			SetStudentPathID(studentPathID).
			SetContentNodeID(contentNodeID).
			SetContentNodeVersionID(contentNodeVersionID).
			SetPosition(item.Position).
			SetNillableSectionLabel(item.SectionLabel)
	}
	return builders, nil
}

func (r *EntStudentPathRepository) GetByID(ctx context.Context, id string) (domain.StudentPath, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.StudentPath{}, domain.ErrNotFound
	}

	row, err := r.client.StudentPath.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.StudentPath{}, domain.ErrNotFound
		}
		return domain.StudentPath{}, err
	}

	itemRows, err := r.client.StudentPathItem.Query().
		Where(studentpathitem.StudentPathID(parsed)).
		Order(studentpathitem.ByPosition()).
		All(ctx)
	if err != nil {
		return domain.StudentPath{}, err
	}

	return toDomainStudentPath(row, itemRows), nil
}

func (r *EntStudentPathRepository) FindActiveStandalone(ctx context.Context, studentID, templateID string) (domain.StudentPath, error) {
	student, err := uuid.Parse(studentID)
	if err != nil {
		return domain.StudentPath{}, domain.ErrNotFound
	}
	template, err := uuid.Parse(templateID)
	if err != nil {
		return domain.StudentPath{}, domain.ErrNotFound
	}
	found, err := r.listWithItems(ctx, r.client.StudentPath.Query().
		Where(
			studentpath.StudentID(student),
			studentpath.SourceTemplateID(template),
			studentpath.ArchivedAtIsNil(),
			studentpath.SourceCourseEnrollmentIDIsNil(),
		).
		Limit(1))
	if err != nil {
		return domain.StudentPath{}, err
	}
	if len(found) == 0 {
		return domain.StudentPath{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (r *EntStudentPathRepository) ListActiveStandaloneByStudentID(ctx context.Context, studentID string) ([]domain.StudentPath, error) {
	parsed, err := uuid.Parse(studentID)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	return r.listWithItems(ctx, r.client.StudentPath.Query().
		Where(
			studentpath.StudentID(parsed),
			studentpath.ArchivedAtIsNil(),
			studentpath.SourceCourseEnrollmentIDIsNil(),
		))
}

func (r *EntStudentPathRepository) ListStandaloneByStudentID(ctx context.Context, studentID string) ([]domain.StudentPath, error) {
	parsed, err := uuid.Parse(studentID)
	if err != nil {
		return []domain.StudentPath{}, nil
	}

	return r.listWithItems(ctx, r.client.StudentPath.Query().
		Where(
			studentpath.StudentID(parsed),
			studentpath.SourceCourseEnrollmentIDIsNil(),
		).
		Order(studentpath.ByAssignedAt(sql.OrderDesc()), studentpath.ByID()))
}

// listWithItems runs query and attaches each resulting StudentPath's items
// in position order.
func (r *EntStudentPathRepository) listWithItems(ctx context.Context, query *ent.StudentPathQuery) ([]domain.StudentPath, error) {
	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]domain.StudentPath, len(rows))
	for i, row := range rows {
		itemRows, err := r.client.StudentPathItem.Query().
			Where(studentpathitem.StudentPathID(row.ID)).
			Order(studentpathitem.ByPosition()).
			All(ctx)
		if err != nil {
			return nil, err
		}
		result[i] = toDomainStudentPath(row, itemRows)
	}
	return result, nil
}

func (r *EntStudentPathRepository) Archive(ctx context.Context, id string, archivedAt time.Time) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}

	n, err := r.client.StudentPath.Update().
		Where(studentpath.ID(parsed)).
		SetArchivedAt(archivedAt).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func toDomainStudentPath(row *ent.StudentPath, itemRows []*ent.StudentPathItem) domain.StudentPath {
	items := make([]domain.StudentPathItemRecord, len(itemRows))
	for i, item := range itemRows {
		items[i] = domain.StudentPathItemRecord{
			Position:             item.Position,
			ContentNodeID:        item.ContentNodeID.String(),
			ContentNodeVersionID: item.ContentNodeVersionID.String(),
			SectionLabel:         item.SectionLabel,
		}
	}

	var sourceCourseEnrollmentID *string
	if row.SourceCourseEnrollmentID != nil {
		s := row.SourceCourseEnrollmentID.String()
		sourceCourseEnrollmentID = &s
	}
	var levelSnapshot *domain.DifficultyLevel
	if row.LevelSnapshot != nil {
		l := domain.DifficultyLevel(*row.LevelSnapshot)
		levelSnapshot = &l
	}
	var createdBySnapshot *string
	if row.CreatedBySnapshot != nil {
		c := row.CreatedBySnapshot.String()
		createdBySnapshot = &c
	}

	return domain.StudentPath{
		ID:                       row.ID.String(),
		StudentID:                row.StudentID.String(),
		SourceTemplateID:         row.SourceTemplateID.String(),
		Title:                    row.Title,
		AssignedBy:               row.AssignedBy.String(),
		AssignedAt:               row.AssignedAt,
		ArchivedAt:               row.ArchivedAt,
		SourceCourseEnrollmentID: sourceCourseEnrollmentID,
		CourseCheckpointPosition: row.CourseCheckpointPosition,
		SummarySnapshot:          row.SummarySnapshot,
		LevelSnapshot:            levelSnapshot,
		ThumbnailURLSnapshot:     row.ThumbnailURLSnapshot,
		CreatedBySnapshot:        createdBySnapshot,
		Items:                    items,
	}
}
