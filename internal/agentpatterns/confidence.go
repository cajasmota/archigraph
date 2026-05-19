package agentpatterns

import (
	"context"
	"math"
	"time"
)

// Confidence bounds and starting values per ADR-0018.
const (
	InitialConfidence  = 0.4
	ConfidenceFloor    = 0.2
	ConfidenceCeiling  = 1.0
	DecayDeltaPer30Day = 0.05 // −0.05 per 30 days since last_applied
)

// ConfidenceEvent enumerates the lifecycle events that affect pattern confidence.
type ConfidenceEvent int

const (
	// EventApplySuccess is emitted when a pattern was applied and the result
	// was confirmed correct. Confidence += 0.10.
	EventApplySuccess ConfidenceEvent = iota
	// EventApplyFailure is emitted when a pattern was applied and the result
	// was wrong. Confidence −= 0.15.
	EventApplyFailure
	// EventReject is emitted when a user explicitly rejects a pattern as
	// stale or incorrect. Confidence −= 0.30.
	EventReject
	// EventRefine is emitted when a pattern's text is updated without a
	// success/failure signal. Confidence delta == 0.
	EventRefine
)

// NewPatternConfidence returns the initial confidence for a newly created
// pattern: 0.4 (per ADR-0018, Table "Confidence model").
func NewPatternConfidence() float64 {
	return InitialConfidence
}

// ApplyConfidenceDelta applies the event-driven delta to current and returns
// the updated value, clamped to [ConfidenceFloor, ConfidenceCeiling].
//
// Deltas per ADR-0018:
//
//	EventApplySuccess  → +0.10
//	EventApplyFailure  → −0.15
//	EventReject        → −0.30
//	EventRefine        → 0 (neutral)
func ApplyConfidenceDelta(current float64, event ConfidenceEvent) float64 {
	var delta float64
	switch event {
	case EventApplySuccess:
		delta = +0.10
	case EventApplyFailure:
		delta = -0.15
	case EventReject:
		delta = -0.30
	case EventRefine:
		delta = 0
	}
	return clampConfidence(current + delta)
}

// ApplyTimeDecay applies the time-decay rule: −0.05 per 30 days since the
// pattern was last applied (floor: ConfidenceFloor).
//
// daysSinceLastApplied must be ≥ 0. If the pattern has never been applied
// (lastAppliedUnix == 0) the caller may pass 0 to skip decay.
func ApplyTimeDecay(current float64, daysSinceLastApplied float64) float64 {
	if daysSinceLastApplied <= 0 {
		return current
	}
	periods := daysSinceLastApplied / 30.0
	decay := DecayDeltaPer30Day * periods
	return clampConfidence(current - decay)
}

// ApplyTimeDecayFromUnix is a convenience wrapper around ApplyTimeDecay that
// computes the elapsed days between lastAppliedUnix and nowUnix.
// If lastAppliedUnix is 0 (never applied), no decay is applied.
func ApplyTimeDecayFromUnix(current float64, lastAppliedUnix, nowUnix int64) float64 {
	if lastAppliedUnix == 0 {
		return current
	}
	elapsed := nowUnix - lastAppliedUnix
	if elapsed <= 0 {
		return current
	}
	days := float64(elapsed) / 86400.0
	return ApplyTimeDecay(current, days)
}

func clampConfidence(v float64) float64 {
	return math.Max(ConfidenceFloor, math.Min(ConfidenceCeiling, v))
}

// ---------------------------------------------------------------------------
// Background decay scheduler skeleton
// ---------------------------------------------------------------------------
// The full integration with the daemon loop is PR β's territory. This skeleton
// provides the timer and decay-job hook so the wiring can be dropped in.

// DecayJob is a function that the scheduler calls on each tick. It receives
// the current time so implementations can compute elapsed days per pattern.
type DecayJob func(nowUnix int64)

// DecayScheduler drives periodic confidence decay passes.
type DecayScheduler struct {
	interval time.Duration
	job      DecayJob
}

// NewDecayScheduler creates a DecayScheduler that calls job every interval.
// Typical interval: 24 hours. The scheduler does not start until Run is called.
func NewDecayScheduler(interval time.Duration, job DecayJob) *DecayScheduler {
	return &DecayScheduler{interval: interval, job: job}
}

// Run starts the scheduler loop. It blocks until ctx is cancelled, then
// returns. The first tick fires after interval (no immediate run).
func (s *DecayScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			s.job(t.Unix())
		}
	}
}
