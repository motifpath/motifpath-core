package repo

import (
	"context"
	"encoding/json"

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
	promptJSON, err := marshalPrompt(ex.Prompt)
	if err != nil {
		return err
	}
	remediationJSON, err := marshalRemediationTargets(ex.RemediationTargets)
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
		SetPrompt(promptJSON).
		SetExerciseType(exercise.ExerciseType(ex.ExerciseType)).
		SetSkillTags(ex.SkillTags).
		SetNillableImageURL(ex.ImageURL).
		SetNillableAudioURL(ex.AudioURL).
		SetNillableEstimatedDurationSeconds(ex.EstimatedDurationSeconds).
		SetNillableRemediationTargets(remediationJSON).
		SetCreatedAt(ex.CreatedAt)
	if _, err := builder.Save(ctx); err != nil {
		return rollback(tx, err)
	}

	optionBuilders, err := buildExerciseOptionCreates(tx, id, ex.Options)
	if err != nil {
		return rollback(tx, err)
	}
	if len(optionBuilders) > 0 {
		if _, err := tx.ExerciseOption.CreateBulk(optionBuilders...).Save(ctx); err != nil {
			return rollback(tx, err)
		}
	}

	return tx.Commit()
}

// buildExerciseOptionCreates prepares one ExerciseOptionCreate builder per
// opt, shared by Create and Update since both fully (re)establish an
// exercise's options the same way.
func buildExerciseOptionCreates(tx *ent.Tx, exerciseID uuid.UUID, options []domain.Option) ([]*ent.ExerciseOptionCreate, error) {
	builders := make([]*ent.ExerciseOptionCreate, len(options))
	for i, opt := range options {
		optionID, err := uuid.Parse(opt.ID)
		if err != nil {
			return nil, err
		}
		optBuilder := tx.ExerciseOption.Create().
			SetID(optionID).
			SetExerciseID(exerciseID).
			SetIsCorrect(opt.IsCorrect).
			SetNillableLabel(opt.Label).
			SetNillableImageURL(opt.ImageURL).
			SetNillableAudioURL(opt.AudioURL)
		if opt.Region != nil {
			optBuilder = optBuilder.
				SetRegionX(opt.Region.X).
				SetRegionY(opt.Region.Y).
				SetRegionWidth(opt.Region.Width).
				SetRegionHeight(opt.Region.Height).
				SetRegionShape(exerciseoption.RegionShape(opt.Region.Shape))
		}
		builders[i] = optBuilder
	}
	return builders, nil
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
// join entity — an implicit ent many-to-many join table carries no order of
// its own, and Postgres makes no row-order guarantee over an unordered
// SELECT, so link order has to be real and queryable. ChallengeExercise's
// auto-incrementing id, assigned atomically by Postgres on insert, already
// gives that for free — no separate position column or read-before-write
// needed.
func (r *EntExerciseRepository) LinkChallenge(ctx context.Context, exerciseID, challengeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	chID, err := uuid.Parse(challengeID)
	if err != nil {
		return err
	}
	return r.client.ChallengeExercise.Create().
		SetChallengeID(chID).
		SetExerciseID(exID).
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
		Order(challengeexercise.ByID()).
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
// via the ContentNodeExercise join entity — see LinkChallenge for why its
// auto-incrementing id column is enough for link order on its own.
func (r *EntExerciseRepository) LinkContentNode(ctx context.Context, exerciseID, contentNodeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	nodeID, err := uuid.Parse(contentNodeID)
	if err != nil {
		return err
	}
	return r.client.ContentNodeExercise.Create().
		SetContentNodeID(nodeID).
		SetExerciseID(exID).
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
		Order(contentnodeexercise.ByID()).
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

// List returns exercises from the whole pool, optionally narrowed by
// skillTag and/or exerciseType. exerciseType filters in the query itself;
// skillTag filters in Go, same as ListBySkillTag, since skill_tags is a JSON
// array field with no generated "contains element" predicate and MVP
// catalog scale doesn't warrant a schema change to support one yet.
func (r *EntExerciseRepository) List(ctx context.Context, skillTag string, exerciseType domain.ExerciseType) ([]domain.Exercise, error) {
	query := r.client.Exercise.Query().
		WithChallenges().
		WithContentNodes().
		WithOptions().
		Order(exercise.ByCreatedAt())
	if exerciseType != "" {
		query = query.Where(exercise.ExerciseTypeEQ(exercise.ExerciseType(exerciseType)))
	}

	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	if skillTag == "" {
		return toDomainExercises(rows), nil
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

// Update replaces ex's mutable fields (title, prompt, skill_tags, image_url,
// audio_url, estimated_duration_seconds) and fully replaces its options —
// the existing ExerciseOption rows are deleted and the new set is bulk
// created, the same way Create establishes them initially, since options
// arrive from the caller as a complete replacement set rather than a diff.
func (r *EntExerciseRepository) Update(ctx context.Context, ex domain.Exercise) error {
	id, err := uuid.Parse(ex.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	promptJSON, err := marshalPrompt(ex.Prompt)
	if err != nil {
		return err
	}
	remediationJSON, err := marshalRemediationTargets(ex.RemediationTargets)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	updateBuilder := tx.Exercise.UpdateOneID(id).
		SetTitle(ex.Title).
		SetPrompt(promptJSON).
		SetSkillTags(ex.SkillTags).
		SetNillableImageURL(ex.ImageURL).
		SetNillableAudioURL(ex.AudioURL).
		SetNillableEstimatedDurationSeconds(ex.EstimatedDurationSeconds)
	if remediationJSON != nil {
		updateBuilder = updateBuilder.SetRemediationTargets(*remediationJSON)
	} else {
		updateBuilder = updateBuilder.ClearRemediationTargets()
	}
	_, err = updateBuilder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return rollback(tx, domain.ErrNotFound)
		}
		return rollback(tx, err)
	}

	if _, err := tx.ExerciseOption.Delete().Where(exerciseoption.ExerciseID(id)).Exec(ctx); err != nil {
		return rollback(tx, err)
	}

	optionBuilders, err := buildExerciseOptionCreates(tx, id, ex.Options)
	if err != nil {
		return rollback(tx, err)
	}
	if len(optionBuilders) > 0 {
		if _, err := tx.ExerciseOption.CreateBulk(optionBuilders...).Save(ctx); err != nil {
			return rollback(tx, err)
		}
	}

	return tx.Commit()
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
		Prompt:                   unmarshalPrompt(row.Prompt),
		ExerciseType:             domain.ExerciseType(row.ExerciseType),
		SkillTags:                row.SkillTags,
		ImageURL:                 row.ImageURL,
		AudioURL:                 row.AudioURL,
		EstimatedDurationSeconds: row.EstimatedDurationSeconds,
		RemediationTargets:       unmarshalRemediationTargets(row.RemediationTargets),
		Options:                  options,
		ChallengeIDs:             challengeIDs,
		ContentNodeIDs:           contentNodeIDs,
		CreatedAt:                row.CreatedAt,
	}
}

// marshalRemediationTargets serializes targets to the JSON text stored in
// the exercise table's remediation_targets column. An empty/nil slice
// marshals to nil (column left unset) rather than the literal string "[]",
// keeping "no remediation configured" indistinguishable in storage from
// "explicitly configured as empty" — the two have no different meaning.
func marshalRemediationTargets(targets []domain.RemediationTarget) (*string, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(targets)
	if err != nil {
		return nil, err
	}
	s := string(data)
	return &s, nil
}

// unmarshalRemediationTargets parses the exercise table's
// remediation_targets column back into a []domain.RemediationTarget. A nil
// column (never configured) or malformed JSON both yield an empty slice
// rather than an error — this column has no legacy pre-JSON data the way
// prompt does, so any unparseable value is a storage bug, not a shimmable
// legacy shape; failing softly here avoids turning a read of an otherwise-
// valid exercise into a hard error.
func unmarshalRemediationTargets(stored *string) []domain.RemediationTarget {
	if stored == nil {
		return nil
	}
	var targets []domain.RemediationTarget
	if err := json.Unmarshal([]byte(*stored), &targets); err != nil {
		return nil
	}
	return targets
}

func toDomainOption(row *ent.ExerciseOption) domain.Option {
	opt := domain.Option{
		ID:        row.ID.String(),
		IsCorrect: row.IsCorrect,
		Label:     row.Label,
		ImageURL:  row.ImageURL,
		AudioURL:  row.AudioURL,
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

// marshalPrompt serializes prompt to the JSON text stored in the exercise
// table's prompt column.
func marshalPrompt(prompt domain.PromptDocument) (string, error) {
	data, err := json.Marshal(prompt)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// unmarshalPrompt parses the exercise table's prompt column back into a
// domain.PromptDocument. A row written before prompts became structured
// documents holds its original plain text, which is not valid
// PromptDocument JSON — such a value is wrapped as a single-paragraph
// document instead of failing the read.
func unmarshalPrompt(stored string) domain.PromptDocument {
	var prompt domain.PromptDocument
	if err := json.Unmarshal([]byte(stored), &prompt); err != nil {
		return domain.NewPlainTextPrompt(stored)
	}
	return prompt
}
