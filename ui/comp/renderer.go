package comp

import (
	"github.com/hndada/mos/internal/draws"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/input"
)

// ── Retained state ────────────────────────────────────────────────────────────

// uiState persists across frames. Only the Renderer writes to it.
type uiState struct {
	// pressed is the path of the node that received the most recent EventDown.
	// Cleared when the finger lifts (EventUp / EventCancel).
	pressed string

	// downPath records which node got EventDown, used to decide if
	// EventUp on the same node counts as a click.
	downPath string

	// focused is the path of the widget that holds keyboard focus (TextField).
	focused string
}

// ia returns the Interaction for a node at the given path.
func (s *uiState) ia(path string) IA {
	return IA{
		Pressed: s.pressed == path,
		Focused: s.focused == path,
	}
}

// ── Renderer ──────────────────────────────────────────────────────────────────

// Renderer runs the declarative Build → Layout → Input → Draw pipeline each
// frame. It is the only stateful object; all Nodes and Widgets are rebuilt
// fresh each frame.
//
// Usage:
//
//	r := comp.NewRenderer(screenW, screenH, app.Build)
//
//	// inside mosapp.App.Update:
//	r.Update(frame)
//
//	// inside mosapp.App.Draw:
//	r.Draw(dst)
type Renderer struct {
	screenW, screenH float64
	build            func() Node
	state            uiState
	last             *placed // placed tree from last Update; drawn by Draw
}

// NewRenderer creates a Renderer for a screen of the given size.
// build is called once per frame to obtain the current UI description.
func NewRenderer(screenW, screenH float64, build func() Node) *Renderer {
	return &Renderer{
		screenW: screenW,
		screenH: screenH,
		build:   build,
	}
}

// Update rebuilds the UI tree, lays it out, and routes input events.
// Call this from mosapp.App.Update.
func (r *Renderer) Update(frame mosapp.Frame) {
	// 1. Build: obtain a fresh immutable Node tree.
	root := r.build()

	// 2. Layout: assign every node a concrete screen Rect.
	screen := Rect{0, 0, r.screenW, r.screenH}
	r.last = root.w.place(screen, "root")

	// 3. Input: route events from the frame to the placed tree.
	r.handleInput(frame)
}

// Draw renders the last laid-out tree onto dst.
// Call this from mosapp.App.Draw.
func (r *Renderer) Draw(dst draws.Image) {
	if r.last == nil {
		return
	}
	iaFn := func(path string) IA { return r.state.ia(path) }
	r.last.drawAll(dst, iaFn)
}

// ── Input routing ─────────────────────────────────────────────────────────────

func (r *Renderer) handleInput(frame mosapp.Frame) {
	for _, ev := range frame.Events {
		pt := draws.XY{X: float64(ev.Pos.X), Y: float64(ev.Pos.Y)}
		switch ev.Kind {
		case input.EventDown:
			hit := r.hitAt(pt)
			if hit != nil {
				r.state.pressed = hit.path
				r.state.downPath = hit.path
				// Clicking a non-focus widget clears keyboard focus.
				if !hit.isFocus {
					r.state.focused = ""
				}
			} else {
				r.state.pressed = ""
				r.state.downPath = ""
				r.state.focused = ""
			}

		case input.EventUp:
			if r.state.pressed != "" {
				hit := r.hitAt(pt)
				if hit != nil && hit.path == r.state.downPath {
					// Tap confirmed: fire the click handler.
					if hit.onClick != nil {
						hit.onClick()
					}
					// If the hit widget accepts focus, give it focus.
					if hit.isFocus {
						r.state.focused = hit.path
					}
				}
			}
			r.state.pressed = ""

		case input.EventMove:
			// Update pressed tracking: if finger drifts outside, cancel.
			if r.state.pressed != "" {
				hit := r.hitAt(pt)
				if hit == nil || hit.path != r.state.downPath {
					r.state.pressed = ""
				}
			}
		}
	}
}

func (r *Renderer) hitAt(pt draws.XY) *placed {
	if r.last == nil {
		return nil
	}
	return r.last.hitTest(pt)
}
