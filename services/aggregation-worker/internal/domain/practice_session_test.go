package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func at(hh, mm int) time.Time { return time.Date(2026, 10, 5, hh, mm, 0, 0, time.UTC) }

func tenMinuteSessionAt(start time.Time) PracticeSession {
	return PracticeSession{}.Started(PracticeSessionStart{
		EventID:           "e-start",
		StudentID:         "alice",
		PracticeSessionID: "s1",
		OccurredAt:        start,
		InstrumentID:      "guitar",
		Minutes:           10,
		PlannedItems:      []PlannedPracticeItem{{ItemKey: "play_along:d1", Reason: "due"}},
	})
}

func end(id string, when time.Time, leftEarly bool, answered int) PracticeSessionEnd {
	return PracticeSessionEnd{EventID: id, OccurredAt: when, LeftEarly: leftEarly, AnsweredCount: answered}
}

func TestPracticeSession_StatusAt(t *testing.T) {
	cases := []struct {
		name        string
		session     PracticeSession
		readAt      time.Time
		wantStatus  PracticeSessionStatus
		wantEndedAt *time.Time
	}{
		{
			name:        "ended without leaving early is finished at its end",
			session:     tenMinuteSessionAt(at(18, 0)).Ended(end("e-end", at(18, 11), false, 6)),
			readAt:      at(23, 0),
			wantStatus:  PracticeSessionFinished,
			wantEndedAt: ptr(at(18, 11)),
		},
		{
			name:        "left early ended early at its end",
			session:     tenMinuteSessionAt(at(18, 0)).Ended(end("e-end", at(18, 4), true, 2)),
			readAt:      at(23, 0),
			wantStatus:  PracticeSessionEndedEarly,
			wantEndedAt: ptr(at(18, 4)),
		},
		{
			name:        "silent for its planned minutes plus 15 is abandoned at its last event",
			session:     tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), ""),
			readAt:      at(18, 29),
			wantStatus:  PracticeSessionAbandoned,
			wantEndedAt: ptr(at(18, 3)),
		},
		{
			name:        "silent for exactly its planned minutes plus 15 is abandoned",
			session:     tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), ""),
			readAt:      at(18, 28),
			wantStatus:  PracticeSessionAbandoned,
			wantEndedAt: ptr(at(18, 3)),
		},
		{
			name:       "within its planned minutes plus 15 is in progress",
			session:    tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), ""),
			readAt:     at(18, 27),
			wantStatus: PracticeSessionInProgress,
		},
		{
			name:        "never answered is abandoned counting from its start",
			session:     tenMinuteSessionAt(at(18, 0)),
			readAt:      at(18, 25),
			wantStatus:  PracticeSessionAbandoned,
			wantEndedAt: ptr(at(18, 0)),
		},
		{
			name: "a late event reopens an abandoned session",
			session: tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), "").
				Answered(at(18, 30), "").Ended(end("e-end", at(18, 35), false, 4)),
			readAt:      at(23, 0),
			wantStatus:  PracticeSessionFinished,
			wantEndedAt: ptr(at(18, 35)),
		},
		{
			name: "an answer delivered out of order doesn't move the last event back",
			session: tenMinuteSessionAt(at(18, 0)).Answered(at(18, 20), "").
				Answered(at(18, 3), ""),
			readAt:     at(18, 40),
			wantStatus: PracticeSessionInProgress,
		},
		{
			name:       "answers seen before the start stay in progress until the start arrives",
			session:    PracticeSession{}.Answered(at(18, 3), ""),
			readAt:     at(23, 0),
			wantStatus: PracticeSessionInProgress,
		},
		{
			name: "an end seen before the start still ends the session",
			session: PracticeSession{}.Ended(end("e-end", at(18, 11), false, 6)).
				Started(PracticeSessionStart{EventID: "e-start", OccurredAt: at(18, 0), Minutes: 10}),
			readAt:      at(23, 0),
			wantStatus:  PracticeSessionFinished,
			wantEndedAt: ptr(at(18, 11)),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, endedAt := c.session.StatusAt(c.readAt)

			assert.Equal(t, c.wantStatus, status)
			assert.Equal(t, c.wantEndedAt, endedAt)
		})
	}
}

func TestPracticeSession_Started_KeepsThePlan(t *testing.T) {
	s := tenMinuteSessionAt(at(18, 0))

	assert.Equal(t, "s1", s.ID)
	assert.Equal(t, "alice", s.StudentID)
	assert.Equal(t, "guitar", s.InstrumentID)
	assert.Equal(t, 10, s.Minutes)
	assert.Equal(t, ptr(at(18, 0)), s.StartedAt)
	assert.Equal(t, at(18, 0), s.LastEventAt)
	assert.Equal(t, []PlannedPracticeItem{{ItemKey: "play_along:d1", Reason: "due"}}, s.PlannedItems)
}

func TestPracticeSession_Ended_KeepsTheFirstDeliveryOfAnEnd(t *testing.T) {
	first := end("e-end", at(18, 11), false, 6)
	redelivered := first

	s := tenMinuteSessionAt(at(18, 0)).Ended(first).Ended(redelivered)

	assert.Equal(t, &first, s.End)
	assert.Equal(t, at(18, 11), s.LastEventAt)
}

func TestPracticeSession_Ended_AnEarlierEndDoesNotReplaceALaterOne(t *testing.T) {
	later := end("e-end-2", at(18, 35), false, 6)

	s := tenMinuteSessionAt(at(18, 0)).Ended(later).Ended(end("e-end-1", at(18, 4), true, 2))

	assert.Equal(t, &later, s.End)
}

func TestPracticeSession_FeltRatedTemplates(t *testing.T) {
	rated := func(ratings ...FeltRating) PracticeSessionEnd {
		e := end("e-end", at(18, 11), false, 2)
		e.FeltRatings = ratings
		return e
	}
	nameHard := FeltRating{DrillTemplateKey: "fretboard_cell:name_the_note", Felt: FeltHard}
	imageEasy := FeltRating{DrillTemplateKey: "exercise:image_choice", Felt: FeltEasy}

	cases := []struct {
		name    string
		session PracticeSession
		want    []string
	}{
		{
			name: "a rating for a drill the session practised counts",
			session: tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), "fretboard_cell:name_the_note").
				Ended(rated(nameHard)),
			want: []string{"fretboard_cell:name_the_note"},
		},
		{
			name: "a rating for a drill the session didn't practise is ignored",
			session: tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), "fretboard_cell:name_the_note").
				Ended(rated(nameHard, imageEasy)),
			want: []string{"fretboard_cell:name_the_note"},
		},
		{
			name: "an answer arriving after the end still makes its drill's rating count",
			session: tenMinuteSessionAt(at(18, 0)).Ended(rated(imageEasy)).
				Answered(at(18, 9), "exercise:image_choice"),
			want: []string{"exercise:image_choice"},
		},
		{
			name:    "an answer with no timed drill practises none",
			session: tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), "").Ended(rated(nameHard)),
			want:    nil,
		},
		{
			name:    "a session not ended yet has no ratings",
			session: tenMinuteSessionAt(at(18, 0)).Answered(at(18, 3), "fretboard_cell:name_the_note"),
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.session.FeltRatedTemplates())
		})
	}
}

func TestPracticeSession_AnsweredKeepsEachPractisedTemplateOnce(t *testing.T) {
	s := tenMinuteSessionAt(at(18, 0)).
		Answered(at(18, 3), "fretboard_cell:name_the_note").
		Answered(at(18, 4), "exercise:text_response").
		Answered(at(18, 5), "fretboard_cell:name_the_note").
		Answered(at(18, 6), "")

	assert.Equal(t, []string{"exercise:text_response", "fretboard_cell:name_the_note"}, s.PractisedTemplates)
}

func TestTimedDrillTemplate(t *testing.T) {
	cell := PracticeItemKey{Kind: PracticeItemKindFretboardCell}
	exercise, err := ParsePracticeItemKey("exercise:11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	ref := PracticeReference{Exercises: map[string]ExerciseReference{exercise.ExerciseID(): {ExerciseType: "image_choice"}}}

	assert.Equal(t, "fretboard_cell:name_the_note", TimedDrillTemplate(cell, PracticeResponse{Type: PracticeResponseNameTheNote}, ref))
	assert.Equal(t, "fretboard_cell:find_the_note", TimedDrillTemplate(cell, PracticeResponse{Type: PracticeResponseFindTheNote}, ref))
	assert.Equal(t, "exercise:image_choice", TimedDrillTemplate(exercise, PracticeResponse{Type: PracticeResponseOptionChoice}, ref))
	shape := PracticeItemKey{Kind: PracticeItemKindDiagramShape}
	assert.Equal(t, "diagram_shape:name_the_shape", TimedDrillTemplate(shape, PracticeResponse{Type: PracticeResponseNameTheShape}, ref))
	assert.Equal(t, "diagram_shape:find_the_degree", TimedDrillTemplate(shape, PracticeResponse{Type: PracticeResponseFindTheDegree}, ref))
	assert.Empty(t, TimedDrillTemplate(PracticeItemKey{Kind: PracticeItemKindPlayAlong}, PracticeResponse{Type: PracticeResponseSelfRating}, ref))
}
