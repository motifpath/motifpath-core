package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// countingProgressions is a ports.ProgressionRepository over progressions
// that counts its reads.
type countingProgressions struct {
	progressions fakeProgressions
	reads        int
}

func (c *countingProgressions) List(ctx context.Context) ([]domain.Progression, error) {
	c.reads++
	return c.progressions.List(ctx)
}

// countingChordDrills is a ports.ChordDrillRepository over catalog that
// counts its reads.
type countingChordDrills struct {
	catalog domain.ChordDrillCatalog
	reads   int
}

func (c *countingChordDrills) Catalog(context.Context) (domain.ChordDrillCatalog, error) {
	c.reads++
	return c.catalog, nil
}

// A list of subjects reads every item the student's knowledge maps hold, so
// what it reads must not grow with them: the diagrams come in one read, and
// each instrument and the progression catalog are read once.
func TestPracticeSessionService_ListSubjectsReads(t *testing.T) {
	t.Run("the diagrams come in one read, however many there are", func(t *testing.T) {
		f := newPracticeFixture(t)
		for i := range 4 {
			f.playAlong(fmt.Sprintf("riff-%d", i), "Riff", practiceGuitar, "riffs", 100)
			f.shapeOn(fmt.Sprintf("grip-%d", i), "grips", practiceGuitar)
		}
		f.onPathFor([]string{practiceGuitar}, "riffs")

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), guitar())

		require.NoError(t, err)
		require.NotEmpty(t, list.Groups)
		assert.Zero(t, f.diagrams.gets, "no diagram is read alone")
		assert.Equal(t, 1, f.diagrams.batchReads)
	})

	t.Run("in the head, the diagrams of every instrument come in one read", func(t *testing.T) {
		f := newPracticeFixture(t)
		for i := range 3 {
			f.shapeOn(fmt.Sprintf("grip-%d", i), "grips", practiceGuitar)
			f.shapeOn(fmt.Sprintf("bass-grip-%d", i), "grips", practiceBass)
		}
		f.onPathFor([]string{practiceGuitar, practiceBass}, "grips")

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), nil)

		require.NoError(t, err)
		require.NotEmpty(t, list.Groups)
		assert.Zero(t, f.diagrams.gets)
		assert.Equal(t, 1, f.diagrams.batchReads)
	})

	// readsFor lists the subjects, in hand on guitar or in the head, with
	// every progression a guide tone and a key tour on both layouts, and
	// tells what was read.
	type reads struct{ instruments, progressions, chords int }
	readsFor := func(t *testing.T, instrumentID *string, progressions ...string) reads {
		f := newPracticeFixture(t)
		f.instruments.put(domain.Instrument{ID: practiceGuitar, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}})
		f.instruments.put(domain.Instrument{ID: practiceBass, Tuning: []string{"E1", "A1", "D2", "G2"}})
		f.skill(targetChordTones)
		f.onPathFor([]string{practiceGuitar, practiceBass}, targetChordTones)
		for _, layout := range []string{practiceGuitar, practiceBass} {
			for _, p := range progressions {
				f.cells[layout] = append(f.cells[layout],
					domain.ClassifiedItem{ItemKey: guideToneItemKey(layout, p), NodeIDs: []string{targetChordTones}},
					domain.ClassifiedItem{ItemKey: tourItemKey(layout, p), NodeIDs: []string{targetChordTones}},
				)
			}
		}
		catalog := &countingProgressions{progressions: guideToneProgressions}
		chords := harmonyChordCatalog()
		f.svc.WithHarmonyDrills(catalog, chords, fakeFretboardLayouts{
			practiceGuitar: {{ID: practiceGuitar}},
			practiceBass:   {{ID: practiceBass}},
		})
		f.svc.WithGuideTones(f.instruments).WithProgressionKeyTours(f.instruments)

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), instrumentID)

		require.NoError(t, err)
		require.NotEmpty(t, list.Groups)
		instruments := 0
		for _, n := range f.instruments.gets {
			instruments += n
		}
		return reads{instruments: instruments, progressions: catalog.reads, chords: chords.finds}
	}

	t.Run("guide tones and key tours read no more with more of them, in hand", func(t *testing.T) {
		few := readsFor(t, guitar(), "ii-V-I")
		many := readsFor(t, guitar(), "ii-V-I", "IV-V-I", "I-V-vi-IV", "I-vi-IV-V", "12-bar-blues")

		assert.Equal(t, few, many)
	})

	t.Run("guide tones and key tours read no more with more of them, in the head", func(t *testing.T) {
		few := readsFor(t, nil, "ii-V-I")
		many := readsFor(t, nil, "ii-V-I", "IV-V-I", "I-V-vi-IV", "I-vi-IV-V", "12-bar-blues")

		assert.Equal(t, few, many)
	})

	t.Run("the exercises come in one read, however many there are", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.exerciseBatch("ex", "progressions", 5, 60)
		f.onPathFor([]string{practiceGuitar}, "progressions")

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), guitar())

		require.NoError(t, err)
		require.NotEmpty(t, list.Groups)
		assert.Equal(t, 5, list.Groups[0].Subjects[0].ItemCount)
		assert.Zero(t, f.exercises.gets, "no exercise is read alone")
		assert.Equal(t, 1, f.exercises.batchReads)
	})

	// chordReadsFor lists the subjects in hand on instrumentID with chords as
	// single chords and root-fifth lines, and every pair of them as a
	// change, and tells how often the chord drill catalog and the
	// instruments were read.
	chordReadsFor := func(t *testing.T, instrumentID string, chords ...string) (catalogReads, instrumentReads int) {
		f := newPracticeFixture(t)
		f.instruments.put(domain.Instrument{ID: practiceBass, Tuning: []string{"E1", "A1", "D2", "G2"}})
		for _, skill := range []string{"play-open-chords", "change-chords", "play-root-fifth-lines", "play-root-notes"} {
			f.skill(skill)
		}
		f.onPathFor([]string{practiceGuitar}, "play-open-chords", "change-chords")
		f.onPathFor([]string{practiceBass}, "play-root-fifth-lines", "play-root-notes")
		for i, chord := range chords {
			f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: voicingKey(chord), NodeIDs: []string{"play-open-chords"}})
			f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: "root_fifth:" + practiceBass + ":" + chord, NodeIDs: []string{"play-root-fifth-lines"}})
			for _, other := range chords[i+1:] {
				f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: changeKey(chord, other), NodeIDs: []string{"change-chords"}})
				f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: "root_change:" + practiceBass + ":" + chord + ":" + other, NodeIDs: []string{"play-root-notes"}})
			}
		}
		catalog := &countingChordDrills{catalog: chordCatalog()}
		f.svc.WithChordDrills(catalog, f.instruments)

		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), &instrumentID)

		require.NoError(t, err)
		require.NotEmpty(t, list.Groups)
		for _, n := range f.instruments.gets {
			instrumentReads += n
		}
		return catalog.reads, instrumentReads
	}

	t.Run("chord drills read the catalog once, however many there are", func(t *testing.T) {
		for _, instrumentID := range []string{practiceGuitar, practiceBass} {
			catalogReads, instrumentReads := chordReadsFor(t, instrumentID, "Am")
			manyCatalogReads, manyInstrumentReads := chordReadsFor(t, instrumentID, "Am", "C", "G", "E")

			assert.Equal(t, 1, catalogReads, instrumentID)
			assert.Equal(t, 1, manyCatalogReads, instrumentID)
			assert.Equal(t, instrumentReads, manyInstrumentReads, instrumentID)
		}
	})

	t.Run("a later list reads again, so a new diagram is found", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.shapeOn("grip-0", "grips", practiceGuitar)
		f.onPathFor([]string{practiceGuitar}, "grips")

		_, err := f.svc.ListSubjects(context.Background(), studentCaller(), nil)
		require.NoError(t, err)
		f.shapeOn("grip-1", "grips", practiceGuitar)
		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), nil)

		require.NoError(t, err)
		assert.Equal(t, 2, f.diagrams.batchReads)
		require.NotEmpty(t, list.Groups)
		assert.Equal(t, 2, list.Groups[0].Subjects[0].ItemCount)
	})
}
