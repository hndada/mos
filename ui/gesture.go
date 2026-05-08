package ui

import (
	"math"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

const (
	DragThresholdPx = 6

	HomeSwipeMinPx = 60
	WakeSwipeMinPx = 80
)

type GestureKind int

const (
	GestureNone GestureKind = iota
	GestureTap
	GestureDrag
	GestureSwipeUp
	GestureSwipeDown
	GestureSwipeLeft
	GestureSwipeRight
)

type GestureEvent struct {
	Kind  GestureKind
	Start draws.XY
	End   draws.XY
	Delta draws.XY
}

// GestureDetector turns a stream of pointer Events into discrete gesture
// events (tap / drag / swipe). It is event-driven: feed it the per-frame
// app.Frame, and it returns the most recent gesture transition for that
// frame (zero value when nothing happened).
//
// Only events whose Down position falls inside Area start tracking; once
// tracking, follow-up Move/Up events are accepted regardless of position
// (so dragging out of the area still produces a gesture).
type GestureDetector struct {
	Area       Box
	MinSwipePx float64

	tracking bool
	dragging bool
	pressed  bool // currently held (after Down inside Area, before Up)
	pointer  int
	start    draws.XY
	current  draws.XY
}

func NewGestureDetector(x, y, w, h float64) GestureDetector {
	var area Box
	area.Size = draws.XY{X: w, Y: h}
	area.Locate(x, y, draws.LeftTop)
	return GestureDetector{
		Area:       area,
		MinSwipePx: HomeSwipeMinPx,
	}
}

// IsHeld reports whether the pointer is currently pressed after starting
// inside Area. Useful for visual "pressed" state on widgets.
func (g *GestureDetector) IsHeld() bool { return g.pressed }

// Update consumes the input events in frame and returns the most recent
// gesture transition. When multiple events fire in one frame, the returned
// event reflects the final state (e.g. Down → Move → Up in one frame
// returns the resulting Tap).
func (g *GestureDetector) Update(frame mosapp.Frame) GestureEvent {
	var result GestureEvent
	for _, ev := range frame.Events {
		switch ev.Kind {
		case input.EventDown:
			if !g.tracking && g.Area.In(ev.Pos) {
				g.tracking = true
				g.dragging = false
				g.pressed = true
				g.pointer = ev.Pointer
				g.start = ev.Pos
				g.current = ev.Pos
			}
		case input.EventMove:
			if g.tracking && ev.Pointer == g.pointer {
				g.current = ev.Pos
				d := ev.Pos.Sub(g.start)
				if math.Hypot(d.X, d.Y) >= DragThresholdPx {
					g.dragging = true
					result = GestureEvent{Kind: GestureDrag, Start: g.start, End: ev.Pos, Delta: d}
				}
			}
		case input.EventUp:
			if g.tracking && ev.Pointer == g.pointer {
				g.tracking = false
				g.pressed = false
				g.current = ev.Pos
				result = g.releaseEvent(ev.Pos)
			}
		}
	}
	return result
}

func (g *GestureDetector) releaseEvent(cursor draws.XY) GestureEvent {
	d := cursor.Sub(g.start)
	e := GestureEvent{Kind: GestureTap, Start: g.start, End: cursor, Delta: d}
	if !g.dragging {
		return e
	}

	ax := math.Abs(d.X)
	ay := math.Abs(d.Y)
	if ax >= ay && ax >= g.MinSwipePx {
		if d.X < 0 {
			e.Kind = GestureSwipeLeft
		} else {
			e.Kind = GestureSwipeRight
		}
		return e
	}
	if ay >= g.MinSwipePx {
		if d.Y < 0 {
			e.Kind = GestureSwipeUp
		} else {
			e.Kind = GestureSwipeDown
		}
		return e
	}
	return e
}

// TriggerButton is a hit-region with an embedded GestureDetector.
// Update returns true on the frame the button is tapped.
type TriggerButton struct {
	Box
	gesture GestureDetector
}

func NewTriggerButton(x, y, w, h float64) TriggerButton {
	var b TriggerButton
	b.Size = draws.XY{X: w, Y: h}
	b.Locate(x, y, draws.LeftTop)
	b.gesture = NewGestureDetector(x, y, w, h)
	return b
}

func (b *TriggerButton) SetRect(x, y, w, h float64) {
	b.Size = draws.XY{X: w, Y: h}
	b.Locate(x, y, draws.LeftTop)
	b.gesture.Area.Size = b.Size
	b.gesture.Area.Locate(x, y, draws.LeftTop)
}

func (b *TriggerButton) Update(frame mosapp.Frame) bool {
	return b.gesture.Update(frame).Kind == GestureTap
}
