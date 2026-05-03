package input

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hndada/mos/internal/draws"
)

// EventKind identifies the category of a pointer Event.
type EventKind int

const (
	EventDown EventKind = iota
	EventMove
	EventUp
)

func (k EventKind) String() string {
	switch k {
	case EventDown:
		return "down"
	case EventMove:
		return "move"
	case EventUp:
		return "up"
	}
	return "unknown"
}

// Event is a single pointer input occurrence in canvas-relative coordinates.
// Pointer is the multi-touch finger ID; for mouse input it is always 0.
//
// Events are produced by Producer.Poll once per frame and routed by the
// windowing server to the active app's goroutine over a channel — see
// internal/windowing.
type Event struct {
	Kind    EventKind
	Pos     draws.XY
	Time    time.Time
	Pointer int
}

// Producer turns each frame's Ebiten input state into a stream of Events.
// It owns the previous-frame state needed to detect button transitions and
// position changes. The zero value is ready to use.
type Producer struct {
	initialized bool
	lastPos     draws.XY
}

// Poll inspects current input state and returns the events that occurred
// since the last call. Order within a frame: Down, Move (if position
// changed), Up.
//
// Currently only the left mouse button is mapped; the Pointer field is
// reserved for multi-touch and is always 0 here.
func (p *Producer) Poll() []Event {
	now := time.Now()
	cx, cy := MouseCursorPosition()
	pos := draws.XY{X: cx, Y: cy}

	var events []Event
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		events = append(events, Event{Kind: EventDown, Pos: pos, Time: now})
	}
	if !p.initialized || pos != p.lastPos {
		events = append(events, Event{Kind: EventMove, Pos: pos, Time: now})
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		events = append(events, Event{Kind: EventUp, Pos: pos, Time: now})
	}

	p.lastPos = pos
	p.initialized = true
	return events
}
