package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestCheckChallenge(t *testing.T) {
	plain := domain.PlaceMark{}
	star := domain.PlaceMark{Shape: "star"}
	root := domain.PlaceMark{Label: "R"}
	place := func(str, fret int, mark domain.PlaceMark) domain.StimulusPlace {
		return domain.StimulusPlace{Place: domain.Place{String: str, Fret: fret}, Mark: mark}
	}
	given := func(p domain.StimulusPlace) domain.StimulusPlace { p.Given = true; return p }
	at := func(str, fret int) domain.Place { return domain.Place{String: str, Fret: fret} }
	option := func(key, label string) domain.StimulusOption {
		return domain.StimulusOption{Key: key, Labels: []string{label}}
	}

	tests := []struct {
		name     string
		stimulus domain.DrillStimulus
		broken   error
	}{
		{
			name: "a degree to find among places drawn alike, the given root labelled",
			stimulus: domain.DrillStimulus{
				Prompt: []string{"3"},
				Places: []domain.StimulusPlace{given(place(5, 3, root)), place(4, 5, plain), place(2, 5, plain)},
				Answer: domain.DrillAnswer{Places: []domain.Place{at(2, 5)}},
			},
		},
		{
			name: "roots to tap drawn as stars mark the answer",
			stimulus: domain.DrillStimulus{
				Places: []domain.StimulusPlace{place(5, 3, star), place(4, 5, plain), place(2, 5, plain)},
				Answer: domain.DrillAnswer{Places: []domain.Place{at(5, 3)}},
			},
			broken: domain.ErrMarkedAnswer,
		},
		{
			name: "an answer that is the only place drawn besides the given ones is marked by being drawn",
			stimulus: domain.DrillStimulus{
				Places: []domain.StimulusPlace{given(place(5, 3, root)), place(4, 5, plain)},
				Answer: domain.DrillAnswer{Places: []domain.Place{at(4, 5)}},
			},
			broken: domain.ErrMarkedAnswer,
		},
		{
			name: "a place to find that isn't drawn at all gives nothing away",
			stimulus: domain.DrillStimulus{
				Prompt: []string{"string 6"},
				Answer: domain.DrillAnswer{Places: []domain.Place{at(6, 3)}, Labels: []string{"G"}},
			},
		},
		{
			name: "an option to pick among its family",
			stimulus: domain.DrillStimulus{
				Options: []domain.StimulusOption{option("C", "C shape"), option("A", "A shape"), option("E", "E shape")},
				Answer:  domain.DrillAnswer{Options: []string{"A"}, Labels: []string{"A shape"}},
			},
		},
		{
			name: "a single option leaves nothing to choose",
			stimulus: domain.DrillStimulus{
				Options: []domain.StimulusOption{option("A", "A shape")},
				Answer:  domain.DrillAnswer{Options: []string{"A"}},
			},
			broken: domain.ErrNoRealChoice,
		},
		{
			name: "an answer that isn't among the options can't be picked",
			stimulus: domain.DrillStimulus{
				Options: []domain.StimulusOption{option("C", "C shape"), option("E", "E shape")},
				Answer:  domain.DrillAnswer{Options: []string{"A"}},
			},
			broken: domain.ErrNoRealChoice,
		},
		{
			name: "an option offered twice narrows the choice",
			stimulus: domain.DrillStimulus{
				Options: []domain.StimulusOption{option("A", "A shape"), option("A", "A shape")},
				Answer:  domain.DrillAnswer{Options: []string{"A"}},
			},
			broken: domain.ErrNoRealChoice,
		},
		{
			name: "a prompt naming the answer gives it away",
			stimulus: domain.DrillStimulus{
				Prompt:  []string{"G", "vi", "Em"},
				Options: []domain.StimulusOption{option("Em", "Em"), option("C", "C"), option("Am", "Am"), option("D", "D")},
				Answer:  domain.DrillAnswer{Options: []string{"Em"}, Labels: []string{"Em"}},
			},
			broken: domain.ErrAnswerInWording,
		},
		{
			name: "another option's name holding the answer gives it away",
			stimulus: domain.DrillStimulus{
				Options: []domain.StimulusOption{option("A", "A shape"), option("C", "C shape, not the A shape")},
				Answer:  domain.DrillAnswer{Options: []string{"A"}, Labels: []string{"A shape"}},
			},
			broken: domain.ErrAnswerInWording,
		},
		{
			name: "the answer's words are matched whatever their case",
			stimulus: domain.DrillStimulus{
				Prompt: []string{"tap the a shape"},
				Answer: domain.DrillAnswer{Places: []domain.Place{at(6, 5)}, Labels: []string{"A shape"}},
			},
			broken: domain.ErrAnswerInWording,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.CheckChallenge(tt.stimulus)
			if tt.broken == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.broken)
		})
	}
}
