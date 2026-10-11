package domain

import (
	"encoding/json"
	"fmt"
)

// StartingPointExperience is how much a student has played an instrument, as they answer it on
// their first visit.
type StartingPointExperience string

const (
	StartingPointExperienceNeverPlayed StartingPointExperience = "never_played"
	StartingPointExperiencePlaysABit   StartingPointExperience = "plays_a_bit"
	StartingPointExperiencePlaysWell   StartingPointExperience = "plays_well"
)

// Levels are the catalog levels a student with this experience starts at; nil for an unknown
// experience.
func (e StartingPointExperience) Levels() []DifficultyLevel {
	switch e {
	case StartingPointExperienceNeverPlayed:
		return []DifficultyLevel{DifficultyLevelBeginner}
	case StartingPointExperiencePlaysABit:
		return []DifficultyLevel{DifficultyLevelEarlyIntermediate, DifficultyLevelIntermediate}
	case StartingPointExperiencePlaysWell:
		return []DifficultyLevel{DifficultyLevelAdvanced, DifficultyLevelExpert}
	}
	return nil
}

// Valid reports whether e is one of the known experiences.
func (e StartingPointExperience) Valid() bool { return e.Levels() != nil }

// StartingPointKind says whether a starting point is a learning path or a course.
type StartingPointKind string

const (
	StartingPointKindPath   StartingPointKind = "path"
	StartingPointKindCourse StartingPointKind = "course"
)

// StartingPointRef names one path or course in a list of starting points.
type StartingPointRef struct {
	Kind StartingPointKind `json:"kind"`
	ID   string            `json:"id"`
}

// StartingPointsList is the starting points the team chose for one instrument, experience and
// language, first shown first.
type StartingPointsList struct {
	InstrumentID string                  `json:"instrument_id"`
	Experience   StartingPointExperience `json:"experience"`
	Language     string                  `json:"language"`
	Items        []StartingPointRef      `json:"items"`
}

// StartingPointsConfig is the value of the starting points setting: every list, at most one per
// instrument, experience and language.
type StartingPointsConfig struct {
	Lists []StartingPointsList `json:"lists"`
}

// ListFor returns the list for instrumentID, experience and language, if there is one.
func (c StartingPointsConfig) ListFor(instrumentID string, experience StartingPointExperience, language string) (StartingPointsList, bool) {
	for _, list := range c.Lists {
		if list.InstrumentID == instrumentID && list.Experience == experience && list.Language == language {
			return list, true
		}
	}
	return StartingPointsList{}, false
}

// The wire shapes keep every field a pointer, so a missing field can be told apart from an empty
// one.
type startingPointRefWire struct {
	Kind *string `json:"kind"`
	ID   *string `json:"id"`
}

type startingPointsListWire struct {
	InstrumentID *string                 `json:"instrument_id"`
	Experience   *string                 `json:"experience"`
	Language     *string                 `json:"language"`
	Items        *[]startingPointRefWire `json:"items"`
}

type startingPointsConfigWire struct {
	Lists *[]startingPointsListWire `json:"lists"`
}

// ParseStartingPointsConfig decodes a starting points value and checks its shape: every list has
// an instrument_id (a UUID), a known experience, a language and at least one entry; every entry a
// known kind and a UUID id; no entry repeats within a list, and no two lists share an instrument,
// experience and language. It reports every problem at once, each named by its path in the value
// (lists[1].items[0].kind). Whether the ids name real, published things is the caller's to check.
func ParseStartingPointsConfig(raw []byte) (StartingPointsConfig, []FieldError) {
	var wire startingPointsConfigWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return StartingPointsConfig{}, []FieldError{{Field: "value", Reason: "must be a JSON object with a lists array: " + err.Error()}}
	}
	if wire.Lists == nil {
		return StartingPointsConfig{}, []FieldError{{Field: "lists", Reason: "is required"}}
	}

	var errs []FieldError
	config := StartingPointsConfig{Lists: make([]StartingPointsList, 0, len(*wire.Lists))}
	seen := map[[3]string]bool{}
	for i, w := range *wire.Lists {
		at := fmt.Sprintf("lists[%d]", i)
		list, listErrs := parseStartingPointsList(at, w)
		errs = append(errs, listErrs...)
		if len(listErrs) == 0 {
			combo := [3]string{list.InstrumentID, string(list.Experience), list.Language}
			if seen[combo] {
				errs = append(errs, FieldError{Field: at, Reason: "another list already has this instrument_id, experience and language"})
			}
			seen[combo] = true
		}
		config.Lists = append(config.Lists, list)
	}
	if len(errs) > 0 {
		return StartingPointsConfig{}, errs
	}
	return config, nil
}

func parseStartingPointsList(at string, w startingPointsListWire) (StartingPointsList, []FieldError) {
	var errs []FieldError
	var list StartingPointsList

	switch {
	case w.InstrumentID == nil:
		errs = append(errs, FieldError{Field: at + ".instrument_id", Reason: "is required"})
	case !uuidPattern.MatchString(*w.InstrumentID):
		errs = append(errs, FieldError{Field: at + ".instrument_id", Reason: "must be a UUID"})
	default:
		list.InstrumentID = *w.InstrumentID
	}

	switch {
	case w.Experience == nil:
		errs = append(errs, FieldError{Field: at + ".experience", Reason: "is required"})
	case !StartingPointExperience(*w.Experience).Valid():
		errs = append(errs, FieldError{Field: at + ".experience", Reason: "must be never_played, plays_a_bit or plays_well"})
	default:
		list.Experience = StartingPointExperience(*w.Experience)
	}

	if w.Language == nil || *w.Language == "" {
		errs = append(errs, FieldError{Field: at + ".language", Reason: "is required"})
	} else {
		list.Language = *w.Language
	}

	if w.Items == nil || len(*w.Items) == 0 {
		errs = append(errs, FieldError{Field: at + ".items", Reason: "must hold at least one path or course"})
		return list, errs
	}
	seen := map[StartingPointRef]bool{}
	for j, item := range *w.Items {
		itemAt := fmt.Sprintf("%s.items[%d]", at, j)
		ref, itemErrs := parseStartingPointRef(itemAt, item)
		errs = append(errs, itemErrs...)
		if len(itemErrs) == 0 {
			if seen[ref] {
				errs = append(errs, FieldError{Field: itemAt, Reason: "repeats an earlier entry of this list"})
			}
			seen[ref] = true
		}
		list.Items = append(list.Items, ref)
	}
	return list, errs
}

func parseStartingPointRef(at string, w startingPointRefWire) (StartingPointRef, []FieldError) {
	var errs []FieldError
	var ref StartingPointRef
	switch {
	case w.Kind == nil:
		errs = append(errs, FieldError{Field: at + ".kind", Reason: "is required"})
	case *w.Kind != string(StartingPointKindPath) && *w.Kind != string(StartingPointKindCourse):
		errs = append(errs, FieldError{Field: at + ".kind", Reason: "must be path or course"})
	default:
		ref.Kind = StartingPointKind(*w.Kind)
	}
	switch {
	case w.ID == nil:
		errs = append(errs, FieldError{Field: at + ".id", Reason: "is required"})
	case !uuidPattern.MatchString(*w.ID):
		errs = append(errs, FieldError{Field: at + ".id", Reason: "must be a UUID"})
	default:
		ref.ID = *w.ID
	}
	return ref, errs
}
