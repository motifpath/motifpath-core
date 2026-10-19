package domain

// timedTapsV1 grades guide-tone takes: a chord tone of each chord of a progression
// tapped on the fretboard in time with its playback, four beats to a bar and one
// chord per bar. A bar is hit when a tap within it sounds the asked tone and no tap
// within it sounds another note; the take is right only when every bar is hit. Taps
// after the last bar are not graded.
type timedTapsV1 struct{}

func (timedTapsV1) ID() string { return "timed_taps.v1" }

// beatsPerBarMs is four beats of a minute, in milliseconds: a bar lasts this
// divided by the tempo.
const beatsPerBarMs = 4 * 60_000

func (timedTapsV1) Grade(key PracticeItemKey, r PracticeResponse, ref PracticeReference) GradeResult {
	if key.Kind != PracticeItemKindGuideTone || r.Type != PracticeResponseTimedTaps || r.TempoBPM == nil || *r.TempoBPM < 1 ||
		(r.Tone != "3" && r.Tone != "7") {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	instrument, ok := ref.Instruments[key.LayoutInstrumentID()]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	progression, ok := ref.Progressions[key.ProgressionKey()]
	if !ok {
		return rejected(GradeRejectionUnknownReference)
	}
	musicKey, ok := pitchClass(r.MusicKey)
	if !ok {
		return rejected(GradeRejectionResponseDoesNotFitItem)
	}
	tapped := make([]int, len(r.Taps))
	for i, t := range r.Taps {
		pc, ok := cellPitchClass(instrument, t.String, t.Fret)
		if !ok {
			return rejected(GradeRejectionInvalidCell)
		}
		tapped[i] = pc
	}

	bars := make([]AnswerBar, len(progression.Bars))
	correct, latency := true, 0
	for i, numeral := range progression.Bars {
		bar, delay, ok := gradeBar(r, tapped, musicKey, i, numeral)
		if !ok {
			return rejected(GradeRejectionUnknownReference)
		}
		bars[i] = bar
		latency = max(latency, delay)
		correct = correct && bar.Hit
	}

	return GradeResult{Evidence: GradedEvidence{
		Source:    EvidenceSourceAutoGraded,
		Correct:   &correct,
		LatencyMs: &latency,
		AnswerKey: &AnswerKey{MusicKey: r.MusicKey, Tone: r.Tone, Bars: bars},
	}}
}

// gradeBar grades bar i, whose chord is numeral in the key: hit when a tap within
// it sounds the asked tone and none sounds another note. Its delay is how long
// after the bar's start its first right tap came, or the bar's length when it
// wasn't hit. It reports false for a numeral that isn't one.
func gradeBar(r PracticeResponse, tapped []int, musicKey, i int, numeral string) (AnswerBar, int, bool) {
	c, ok := chordOf(musicKey, numeral)
	if !ok {
		return AnswerBar{}, 0, false
	}
	tone := c.third()
	if r.Tone == "7" {
		tone = seventhOf(musicKey, numeral)
	}
	start, end := i*beatsPerBarMs / *r.TempoBPM, (i+1)*beatsPerBarMs / *r.TempoBPM
	firstHit, wrongNote := -1, false
	for j, t := range r.Taps {
		switch {
		case t.AtMs < start || t.AtMs >= end:
		case tapped[j] != tone:
			wrongNote = true
		case firstHit < 0 || t.AtMs < firstHit:
			firstHit = t.AtMs
		}
	}
	hit := !wrongNote && firstHit >= 0
	delay := end - start
	if hit {
		delay = firstHit - start
	}
	return AnswerBar{Chord: c.symbol(), NoteName: sharpNames[tone], Hit: hit}, delay, true
}

// cellPitchClass is the pitch class a cell of the instrument sounds, and false for a
// string the instrument doesn't have or a tuning entry that isn't a note.
func cellPitchClass(instrument InstrumentReference, str, fret int) (int, bool) {
	strings := len(instrument.Tuning)
	if str < 1 || str > strings || fret < 0 {
		return 0, false
	}
	open, ok := pitchClass(instrument.Tuning[strings-str])
	if !ok {
		return 0, false
	}
	return (open + fret) % 12, true
}
