package windowing

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

// Lifecycle tracks the animation / activation phase of a Window.
type Lifecycle int

const (
	LifecycleInitializing Lifecycle = iota
	LifecycleShowing
	LifecycleShown
	LifecycleHiding
	LifecycleHidden
	LifecycleDestroying
	LifecycleDestroyed
)

func (l Lifecycle) Visible() bool {
	return l == LifecycleShowing || l == LifecycleShown || l == LifecycleHiding
}

func (l Lifecycle) String() string {
	switch l {
	case LifecycleInitializing:
		return "initializing"
	case LifecycleShowing:
		return "showing"
	case LifecycleShown:
		return "shown"
	case LifecycleHiding:
		return "hiding"
	case LifecycleHidden:
		return "hidden"
	case LifecycleDestroying:
		return "destroying"
	case LifecycleDestroyed:
		return "destroyed"
	default:
		return "unknown"
	}
}

// Window owns its off-screen canvas. All visual properties (position, size,
// alpha) are driven by a single anim, whose retargetable transitions
// allow Dismiss to interrupt an in-flight open without snapping.
//
// Multi-window layout: in fullscreen mode the canvas fills the screen and
// placement equals the screen rect. In split/pip/float modes the canvas is
// still the full screen size but the anim targets a smaller rect;
// the compositor scales the canvas to fit that rect, and input events are
// reverse-transformed from display-rect coords back to canvas-rect coords
// before being sent to the app (see ToCanvasFrame).
//
// The app's content is updated and drawn on the main goroutine. Context
// methods enqueue commands through windowProc so app code can still request
// OS actions without mutating server state directly.
//
// Canvas caching: canvasDirty is true when the canvas must be repainted via
// app.Draw before the next composite. It is set true on construction (first
// paint) and after every UpdateApp tick (app state may have changed). It is
// cleared after updateCanvas runs. Draw reuses the last canvas without calling
// updateCanvas while the flag is false.
type Window struct {
	app         *App
	ctx         *windowContext
	proc        *windowProc
	canvas      draws.Image
	lifecycle   Lifecycle
	clr         color.RGBA
	iconPos     draws.XY
	iconSize    draws.XY
	screenW     float64
	screenH     float64
	anim        anim
	mode        Mode
	placement   Placement
	captured    map[int]bool
	canvasDirty bool // true = updateCanvas must run before next composite
}

// NewWindow creates a window and runs OnCreate before returning. The window
// animates open from iconPos / iconSize.
func NewWindow(iconPos, iconSize draws.XY, clr color.RGBA, appID string, screenW, screenH float64, ws *Server) *Window {
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(clr)
	if appID == "" {
		appID = AppIDColor
	}
	proc := newWindowProc()
	ctx := newWindowContext(ws, proc)
	app := NewApp(appID, clr, ctx)
	ctx.appID = app.ID
	w := &Window{
		app:         app,
		ctx:         ctx,
		proc:        proc,
		canvas:      canvas,
		lifecycle:   LifecycleShowing,
		clr:         clr,
		iconPos:     iconPos,
		iconSize:    iconSize,
		screenW:     screenW,
		screenH:     screenH,
		mode:        ModeFullscreen,
		placement:   fullscreenPlacement(screenW, screenH),
		captured:    make(map[int]bool),
		canvasDirty: true, // paint the initial frame immediately
	}
	w.anim.OpenFrom(iconPos, iconSize, draws.XY{X: screenW, Y: screenH}, DurationOpening)

	proc.create(app.content, ctx)
	proc.drain(ws, w)
	return w
}

// NewRestoredWindow creates a window that is immediately fully open (no open animation).
// Used to re-display the active app after a display-mode change.
func NewRestoredWindow(state AppState, screenW, screenH float64, ws *Server) *Window {
	if state.ID == "" {
		state.ID = AppIDColor
	}
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(state.Color)
	proc := newWindowProc()
	ctx := newWindowContext(ws, proc)
	app := NewApp(state.ID, state.Color, ctx)
	ctx.appID = app.ID
	w := &Window{
		app:         app,
		ctx:         ctx,
		proc:        proc,
		canvas:      canvas,
		lifecycle:   LifecycleShown,
		clr:         state.Color,
		iconPos:     draws.XY{X: screenW / 2, Y: screenH / 2},
		iconSize:    draws.XY{X: screenW, Y: screenH},
		screenW:     screenW,
		screenH:     screenH,
		mode:        ModeFullscreen,
		placement:   fullscreenPlacement(screenW, screenH),
		captured:    make(map[int]bool),
		canvasDirty: true, // paint the initial frame immediately
	}
	w.anim.SnapOpen(draws.XY{X: screenW, Y: screenH})

	proc.create(app.content, ctx)
	proc.resume(app.content)
	proc.drain(ws, w)
	return w
}

// Dismiss animates the window closed, shrinking back to the icon it launched from.
func (w *Window) Dismiss() {
	w.DismissTo(w.iconPos, w.iconSize)
}

// DismissTo animates the window closed, shrinking to an arbitrary target.
// Safe to call mid-open: the underlying Transition rebases from the current
// value, so the reversal is continuous. Fires OnPause on the app.
func (w *Window) DismissTo(targetCenter, targetSize draws.XY) {
	if w.lifecycle != LifecycleShown && w.lifecycle != LifecycleShowing {
		return
	}
	w.lifecycle = LifecycleHiding
	w.anim.CloseTo(targetCenter, targetSize, DurationClosing)
	w.proc.pause(w.app.content)
	w.proc.drain(w.ctx.ws, w)
}

// Destroy fires OnDestroy and marks the window as destroyed. cmdCh is left
// open so late writes from app-owned goroutines are silently buffered or
// dropped instead of panicking.
func (w *Window) Destroy() {
	w.proc.destroy(w.app.content)
	w.proc.drain(w.ctx.ws, w)
	w.lifecycle = LifecycleDestroyed
}

// Update advances animation state and, when an open animation completes,
// fires OnResume on the app. The actual app Update is driven by the
// windowing server via UpdateApp — not here.
func (w *Window) Update() {
	switch w.lifecycle {
	case LifecycleShowing:
		if w.anim.Done() {
			w.lifecycle = LifecycleShown
			w.proc.resume(w.app.content)
			w.proc.drain(w.ctx.ws, w)
			// OnResume may change visual state; mark canvas dirty so the
			// first fully-open frame reflects it.
			w.canvasDirty = true
		}
	case LifecycleHiding:
		if w.anim.Done() {
			w.lifecycle = LifecycleHidden
		}
	}
}

// UpdateApp routes the input frame to the app. Call only when lifecycle ==
// LifecycleShown. After Update, the server drains queued Context commands.
// It also marks the canvas dirty: the app's Update may have changed state
// that Draw must reflect.
func (w *Window) UpdateApp(frame mosapp.Frame) {
	w.proc.update(w.app.content, frame)
	w.proc.drain(w.ctx.ws, w)
	w.canvasDirty = true
}

func (w *Window) updateCanvas() {
	w.app.Draw(w.canvas)
}

// HistoryEntry renders the current app frame into a snapshot for the recents carousel.
func (w *Window) HistoryEntry() apps.HistoryEntry {
	w.updateCanvas()
	size := w.canvas.Size()
	snapshot := draws.CreateImage(size.X, size.Y)
	snapshot.DrawImage(w.canvas.Image, &ebiten.DrawImageOptions{})
	return apps.HistoryEntry{
		AppID:    w.app.ID,
		Color:    w.clr,
		Snapshot: snapshot,
	}
}

func (w *Window) AppState() AppState { return AppState{ID: w.app.ID, Color: w.clr} }
func (w *Window) AppID() string      { return w.app.ID }
func (w *Window) Mode() Mode         { return w.mode }

// SetPlacement retargets the window animation to a new display rect and
// records the final placement. The mode field must be set by the caller.
func (w *Window) SetPlacement(p Placement, dur time.Duration) {
	w.placement = p
	w.anim.Retarget(p.Center, p.Size, dur)
}

// ToCanvasFrame translates screen-space coordinates in frame to canvas-space.
// The canvas is always screenW×screenH; the window display rect is whatever
// the animation currently reports. Events outside the display rect are still
// included but with out-of-bounds canvas coordinates (the app handles them
// gracefully because hit-tests will fail on its own widgets).
func (w *Window) ToCanvasFrame(frame mosapp.Frame, screenW, screenH float64) mosapp.Frame {
	center := w.anim.Pos()
	size := w.anim.Size()
	if size.X == 0 || size.Y == 0 {
		return frame
	}
	minX := center.X - size.X/2
	minY := center.Y - size.Y/2
	sx := screenW / size.X
	sy := screenH / size.Y

	out := mosapp.Frame{
		Cursor: draws.XY{
			X: (frame.Cursor.X - minX) * sx,
			Y: (frame.Cursor.Y - minY) * sy,
		},
	}
	if len(frame.Events) > 0 {
		out.Events = make([]input.Event, len(frame.Events))
		for i, ev := range frame.Events {
			ev.Pos = draws.XY{
				X: (ev.Pos.X - minX) * sx,
				Y: (ev.Pos.Y - minY) * sy,
			}
			out.Events[i] = ev
		}
	}
	return out
}

// ContainsScreenPos reports whether a screen-space position falls inside the
// window's current animated display rect.
func (w *Window) ContainsScreenPos(pos draws.XY) bool {
	c := w.anim.Pos()
	sz := w.anim.Size()
	minX := c.X - sz.X/2
	minY := c.Y - sz.Y/2
	return pos.X >= minX && pos.X < minX+sz.X &&
		pos.Y >= minY && pos.Y < minY+sz.Y
}

func (w *Window) Draw(dst draws.Image) {
	if !w.lifecycle.Visible() {
		return
	}
	// Canvas caching: only invoke app.Draw when state has changed since the
	// last paint. The canvas retains its last content across frames where no
	// tick ran (opening/closing animations, suppressed ticks). The compositor
	// step below always runs because position and alpha may still be animating
	// even when the content is unchanged.
	if w.canvasDirty {
		w.updateCanvas()
		w.canvasDirty = false
	}
	s := draws.NewSprite(w.canvas)
	s.Position = w.anim.Pos()
	s.Size = w.anim.Size()
	s.Aligns = draws.CenterMiddle
	s.ColorScale.ScaleAlpha(float32(w.anim.Alpha()))
	s.Draw(dst)
}
