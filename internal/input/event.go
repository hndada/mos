package input

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hndada/mos/internal/draws"
)

// EventKind identifies the category of an input Event.
type EventKind int

const (
	EventDown  EventKind = iota // pointer pressed
	EventMove                   // pointer moved (only emitted when position changed)
	EventUp                     // pointer released
	EventWheel                  // scroll wheel rotated
)

func (k EventKind) String() string {
	switch k {
	case EventDown:
		return "down"
	case EventMove:
		return "move"
	case EventUp:
		return "up"
	case EventWheel:
		return "wheel"
	}
	return "unknown"
}

// MousePointer is the synthetic Pointer ID assigned to mouse events.
// Touch events use the underlying ebiten.TouchID (always > 0 in practice),
// so 0 unambiguously means "the mouse."
const MousePointer = 0

// Event is a single input occurrence in canvas-relative coordinates.
// Pointer is the multi-touch finger ID; for mouse input it is MousePointer.
// Wheel is populated only for EventWheel.
//
// Events are produced by Producer.Poll once per frame and routed by the
// windowing server to the active app's goroutine over a channel — see
// internal/windowing.
type Event struct {
	Kind    EventKind
	Pos     draws.XY
	Wheel   draws.XY
	Time    time.Time
	Pointer int
}

// Producer turns each frame's Ebiten input state into a stream of Events.
// It owns the previous-frame state needed to detect transitions: mouse
// button up/down and per-touch press/release/move.
//
// The zero value is ready to use. Producers are independent — multiple
// instances can poll the same Ebiten frame state without interference,
// each tracking their own previous positions in their own coordinate
// system. Useful when one consumer (e.g. cmd/sim) needs raw window
// coordinates and another (the windowing server) needs canvas-relative.
type Producer struct {
	initialized bool
	lastPos     draws.XY

	// Per-touch tracking. We keep the previous-frame positions so we can
	// emit a Move event only when a touch actually moved, matching the
	// mouse-side behaviour. touchScratch is reused each frame to avoid
	// allocating a temporary slice for ebiten's AppendTouchIDs API.
	touchPos     map[ebiten.TouchID]draws.XY
	touchScratch []ebiten.TouchID
}

// Poll inspects current input state and returns the events that occurred
// since the last call. Order within a frame: Wheel, mouse Down/Move/Up,
// then per-touch Down/Move/Up.
func (p *Producer) Poll() []Event {
	now := time.Now()
	cx, cy := MouseCursorPosition()
	pos := draws.XY{X: cx, Y: cy}

	var events []Event

	// Wheel.
	if wx, wy := MouseWheelPosition(); wx != 0 || wy != 0 {
		events = append(events, Event{
			Kind:    EventWheel,
			Pos:     pos,
			Wheel:   draws.XY{X: wx, Y: wy},
			Time:    now,
			Pointer: MousePointer,
		})
	}

	// Mouse pointer.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		events = append(events, Event{Kind: EventDown, Pos: pos, Time: now, Pointer: MousePointer})
	}
	if !p.initialized || pos != p.lastPos {
		events = append(events, Event{Kind: EventMove, Pos: pos, Time: now, Pointer: MousePointer})
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		events = append(events, Event{Kind: EventUp, Pos: pos, Time: now, Pointer: MousePointer})
	}

	p.lastPos = pos
	p.initialized = true

	// Touch pointers. Each touch is its own Pointer (using the ebiten ID).
	// A touch always begins with a JustPressed → emit Down, then continues
	// with a Move whenever it shifts, and ends with JustReleased → Up.
	if p.touchPos == nil {
		p.touchPos = make(map[ebiten.TouchID]draws.XY)
	}

	for _, id := range inpututil.AppendJustPressedTouchIDs(p.touchScratch[:0]) {
		tx, ty := ebiten.TouchPosition(id)
		tp := p.canvasPos(tx, ty)
		p.touchPos[id] = tp
		events = append(events, Event{Kind: EventDown, Pos: tp, Time: now, Pointer: int(id)})
	}

	for _, id := range ebiten.AppendTouchIDs(p.touchScratch[:0]) {
		tx, ty := ebiten.TouchPosition(id)
		tp := p.canvasPos(tx, ty)
		if prev, ok := p.touchPos[id]; ok && prev == tp {
			continue
		}
		// Either first sighting (no prior Down — shouldn't happen, but be
		// defensive) or position changed.
		p.touchPos[id] = tp
		events = append(events, Event{Kind: EventMove, Pos: tp, Time: now, Pointer: int(id)})
	}

	for _, id := range inpututil.AppendJustReleasedTouchIDs(p.touchScratch[:0]) {
		tp := p.touchPos[id] // last known position; ebiten clears TouchPosition on release
		delete(p.touchPos, id)
		events = append(events, Event{Kind: EventUp, Pos: tp, Time: now, Pointer: int(id)})
	}

	return events
}

// canvasPos applies the same cursor offset MouseCursorPosition does, so
// touch events land in the same coordinate space as mouse events.
func (p *Producer) canvasPos(x, y int) draws.XY {
	return draws.XY{X: float64(x) - cursorOffsetX, Y: float64(y) - cursorOffsetY}
}
