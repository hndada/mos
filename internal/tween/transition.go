package tween

import (
	"time"

	"github.com/hndada/mos/internal/times"
)

// Transition animates a single float toward a target value over a duration,
// following an Easing curve. Unlike Tween (a sequence with looping), Transition
// is a single shot designed to be re-targeted mid-flight: each call to To
// rebases `from` to the *current* Value(), so motion continues smoothly
// without snapping back to a stale begin point.
//
// Value is wall-clock driven (via internal/times); no per-frame Update call
// is required. The zero value is valid: Value() returns 0, Done() returns true.
type Transition struct {
	from, to  float64
	startTime time.Time
	duration  time.Duration
	easing    Easing
	stopped   bool
}

// To retargets the transition to end at target after dur using ease.
// `from` is rebased to the current Value(), so calling To while the previous
// transition is still in flight produces a continuous (no-jump) motion.
func (t *Transition) To(target float64, dur time.Duration, ease Easing) {
	t.from = t.Value()
	t.to = target
	t.duration = dur
	t.easing = ease
	t.startTime = time.Now()
	t.stopped = false
}

// Snap sets the value immediately to v with no animation. Done() returns true.
func (t *Transition) Snap(v float64) {
	t.from = v
	t.to = v
	t.duration = 0
	t.stopped = true
}

// Done reports whether the transition has reached its target.
func (t *Transition) Done() bool {
	if t.stopped || t.duration <= 0 {
		return true
	}
	return times.Since(t.startTime) >= t.duration
}

// Value returns the current value at the current playback time.
func (t *Transition) Value() float64 {
	if t.stopped || t.duration <= 0 || t.easing == nil {
		return t.to
	}
	elapsed := times.Since(t.startTime)
	if elapsed >= t.duration {
		return t.to
	}
	return t.easing(elapsed, t.from, t.to-t.from, t.duration)
}
