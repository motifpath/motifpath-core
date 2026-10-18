package domain

import "time"

// ChordDrill is a chord practised with the instrument in hand: its open voicing
// fretted, strummed once and rated (on bass, its root and fifth). Chord drills
// are reference data, installed only by migrations.
type ChordDrill struct {
	ID string
	// DiagramID is the chord catalog voicing diagram played.
	DiagramID string
	// Chord is the voicing's chord symbol, such as Am.
	Chord   string
	Ordinal int
}

// ChordChangePair is two chord drills practised as a minute of changes, back and
// forth, one change on every beat (on bass, between their roots). One pair is
// one item for both directions, keyed in the order From, To.
type ChordChangePair struct {
	ID               string
	FromChordDrillID string
	ToChordDrillID   string
	Ordinal          int
}

// ChangeTarget is one version of the clean changes a minute that count as fully
// fluent on a chord or root change, the same for every pair and instrument.
// Versions are reference data, never edited: a new one starts at a later
// EffectiveFrom, and each minute is judged by the version in force when it was
// played.
type ChangeTarget struct {
	ID               string
	Version          int
	EffectiveFrom    time.Time
	ChangesPerMinute int
	Source           DrillThresholdSource
}

// ChordDrillCatalog is every installed chord drill and change pair, in catalog
// order, and every change target version, oldest first.
type ChordDrillCatalog struct {
	Drills  []ChordDrill
	Changes []ChordChangePair
	Targets []ChangeTarget
}
