package repo

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/challengeexercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnodeexercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exerciseoption"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntExerciseRepository persists Exercise, its ExerciseOption children, and
// its many-to-many links to Challenge, via ent/Postgres.
type EntExerciseRepository struct {
	client *ent.Client
}

func NewEntExerciseRepository(client *ent.Client) *EntExerciseRepository {
	return &EntExerciseRepository{client: client}
}

func (r *EntExerciseRepository) Create(ctx context.Context, ex domain.Exercise) error {
	id, err := uuid.Parse(ex.ID)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	builder := tx.Exercise.Create().
		SetID(id).
		SetTitle(ex.Title).
		SetPrompt(ex.Prompt).
		SetExerciseType(exercise.ExerciseType(ex.ExerciseType)).
		SetSkillTags(ex.SkillTags).
		SetNillableImageURL(ex.ImageURL).
		SetNillableAudioURL(ex.AudioURL).
		SetNillableEstimatedDurationSeconds(ex.EstimatedDurationSeconds).
		SetCreatedAt(ex.CreatedAt)
	if _, err := builder.Save(ctx); err != nil {
		return rollback(tx, err)
	}

	optionBuilders := make([]*ent.ExerciseOptionCreate, len(ex.Options))
	for i, opt := range ex.Options {
		optionID, err := uuid.Parse(opt.ID)
		if err != nil {
			return rollback(tx, err)
		}
		optBuilder := tx.ExerciseOption.Create().
			SetID(optionID).
			SetExerciseID(id).
			SetIsCorrect(opt.IsCorrect).
			SetNillableLabel(opt.Label).
			SetNillableImageURL(opt.ImageURL)
		if opt.Region != nil {
			shape := exerciseoption.RegionShape(opt.Region.Shape)
			optBuilder = optBuilder.
				SetRegionX(opt.Region.X).
				SetRegionY(opt.Region.Y).
				SetRegionWidth(opt.Region.Width).
				SetRegionHeight(opt.Region.Height).
				SetRegionShape(shape)
		}
		optionBuilders[i] = optBuilder
	}
	if len(optionBuilders) > 0 {
		if _, err := tx.ExerciseOption.CreateBulk(optionBuilders...).Save(ctx); err != nil {
			return rollback(tx, err)
		}
	}

	return tx.Commit()
}

func (r *EntExerciseRepository) GetByID(ctx context.Context, id string) (domain.Exercise, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Exercise{}, domain.ErrNotFound
	}
	row, err := r.client.Exercise.Query().
		Where(exercise.ID(parsed)).
		WithChallenges().
		WithContentNodes().
		WithOptions().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Exercise{}, domain.ErrNotFound
		}
		return domain.Exercise{}, err
	}
	return toDomainExercise(row), nil
}

// LinkChallenge links exerciseID into challengeID via the ChallengeExercise
// join entity, at the position one past the challenge's current highest —
// an implicit ent many-to-many join table carries no order of its own, and
// Postgres makes no row-order guarantee over an unordered SELECT, so link
// order has to be a real, queried column.
func (r *EntExerciseRepository) LinkChallenge(ctx context.Context, exerciseID, challengeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	chID, err := uuid.Parse(challengeID)
	if err != nil {
		return err
	}
	position, err := nextChallengeExercisePosition(ctx, r.client, chID)
	if err != nil {
		return err
	}
	return r.client.ChallengeExercise.Create().
		SetChallengeID(chID).
		SetExerciseID(exID).
		SetPosition(position).
		Exec(ctx)
}

func (r *EntExerciseRepository) UnlinkChallenge(ctx context.Context, exerciseID, challengeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	chID, err := uuid.Parse(challengeID)
	if err != nil {
		return err
	}
	_, err = r.client.ChallengeExercise.Delete().
		Where(challengeexercise.ExerciseID(exID), challengeexercise.ChallengeID(chID)).
		Exec(ctx)
	return err
}

func (r *EntExerciseRepository) ListByChallengeID(ctx context.Context, challengeID string) ([]domain.Exercise, error) {
	parsed, err := uuid.Parse(challengeID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	links, err := r.client.ChallengeExercise.Query().
		Where(challengeexercise.ChallengeID(parsed)).
		Order(challengeexercise.ByPosition()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(links))
	for i, link := range links {
		ids[i] = link.ExerciseID
	}
	return r.exercisesInOrder(ctx, ids)
}

// LinkContentNode links exerciseID into contentNodeID as a path exercise,
// via the ContentNodeExercise join entity — see LinkChallenge for why
// position is a real column rather than incidental row order.
func (r *EntExerciseRepository) LinkContentNode(ctx context.Context, exerciseID, contentNodeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	nodeID, err := uuid.Parse(contentNodeID)
	if err != nil {
		return err
	}
	position, err := nextContentNodeExercisePosition(ctx, r.client, nodeID)
	if err != nil {
		return err
	}
	return r.client.ContentNodeExercise.Create().
		SetContentNodeID(nodeID).
		SetExerciseID(exID).
		SetPosition(position).
		Exec(ctx)
}

func (r *EntExerciseRepository) UnlinkContentNode(ctx context.Context, exerciseID, contentNodeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	nodeID, err := uuid.Parse(contentNodeID)
	if err != nil {
		return err
	}
	_, err = r.client.ContentNodeExercise.Delete().
		Where(contentnodeexercise.ExerciseID(exID), contentnodeexercise.ContentNodeID(nodeID)).
		Exec(ctx)
	return err
}

func (r *EntExerciseRepository) ListByContentNodeID(ctx context.Context, contentNodeID string) ([]domain.Exercise, error) {
	parsed, err := uuid.Parse(contentNodeID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	links, err := r.client.ContentNodeExercise.Query().
		Where(contentnodeexercise.ContentNodeID(parsed)).
		Order(contentnodeexercise.ByPosition()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(links))
	for i, link := range links {
		ids[i] = link.ExerciseID
	}
	return r.exercisesInOrder(ctx, ids)
}

// exercisesInOrder fetches the exercises with the given ids and returns
// them in exactly that order. A single WHERE id IN (...) query returns rows
// in no particular order, so the requested sequence (link order, from
// ListByChallengeID/ListByContentNodeID) is reapplied in Go rather than
// relied upon from SQL.
func (r *EntExerciseRepository) exercisesInOrder(ctx context.Context, ids []uuid.UUID) ([]domain.Exercise, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.client.Exercise.Query().
		Where(exercise.IDIn(ids...)).
		WithChallenges().
		WithContentNodes().
		WithOptions().
		All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Exercise, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]*ent.Exercise, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return toDomainExercises(ordered), nil
}

// nextChallengeExercisePosition returns one past the highest position
// currently linked to challengeID, or 0 if it has none yet.
func nextChallengeExercisePosition(ctx context.Context, client *ent.Client, challengeID uuid.UUID) (int, error) {
	last, err := client.ChallengeExercise.Query().
		Where(challengeexercise.ChallengeID(challengeID)).
		Order(challengeexercise.ByPosition(sql.OrderDesc())).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return last.Position + 1, nil
}

// nextContentNodeExercisePosition returns one past the highest position
// currently linked to contentNodeID, or 0 if it has none yet.
func nextContentNodeExercisePosition(ctx context.Context, client *ent.Client, contentNodeID uuid.UUID) (int, error) {
	last, err := client.ContentNodeExercise.Query().
		Where(contentnodeexercise.ContentNodeID(contentNodeID)).
		Order(contentnodeexercise.ByPosition(sql.OrderDesc())).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return last.Position + 1, nil
}


// ListBySkillTag filters in Go rather than in the query — skill_tags is a
// JSON array field with no generated "contains element" predicate, and MVP
// catalog scale doesn't warrant a schema change to support one yet.
func (r *EntExerciseRepository) ListBySkillTag(ctx context.Context, skillTag string) ([]domain.Exercise, error) {
	rows, err := r.client.Exercise.Query().
		WithChallenges().
		WithContentNodes().
		WithOptions().
		All(ctx)
	if err != nil {
		return nil, err
	}

	var matched []*ent.Exercise
	for _, row := range rows {
		for _, tag := range row.SkillTags {
			if tag == skillTag {
				matched = append(matched, row)
				break
			}
		}
	}
	return toDomainExercises(matched), nil
}

func toDomainExercises(rows []*ent.Exercise) []domain.Exercise {
	result := make([]domain.Exercise, len(rows))
	for i, row := range rows {
		result[i] = toDomainExercise(row)
	}
	return result
}

func toDomainExercise(row *ent.Exercise) domain.Exercise {
	challengeIDs := make([]string, len(row.Edges.Challenges))
	for i, c := range row.Edges.Challenges {
		challengeIDs[i] = c.ID.String()
	}
	contentNodeIDs := make([]string, len(row.Edges.ContentNodes))
	for i, n := range row.Edges.ContentNodes {
		contentNodeIDs[i] = n.ID.String()
	}

	options := make([]domain.Option, len(row.Edges.Options))
	for i, opt := range row.Edges.Options {
		options[i] = toDomainOption(opt)
	}

	return domain.Exercise{
		ID:                       row.ID.String(),
		Title:                    row.Title,
		Prompt:                   row.Prompt,
		ExerciseType:             domain.ExerciseType(row.ExerciseType),
		SkillTags:                row.SkillTags,
		ImageURL:                 row.ImageURL,
		AudioURL:                 row.AudioURL,
		EstimatedDurationSeconds: row.EstimatedDurationSeconds,
		Options:                  options,
		ChallengeIDs:             challengeIDs,
		ContentNodeIDs:           contentNodeIDs,
		CreatedAt:                row.CreatedAt,
	}
}

func toDomainOption(row *ent.ExerciseOption) domain.Option {
	opt := domain.Option{
		ID:        row.ID.String(),
		IsCorrect: row.IsCorrect,
		Label:     row.Label,
		ImageURL:  row.ImageURL,
	}
	if row.RegionShape != nil {
		opt.Region = &domain.OptionRegion{
			X:      valueOrZero(row.RegionX),
			Y:      valueOrZero(row.RegionY),
			Width:  valueOrZero(row.RegionWidth),
			Height: valueOrZero(row.RegionHeight),
			Shape:  domain.OptionRegionShape(*row.RegionShape),
		}
	}
	return opt
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
