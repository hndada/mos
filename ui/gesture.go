package ui

import (
	"math"

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

type GestureDetector struct {
	Area       Box
	MinSwipePx float64

	tracking bool
	dragging bool
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

func (g *GestureDetector) Update(cursor draws.XY) GestureEvent {
	if input.IsMouseButtonJustPressed(input.MouseButtonLeft) && g.Area.In(cursor) {
		g.tracking = true
		g.dragging = false
		g.start = cursor
		g.current = cursor
	}

	if !g.tracking {
		return GestureEvent{}
	}

	if input.IsMouseButtonPressed(input.MouseButtonLeft) {
		g.current = cursor
		d := cursor.Sub(g.start)
		if math.Hypot(d.X, d.Y) >= DragThresholdPx {
			g.dragging = true
			return GestureEvent{Kind: GestureDrag, Start: g.start, End: cursor, Delta: d}
		}
		return GestureEvent{}
	}

	if input.IsMouseButtonJustReleased(input.MouseButtonLeft) {
		g.tracking = false
		g.current = cursor
		return g.releaseEvent(cursor)
	}

	return GestureEvent{}
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

func (b *TriggerButton) Update(cursor draws.XY) bool {
	return b.gesture.Update(cursor).Kind == GestureTap
}
