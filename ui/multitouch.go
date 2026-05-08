package ui

import (
	"math"
	"sort"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

// TouchPoint is one currently active pointer in a MultiTouchTracker.
type TouchPoint struct {
	Pointer int
	Pos     draws.XY
}

// PinchEvent describes the current two-finger pinch state.
// Scale is relative to the previous frame; Values above 1 mean the fingers
// moved apart, values below 1 mean they moved together.
type PinchEvent struct {
	Center   draws.XY
	Distance float64
	Scale    float64
	Active   bool
}

// MultiTouchTracker keeps active touch/mouse pointers by ID and exposes
// stable snapshots for widgets that need two-finger gestures.
type MultiTouchTracker struct {
	active       map[int]draws.XY
	prevDistance float64
}

// Update consumes pointer events from frame and returns the current pinch
// state when at least two pointers are active.
func (t *MultiTouchTracker) Update(frame mosapp.Frame) PinchEvent {
	if t.active == nil {
		t.active = make(map[int]draws.XY)
	}
	for _, ev := range frame.Events {
		switch ev.Kind {
		case input.EventDown:
			t.active[ev.Pointer] = ev.Pos
		case input.EventMove:
			if _, ok := t.active[ev.Pointer]; ok {
				t.active[ev.Pointer] = ev.Pos
			}
		case input.EventUp:
			delete(t.active, ev.Pointer)
		}
	}

	points := t.Points()
	if len(points) < 2 {
		t.prevDistance = 0
		return PinchEvent{}
	}

	a, b := points[0].Pos, points[1].Pos
	center := draws.XY{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
	distance := math.Hypot(a.X-b.X, a.Y-b.Y)
	scale := 1.0
	if t.prevDistance > 0 && distance > 0 {
		scale = distance / t.prevDistance
	}
	t.prevDistance = distance
	return PinchEvent{
		Center:   center,
		Distance: distance,
		Scale:    scale,
		Active:   true,
	}
}

// Count returns the number of active pointers.
func (t *MultiTouchTracker) Count() int { return len(t.active) }

// Points returns a stable, pointer-ID sorted snapshot of active pointers.
func (t *MultiTouchTracker) Points() []TouchPoint {
	if len(t.active) == 0 {
		return nil
	}
	points := make([]TouchPoint, 0, len(t.active))
	for id, pos := range t.active {
		points = append(points, TouchPoint{Pointer: id, Pos: pos})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Pointer < points[j].Pointer
	})
	return points
}
