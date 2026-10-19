package domain

import "time"

// DrillThresholdSource is where a fluent time comes from.
type DrillThresholdSource string

const (
	// DrillThresholdSourceDefault is a fluent time the team sets per drill
	// template as a starting point.
	DrillThresholdSourceDefault DrillThresholdSource = "default"
	// DrillThresholdSourceBenchmark is twice the team's median net time.
	DrillThresholdSourceBenchmark DrillThresholdSource = "benchmark"
	// DrillThresholdSourceCalibrated comes from felt-rated sessions.
	DrillThresholdSourceCalibrated DrillThresholdSource = "calibrated"
)

// DrillThreshold is one version of a timed drill template's fluent time:
// the time a fluent student spends knowing the answer, net of their tap time
// and of any audio heard once. Versions are reference data, never edited: a
// new one starts at a later EffectiveFrom, and each answer is judged by the
// version in force when it was given.
type DrillThreshold struct {
	ID            string
	TemplateKey   string
	Version       int
	EffectiveFrom time.Time
	FluentNetMs   int
	Source        DrillThresholdSource
}
