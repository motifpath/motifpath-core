package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// The ways a generated question fails to challenge the student: each lets a
// right answer come from the screen instead of the skill.
var (
	// ErrMarkedAnswer is a question that draws its answer apart from the
	// other places: by a label, a marker shape, a colour, a highlight or a
	// size of its own, or by being all that is drawn besides what it gives.
	ErrMarkedAnswer = errors.New("the question marks its answer")
	// ErrNoRealChoice is a question asked among options that doesn't offer
	// its answer among two or more distinct options.
	ErrNoRealChoice = errors.New("the question offers no real choice")
	// ErrAnswerInWording is a question whose prompt, or another option's
	// name, says its answer.
	ErrAnswerInWording = errors.New("the question's wording says its answer")
)

// Place is a string and a fret on a fretboard, 1 being the highest-pitched
// string and 0 the open string.
type Place struct {
	String int
	Fret   int
}

// PlaceMark is how a place is drawn: the zero value is a plain marker.
type PlaceMark struct {
	Label     string
	Shape     string
	Colour    string
	Highlight bool
	Size      string
}

// StimulusPlace is a place a question draws. Given is true for what the
// question gives to answer from, such as the root a degree is counted
// from, which may be marked and is never asked.
type StimulusPlace struct {
	Place
	Mark  PlaceMark
	Given bool
}

// StimulusOption is an option a question offers: its key, sent back as the
// answer, and its name in each language it is shown in.
type StimulusOption struct {
	Key    string
	Labels []string
}

// DrillAnswer is what answers a question right: the places to tap or the
// options to pick, and the words that name the answer in each language.
type DrillAnswer struct {
	Places  []Place
	Options []string
	Labels  []string
}

// DrillStimulus is what a planned question shows the student, as the client
// draws it, and what answers it: the words of its prompt that vary with the
// item (a note, a key, a numeral), the places it draws and the options it
// offers.
type DrillStimulus struct {
	Prompt  []string
	Places  []StimulusPlace
	Options []StimulusOption
	Answer  DrillAnswer
}

// CheckChallenge reports why a planned question doesn't challenge the
// student: it marks its answer (ErrMarkedAnswer), offers no real choice
// among its options (ErrNoRealChoice), or says its answer in its wording
// (ErrAnswerInWording). nil when it challenges.
func CheckChallenge(s DrillStimulus) error {
	if err := checkUnmarked(s); err != nil {
		return err
	}
	if err := checkChoice(s); err != nil {
		return err
	}
	return checkWording(s)
}

// checkUnmarked holds when every place an answer taps that the question
// draws is drawn like every other place it doesn't give, and some such place
// isn't an answer.
func checkUnmarked(s DrillStimulus) error {
	var answers, others []StimulusPlace
	for _, p := range s.Places {
		switch {
		case p.Given:
		case slices.Contains(s.Answer.Places, p.Place):
			answers = append(answers, p)
		default:
			others = append(others, p)
		}
	}
	if len(answers) == 0 {
		return nil
	}
	if len(others) == 0 {
		return fmt.Errorf("%w: its answer is all it draws besides what it gives", ErrMarkedAnswer)
	}
	mark := others[0].Mark
	for _, p := range slices.Concat(answers, others) {
		if p.Mark != mark {
			return fmt.Errorf("%w: string %d fret %d is drawn apart", ErrMarkedAnswer, p.String, p.Fret)
		}
	}
	return nil
}

// checkChoice holds for a question with no options, and for one offering
// its every answer among two or more options with distinct keys.
func checkChoice(s DrillStimulus) error {
	if len(s.Options) == 0 && len(s.Answer.Options) == 0 {
		return nil
	}
	keys := make([]string, len(s.Options))
	for i, o := range s.Options {
		if slices.Contains(keys[:i], o.Key) {
			return fmt.Errorf("%w: it offers %q twice", ErrNoRealChoice, o.Key)
		}
		keys[i] = o.Key
	}
	if len(keys) < 2 {
		return fmt.Errorf("%w: it offers %d option", ErrNoRealChoice, len(keys))
	}
	for _, a := range s.Answer.Options {
		if !slices.Contains(keys, a) {
			return fmt.Errorf("%w: its answer %q isn't offered", ErrNoRealChoice, a)
		}
	}
	return nil
}

// checkWording holds when no answer label is said, as whole words, by the
// prompt or by an option that isn't an answer.
func checkWording(s DrillStimulus) error {
	var texts []string
	texts = append(texts, s.Prompt...)
	for _, o := range s.Options {
		if !slices.Contains(s.Answer.Options, o.Key) {
			texts = append(texts, o.Labels...)
		}
	}
	for _, label := range s.Answer.Labels {
		said := words(label)
		if len(said) == 0 {
			continue
		}
		for _, text := range texts {
			if containsRun(words(text), said) {
				return fmt.Errorf("%w: %q says %q", ErrAnswerInWording, text, label)
			}
		}
	}
	return nil
}

// words splits text into its lower-case words: runs of anything but spaces
// and punctuation, so a chord symbol such as F#m or vii° is one word.
func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",;:()–—", r)
	})
}

// containsRun reports whether run appears in items, in order and side by
// side.
func containsRun[T comparable](items, run []T) bool {
	for i := 0; i+len(run) <= len(items); i++ {
		if slices.Equal(items[i:i+len(run)], run) {
			return true
		}
	}
	return false
}
