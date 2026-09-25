package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/course"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/coursecheckpoint"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpath"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntCourseRepository persists Course and CourseCheckpoint records via
// ent/Postgres. Checkpoint titles are not denormalised onto
// course_checkpoints — GetByID/List join back to learning_paths at read
// time to resolve each checkpoint's effective_title, the same pattern
// EntLearningPathRepository uses for its items' Title/ContentType.
type EntCourseRepository struct {
	client *ent.Client
}

func NewEntCourseRepository(client *ent.Client) *EntCourseRepository {
	return &EntCourseRepository{client: client}
}

func (r *EntCourseRepository) Create(ctx context.Context, c domain.Course) error {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return err
	}
	createdBy, err := uuid.Parse(c.CreatedBy)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Course.Create().
		SetID(id).
		SetTitle(c.Title).
		SetSummary(c.Summary).
		SetLevel(course.Level(c.Level)).
		SetStatus(course.Status(c.Status)).
		SetCreatedBy(createdBy).
		SetCreatedAt(c.CreatedAt).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}

	if err := createCheckpoints(ctx, tx, id, c.Checkpoints); err != nil {
		return rollback(tx, err)
	}

	return tx.Commit()
}

func (r *EntCourseRepository) GetByID(ctx context.Context, id string) (domain.Course, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Course{}, domain.ErrNotFound
	}

	courseRow, err := r.client.Course.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Course{}, domain.ErrNotFound
		}
		return domain.Course{}, err
	}

	checkpointRows, err := r.client.CourseCheckpoint.Query().
		Where(coursecheckpoint.CourseID(parsed)).
		Order(coursecheckpoint.ByPosition()).
		All(ctx)
	if err != nil {
		return domain.Course{}, err
	}

	pathsByID, err := r.learningPathsForCheckpoints(ctx, checkpointRows)
	if err != nil {
		return domain.Course{}, err
	}

	checkpoints, err := buildCourseCheckpoints(checkpointRows, pathsByID)
	if err != nil {
		return domain.Course{}, err
	}

	return toDomainCourse(courseRow, checkpoints), nil
}

// List returns one page of the courses matching filter (see
// courseListPredicates), batching the checkpoint and learning-path lookups
// into one query each across the page's courses — the same batching
// EntLearningPathRepository.List uses for its items and content nodes.
func (r *EntCourseRepository) List(ctx context.Context, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error) {
	predicates, err := courseListPredicates(filter)
	if err != nil {
		return domain.Page[domain.Course]{}, err
	}
	query := r.client.Course.Query().Where(predicates...)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return domain.Page[domain.Course]{}, err
	}
	courseRows, err := query.
		Order(courseListOrder(filter)...).
		Limit(page.Limit).
		Offset(page.Offset).
		All(ctx)
	if err != nil {
		return domain.Page[domain.Course]{}, err
	}
	if len(courseRows) == 0 {
		return domain.Page[domain.Course]{Items: []domain.Course{}, Total: total}, nil
	}

	courseIDs := make([]uuid.UUID, len(courseRows))
	for i, c := range courseRows {
		courseIDs[i] = c.ID
	}
	checkpointRows, err := r.client.CourseCheckpoint.Query().
		Where(coursecheckpoint.CourseIDIn(courseIDs...)).
		Order(coursecheckpoint.ByPosition()).
		All(ctx)
	if err != nil {
		return domain.Page[domain.Course]{}, err
	}

	pathsByID, err := r.learningPathsForCheckpoints(ctx, checkpointRows)
	if err != nil {
		return domain.Page[domain.Course]{}, err
	}

	// checkpointRows is sorted by position across all courses; bucketing by
	// course_id below preserves that relative order within each bucket, so
	// no per-course re-sort is needed.
	checkpointsByCourseID := make(map[uuid.UUID][]*ent.CourseCheckpoint, len(courseRows))
	for _, cp := range checkpointRows {
		checkpointsByCourseID[cp.CourseID] = append(checkpointsByCourseID[cp.CourseID], cp)
	}

	items := make([]domain.Course, len(courseRows))
	for i, c := range courseRows {
		checkpoints, err := buildCourseCheckpoints(checkpointsByCourseID[c.ID], pathsByID)
		if err != nil {
			return domain.Page[domain.Course]{}, err
		}
		items[i] = toDomainCourse(c, checkpoints)
	}
	return domain.Page[domain.Course]{Items: items, Total: total}, nil
}

// ListCreatorIDs returns the distinct created_by of every course matching
// filter, using the same predicates as List.
func (r *EntCourseRepository) ListCreatorIDs(ctx context.Context, filter domain.CourseListFilter) ([]string, error) {
	predicates, err := courseListPredicates(filter)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CreatedBy uuid.UUID `json:"created_by"`
	}
	if err := r.client.Course.Query().
		Where(predicates...).
		Unique(true).
		Select(course.FieldCreatedBy).
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.CreatedBy.String()
	}
	return ids, nil
}

// Replace deletes course's current checkpoints and inserts
// course.Checkpoints in their place, in one transaction, then updates the
// course's own title/summary/level. Checkpoints are immutable once created
// (see CourseCheckpoint's schema), so a replace is expressed as
// delete-then-recreate rather than a per-checkpoint update. This never
// touches any CourseVersion — publishing a snapshot is a separate,
// explicit operation.
func (r *EntCourseRepository) Replace(ctx context.Context, c domain.Course) error {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return domain.ErrNotFound
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Course.UpdateOneID(id).
		SetTitle(c.Title).
		SetSummary(c.Summary).
		SetLevel(course.Level(c.Level)).
		Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return rollback(tx, domain.ErrNotFound)
		}
		return rollback(tx, err)
	}

	if _, err := tx.CourseCheckpoint.Delete().
		Where(coursecheckpoint.CourseID(id)).
		Exec(ctx); err != nil {
		return rollback(tx, err)
	}

	if err := createCheckpoints(ctx, tx, id, c.Checkpoints); err != nil {
		return rollback(tx, err)
	}

	return tx.Commit()
}

// UpdateStatus sets the status of the course with the given id.
func (r *EntCourseRepository) UpdateStatus(ctx context.Context, id string, status domain.CourseStatus) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}

	_, err = r.client.Course.UpdateOneID(parsed).
		SetStatus(course.Status(status)).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

// createCheckpoints bulk-inserts checkpoints under courseID within tx,
// shared by Create and Replace.
func createCheckpoints(ctx context.Context, tx *ent.Tx, courseID uuid.UUID, checkpoints []domain.CourseCheckpoint) error {
	if len(checkpoints) == 0 {
		return nil
	}
	builders := make([]*ent.CourseCheckpointCreate, len(checkpoints))
	for i, cp := range checkpoints {
		learningPathID, err := uuid.Parse(cp.LearningPathID)
		if err != nil {
			return err
		}
		builders[i] = tx.CourseCheckpoint.Create().
			SetID(uuid.New()).
			SetCourseID(courseID).
			SetLearningPathID(learningPathID).
			SetPosition(cp.Position).
			SetNillableTitle(cp.Title)
	}
	_, err := tx.CourseCheckpoint.CreateBulk(builders...).Save(ctx)
	return err
}

// learningPathsForCheckpoints batch-fetches the learning path templates
// referenced by checkpointRows, keyed by id, to resolve each checkpoint's
// effective_title.
func (r *EntCourseRepository) learningPathsForCheckpoints(ctx context.Context, checkpointRows []*ent.CourseCheckpoint) (map[uuid.UUID]*ent.LearningPath, error) {
	pathIDs := make([]uuid.UUID, len(checkpointRows))
	for i, cp := range checkpointRows {
		pathIDs[i] = cp.LearningPathID
	}
	pathRows, err := r.client.LearningPath.Query().Where(learningpath.IDIn(pathIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	pathsByID := make(map[uuid.UUID]*ent.LearningPath, len(pathRows))
	for _, p := range pathRows {
		pathsByID[p.ID] = p
	}
	return pathsByID, nil
}

// buildCourseCheckpoints resolves each checkpoint's effective_title from
// pathsByID, shared by GetByID and List.
func buildCourseCheckpoints(checkpointRows []*ent.CourseCheckpoint, pathsByID map[uuid.UUID]*ent.LearningPath) ([]domain.CourseCheckpoint, error) {
	checkpoints := make([]domain.CourseCheckpoint, len(checkpointRows))
	for i, cp := range checkpointRows {
		path, ok := pathsByID[cp.LearningPathID]
		if !ok {
			return nil, fmt.Errorf("course checkpoint %s references missing learning path %s", cp.ID, cp.LearningPathID)
		}
		effectiveTitle := path.Title
		if cp.Title != nil && *cp.Title != "" {
			effectiveTitle = *cp.Title
		}
		checkpoints[i] = domain.CourseCheckpoint{
			Position:       cp.Position,
			LearningPathID: cp.LearningPathID.String(),
			Title:          cp.Title,
			EffectiveTitle: effectiveTitle,
		}
	}
	return checkpoints, nil
}

func toDomainCourse(row *ent.Course, checkpoints []domain.CourseCheckpoint) domain.Course {
	return domain.Course{
		ID:          row.ID.String(),
		Title:       row.Title,
		Summary:     row.Summary,
		Level:       domain.DifficultyLevel(row.Level),
		Status:      domain.CourseStatus(row.Status),
		CreatedBy:   row.CreatedBy.String(),
		CreatedAt:   row.CreatedAt,
		Checkpoints: checkpoints,
	}
}
