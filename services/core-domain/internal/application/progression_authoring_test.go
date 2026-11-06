package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// memoryProgressions is a ports.ProgressionRepository a test can create in.
type memoryProgressions struct {
	fakeProgressions
	createErr error
}

func (m *memoryProgressions) List(context.Context) ([]domain.Progression, error) {
	return m.fakeProgressions, nil
}

func (m *memoryProgressions) GetByID(_ context.Context, id string) (domain.Progression, error) {
	i := slices.IndexFunc(m.fakeProgressions, func(p domain.Progression) bool { return p.ID == id })
	if i < 0 {
		return domain.Progression{}, domain.ErrNotFound
	}
	return m.fakeProgressions[i], nil
}

func (m *memoryProgressions) FindByNumerals(_ context.Context, mode string, bars []string) (domain.Progression, error) {
	i := slices.IndexFunc(m.fakeProgressions, func(p domain.Progression) bool { return p.Mode == mode && slices.Equal(p.Bars, bars) })
	if i < 0 {
		return domain.Progression{}, domain.ErrNotFound
	}
	return m.fakeProgressions[i], nil
}

func (m *memoryProgressions) Create(_ context.Context, p domain.Progression) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.fakeProgressions = append(m.fakeProgressions, p)
	return nil
}

func (m *memoryProgressions) SaveNames(_ context.Context, p domain.Progression) error {
	for i := range m.fakeProgressions {
		if m.fakeProgressions[i].ID == p.ID {
			m.fakeProgressions[i] = p
		}
	}
	return nil
}

func (m *memoryProgressions) Delete(_ context.Context, id string) error {
	m.fakeProgressions = slices.DeleteFunc(m.fakeProgressions, func(p domain.Progression) bool { return p.ID == id })
	return nil
}

// fixedProgressionUses is a ports.ProgressionUseReader with the given sections.
type fixedProgressionUses []domain.ProgressionUse

func (f fixedProgressionUses) ProgressionUses(context.Context) ([]domain.ProgressionUse, error) {
	return slices.Clone(f), nil
}

var (
	progressionAdmin = domain.User{ID: "admin-1", Role: domain.RoleAdmin}
	progressionNow   = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
)

func catalogPop() domain.Progression {
	return domain.Progression{ID: "pr-pop", Key: "I-V-vi-IV", Mode: "major", Bars: []string{"I", "V", "vi", "IV"}, InDrills: true}
}

func progressionService(repo *memoryProgressions, uses fixedProgressionUses) *application.ProgressionCatalogService {
	return application.NewProgressionCatalogService(repo, uses, newFakeLanguageRepository(), newFakeKnowledgeNodeRepository(), newFakePracticeReferenceWriter(), func() string { return "new-id" }, func() time.Time { return progressionNow })
}

func TestProgressionCatalogService_Create(t *testing.T) {
	ctx := context.Background()
	rock := application.ProgressionInput{Mode: "major", Bars: []string{"I", "bVII", "IV", "I"}}

	t.Run("an admin creates a chart-only progression, named from its numerals", func(t *testing.T) {
		repo := &memoryProgressions{}

		got, err := progressionService(repo, nil).Create(ctx, progressionAdmin, rock)

		require.NoError(t, err)
		assert.Equal(t, "major/I-bVII-IV-I", got.Key)
		assert.Equal(t, []string{"I", "♭VII", "IV", "I"}, got.Bars)
		assert.False(t, got.InDrills)
		assert.Equal(t, "I–♭VII–IV–I", got.Names["en"])
		assert.Equal(t, "new-id", got.ID)
		assert.Equal(t, &progressionAdmin.ID, got.CreatedBy)
		assert.Equal(t, &progressionNow, got.CreatedAt)
		assert.Len(t, repo.fakeProgressions, 1)
	})

	t.Run("a teacher is refused", func(t *testing.T) {
		_, err := progressionService(&memoryProgressions{}, nil).Create(ctx, domain.User{Role: domain.RoleTeacher}, rock)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("the same numerals return the existing one, with its chart count", func(t *testing.T) {
		repo := &memoryProgressions{fakeProgressions: fakeProgressions{catalogPop()}}
		uses := fixedProgressionUses{{ProgressionKey: "I-V-vi-IV", SongChartID: "c1"}}

		_, err := progressionService(repo, uses).Create(ctx, progressionAdmin, application.ProgressionInput{Mode: "major", Bars: []string{"I", "V", "vi", "IV"}})

		var exists *domain.ProgressionExistsError
		require.ErrorAs(t, err, &exists)
		assert.Equal(t, "I-V-vi-IV", exists.Existing.Key)
		assert.Equal(t, 1, exists.Existing.SongChartCount)
	})

	t.Run("the same chords spelled differently are another progression: harmonic analysis reads them apart", func(t *testing.T) {
		repo := &memoryProgressions{}
		service := progressionService(repo, nil)
		_, err := service.Create(ctx, progressionAdmin, rock)
		require.NoError(t, err)

		twin, err := service.Create(ctx, progressionAdmin, application.ProgressionInput{Mode: "major", Bars: []string{"I", "#VI", "IV", "I"}})

		require.NoError(t, err)
		assert.Equal(t, "major/I-#VI-IV-I", twin.Key)
		assert.Len(t, repo.fakeProgressions, 2)
	})

	t.Run("losing a race to another admin returns their progression", func(t *testing.T) {
		repo := &racingProgressions{memoryProgressions: memoryProgressions{createErr: domain.ErrConflict}}
		service := application.NewProgressionCatalogService(repo, fixedProgressionUses{}, newFakeLanguageRepository(), newFakeKnowledgeNodeRepository(), newFakePracticeReferenceWriter(), func() string { return "new-id" }, func() time.Time { return progressionNow })

		_, err := service.Create(ctx, progressionAdmin, rock)

		var exists *domain.ProgressionExistsError
		require.ErrorAs(t, err, &exists)
		assert.Equal(t, "major/theirs", exists.Existing.Key)
	})

	t.Run("a failed store is reported", func(t *testing.T) {
		boom := errors.New("db down")

		_, err := progressionService(&memoryProgressions{createErr: boom}, nil).Create(ctx, progressionAdmin, rock)

		assert.ErrorIs(t, err, boom)
	})
}

// racingProgressions finds nothing at first, then the row another admin stored
// between the check and the write.
type racingProgressions struct {
	memoryProgressions
	checked bool
}

func (r *racingProgressions) FindByNumerals(ctx context.Context, mode string, bars []string) (domain.Progression, error) {
	if !r.checked {
		r.checked = true
		return domain.Progression{}, domain.ErrNotFound
	}
	return domain.Progression{Key: mode + "/theirs", Mode: mode, Bars: bars}, nil
}

func TestProgressionCatalogService_UpdateNicknames(t *testing.T) {
	ctx := context.Background()
	creator := "admin-1"
	created := func() domain.Progression {
		return domain.Progression{ID: "pr-rock", Key: "major/I-bVII-IV-I", Mode: "major", Bars: []string{"I", "♭VII", "IV", "I"}, CreatedBy: &creator}
	}

	tests := []struct {
		name      string
		caller    domain.User
		id        string
		nicknames map[string]string
		wantErr   error
		wantName  string
	}{
		{name: "a nickname follows the numerals", caller: progressionAdmin, id: "pr-rock", nicknames: map[string]string{"en": " rock "}, wantName: "I–♭VII–IV–I (rock)"},
		{name: "no nicknames removes them", caller: progressionAdmin, id: "pr-rock", nicknames: map[string]string{}, wantName: "I–♭VII–IV–I"},
		{name: "a teacher is refused", caller: domain.User{Role: domain.RoleTeacher}, id: "pr-rock", wantErr: domain.ErrForbidden},
		{name: "an unknown progression", caller: progressionAdmin, id: "missing", wantErr: domain.ErrNotFound},
		{name: "the catalog's names stay", caller: progressionAdmin, id: "pr-pop", wantErr: domain.ErrProgressionIsCatalog},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryProgressions{fakeProgressions: fakeProgressions{catalogPop(), created()}}

			got, err := progressionService(repo, nil).UpdateNicknames(ctx, tc.caller, tc.id, tc.nicknames)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, got.Names["en"])
			assert.Equal(t, "major/I-bVII-IV-I", got.Key)
		})
	}

	t.Run("a nickname in no language is invalid", func(t *testing.T) {
		repo := &memoryProgressions{fakeProgressions: fakeProgressions{created()}}

		_, err := progressionService(repo, nil).UpdateNicknames(ctx, progressionAdmin, "pr-rock", map[string]string{"xx": "rock"})

		var invalid *domain.ValidationError
		assert.ErrorAs(t, err, &invalid)
	})
}

func TestProgressionCatalogService_Delete(t *testing.T) {
	ctx := context.Background()
	creator := "admin-1"
	rock := domain.Progression{ID: "pr-rock", Key: "major/I-bVII-IV-I", Mode: "major", Bars: []string{"I", "♭VII", "IV", "I"}, CreatedBy: &creator}
	draftUse := domain.ProgressionUse{ProgressionKey: rock.Key, SongChartID: "c1", Title: "Sweet Home", Revision: domain.ProgressionUseDraft}
	otherUse := domain.ProgressionUse{ProgressionKey: "I-V-vi-IV", SongChartID: "c2"}

	t.Run("unused, it is deleted", func(t *testing.T) {
		repo := &memoryProgressions{fakeProgressions: fakeProgressions{rock}}

		err := progressionService(repo, fixedProgressionUses{otherUse}).Delete(ctx, progressionAdmin, rock.ID)

		require.NoError(t, err)
		assert.Empty(t, repo.fakeProgressions)
	})

	t.Run("in use, it stays and the sections are listed", func(t *testing.T) {
		repo := &memoryProgressions{fakeProgressions: fakeProgressions{rock}}

		err := progressionService(repo, fixedProgressionUses{draftUse, otherUse}).Delete(ctx, progressionAdmin, rock.ID)

		var refused *domain.ProgressionNotRemovableError
		require.ErrorAs(t, err, &refused)
		assert.Equal(t, domain.ProgressionNotRemovableInUse, refused.Reason)
		assert.Equal(t, []domain.ProgressionUse{draftUse}, refused.Uses)
		assert.Len(t, repo.fakeProgressions, 1)
	})

	t.Run("a drill progression stays, whoever created it", func(t *testing.T) {
		added := rock
		added.InDrills = true
		repo := &memoryProgressions{fakeProgressions: fakeProgressions{added, catalogPop()}}
		service := progressionService(repo, nil)

		for _, id := range []string{added.ID, "pr-pop"} {
			var refused *domain.ProgressionNotRemovableError
			require.ErrorAs(t, service.Delete(ctx, progressionAdmin, id), &refused)
			assert.Equal(t, domain.ProgressionNotRemovableInDrills, refused.Reason)
		}
		assert.Len(t, repo.fakeProgressions, 2)
	})

	t.Run("a teacher is refused and an unknown one isn't found", func(t *testing.T) {
		service := progressionService(&memoryProgressions{}, nil)

		assert.ErrorIs(t, service.Delete(ctx, domain.User{Role: domain.RoleTeacher}, rock.ID), domain.ErrForbidden)
		assert.ErrorIs(t, service.Delete(ctx, progressionAdmin, "missing"), domain.ErrNotFound)
	})
}

func TestProgressionCatalogService_ListCreator(t *testing.T) {
	creator := "admin-1"
	rows := func() *memoryProgressions {
		return &memoryProgressions{fakeProgressions: fakeProgressions{{Key: "major/I", CreatedBy: &creator}}}
	}

	tests := []struct {
		name   string
		caller domain.User
		want   *string
	}{
		{"an admin sees who created it", domain.User{Role: domain.RoleAdmin}, &creator},
		{"a teacher doesn't", domain.User{Role: domain.RoleTeacher}, nil},
		{"a student doesn't", domain.User{Role: domain.RoleStudent}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := rows()

			got, err := progressionService(repo, nil).List(context.Background(), tc.caller)

			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0].CreatedBy)
			assert.Equal(t, &creator, repo.fakeProgressions[0].CreatedBy, "the stored row keeps its creator")
		})
	}
}

func TestProgressionCatalogService_ListCountsCharts(t *testing.T) {
	repo := &memoryProgressions{fakeProgressions: fakeProgressions{catalogPop()}}
	uses := fixedProgressionUses{
		{ProgressionKey: "I-V-vi-IV", SongChartID: "c1", SectionIndex: 0},
		{ProgressionKey: "I-V-vi-IV", SongChartID: "c1", SectionIndex: 1},
		{ProgressionKey: "I-V-vi-IV", SongChartID: "c2"},
	}

	got, err := progressionService(repo, uses).List(context.Background(), progressionAdmin)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].SongChartCount, "two sections of one chart count once")
}

func TestProgressionCatalogService_CreateInDrills(t *testing.T) {
	ctx := context.Background()
	input := application.ProgressionInput{Mode: "major", Bars: []string{"I", "♭VII", "IV", "V"}, InDrills: true, ConceptIDs: []string{"common-progressions"}}

	fixture := func() (*application.ProgressionCatalogService, *memoryProgressions, *fakePracticeReferenceWriter) {
		repo := &memoryProgressions{fakeProgressions: fakeProgressions{catalogPop()}}
		nodes := newFakeKnowledgeNodeRepository()
		nodes.put(domain.KnowledgeNode{ID: "common-progressions", Kind: domain.KnowledgeNodeKindConcept, Key: "common-progressions"})
		nodes.put(domain.KnowledgeNode{ID: "target-chord-tones", Kind: domain.KnowledgeNodeKindSkill, Key: "target-chord-tones"})
		references := newFakePracticeReferenceWriter()
		service := application.NewProgressionCatalogService(repo, fixedProgressionUses{}, newFakeLanguageRepository(), nodes, references, func() string { return "new-id" }, func() time.Time { return progressionNow })
		return service, repo, references
	}

	t.Run("it is stored in the drills, with its nodes and wrong answers", func(t *testing.T) {
		service, repo, _ := fixture()
		both := input
		both.SkillIDs = []string{"target-chord-tones"}

		got, err := service.Create(ctx, progressionAdmin, both)

		require.NoError(t, err)
		assert.True(t, got.InDrills)
		assert.ElementsMatch(t, []string{"target-chord-tones", "common-progressions"}, got.NodeIDs)
		assert.Equal(t, []string{"I-V-vi-IV"}, got.Confusable)
		assert.Equal(t, got, repo.fakeProgressions[1])
	})

	t.Run("it reaches the practice snapshot", func(t *testing.T) {
		service, _, references := fixture()

		got, err := service.Create(ctx, progressionAdmin, input)

		require.NoError(t, err)
		require.Len(t, references.progs, 1)
		assert.Equal(t, got.Key, references.progs[0].Key)
	})

	t.Run("a progression for charts only stays out of the snapshot and takes no nodes", func(t *testing.T) {
		service, _, references := fixture()
		chartOnly := input
		chartOnly.InDrills = false

		got, err := service.Create(ctx, progressionAdmin, chartOnly)

		require.NoError(t, err)
		assert.Empty(t, references.progs)
		assert.False(t, got.InDrills)
		assert.Empty(t, got.NodeIDs)
	})

	t.Run("a snapshot that can't be written doesn't undo the progression", func(t *testing.T) {
		service, repo, references := fixture()
		references.err = errors.New("mongo down")

		_, err := service.Create(ctx, progressionAdmin, input)

		require.NoError(t, err)
		assert.Len(t, repo.fakeProgressions, 2)
	})

	refused := []struct {
		name  string
		in    func(application.ProgressionInput) application.ProgressionInput
		field string
	}{
		{"no skill and no concept", func(in application.ProgressionInput) application.ProgressionInput {
			in.ConceptIDs = nil
			return in
		}, "skill_ids"},
		{"a skill that doesn't exist", func(in application.ProgressionInput) application.ProgressionInput {
			in.SkillIDs = []string{"nope"}
			return in
		}, "skill_ids"},
		{"a concept that doesn't exist", func(in application.ProgressionInput) application.ProgressionInput {
			in.ConceptIDs = []string{"nope"}
			return in
		}, "concept_ids"},
		{"a skill sent as a concept", func(in application.ProgressionInput) application.ProgressionInput {
			in.ConceptIDs = []string{"target-chord-tones"}
			return in
		}, "concept_ids"},
	}
	for _, tt := range refused {
		t.Run("invalid: "+tt.name, func(t *testing.T) {
			service, repo, references := fixture()

			_, err := service.Create(ctx, progressionAdmin, tt.in(input))

			var invalid *domain.ValidationError
			require.ErrorAs(t, err, &invalid)
			require.NotEmpty(t, invalid.Fields)
			assert.Equal(t, tt.field, invalid.Fields[0].Field)
			assert.Len(t, repo.fakeProgressions, 1, "nothing is created")
			assert.Empty(t, references.progs)
		})
	}

	t.Run("what can't be added creates nothing and says why", func(t *testing.T) {
		service, repo, references := fixture()
		threeBars := input
		threeBars.Bars = []string{"I", "IV", "V"}

		_, err := service.Create(ctx, progressionAdmin, threeBars)

		var notAddable *domain.ProgressionNotAddableError
		require.ErrorAs(t, err, &notAddable)
		assert.Contains(t, notAddable.Reasons, domain.ProgressionNotAddableUnsupportedBarCount)
		assert.Len(t, repo.fakeProgressions, 1)
		assert.Empty(t, references.progs)
	})

	t.Run("numerals another progression has are refused as existing, not as unaddable", func(t *testing.T) {
		service, _, _ := fixture()
		same := input
		same.Bars = catalogPop().Bars

		_, err := service.Create(ctx, progressionAdmin, same)

		var exists *domain.ProgressionExistsError
		require.ErrorAs(t, err, &exists)
	})

	t.Run("wrong answers come only from the drills", func(t *testing.T) {
		service, repo, _ := fixture()
		repo.fakeProgressions = append(repo.fakeProgressions, domain.Progression{ID: "chart", Key: "major/I-iii-IV-V", Mode: "major", Bars: []string{"I", "iii", "IV", "V"}})

		got, err := service.Create(ctx, progressionAdmin, input)

		require.NoError(t, err)
		assert.Equal(t, []string{"I-V-vi-IV"}, got.Confusable)
	})
}
