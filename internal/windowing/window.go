package windowing

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
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
// alpha) are driven by a single WindowAnim, whose retargetable transitions
// allow Dismiss to interrupt an in-flight open without snapping.
//
// The app's content is hosted on a per-window goroutine via windowProc;
// this Window struct only exposes main-thread operations (animation,
// composition, lifecycle dispatch). See window_proc.go for the boundary
// protocol and its real-OS analogue.
type Window struct {
	app       *App
	ctx       *windowContext
	proc      *windowProc
	canvas    draws.Image
	lifecycle Lifecycle
	clr       color.RGBA
	iconPos   draws.XY
	iconSize  draws.XY
	screenW   float64
	screenH   float64
	anim      WindowAnim
}

// NewWindow creates a window, starts its goroutine, and waits for OnCreate
// to ack before returning. The window animates open from iconPos / iconSize.
func NewWindow(iconPos, iconSize draws.XY, clr color.RGBA, appID string, screenW, screenH float64, ws *WindowingServer) *Window {
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(clr)
	if appID == "" {
		appID = AppIDColor
	}
	proc := newWindowProc()
	ctx := newWindowContext(ws, proc)
	app := NewApp(appID, clr, ctx)
	w := &Window{
		app:       app,
		ctx:       ctx,
		proc:      proc,
		canvas:    canvas,
		lifecycle: LifecycleShowing,
		clr:       clr,
		iconPos:   iconPos,
		iconSize:  iconSize,
		screenW:   screenW,
		screenH:   screenH,
	}
	w.anim.OpenFrom(iconPos, iconSize, draws.XY{X: screenW, Y: screenH}, DurationOpening)

	// Start the goroutine; OnCreate runs there and acks before we return.
	go proc.run(app.content, ctx)
	<-proc.ackCh
	return w
}

// NewRestoredWindow creates a window that is immediately fully open (no open animation).
// Used to re-display the active app after a display-mode change.
func NewRestoredWindow(state AppState, screenW, screenH float64, ws *WindowingServer) *Window {
	if state.ID == "" {
		state.ID = AppIDColor
	}
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(state.Color)
	proc := newWindowProc()
	ctx := newWindowContext(ws, proc)
	app := NewApp(state.ID, state.Color, ctx)
	w := &Window{
		app:       app,
		ctx:       ctx,
		proc:      proc,
		canvas:    canvas,
		lifecycle: LifecycleShown,
		clr:       state.Color,
		iconPos:   draws.XY{X: screenW / 2, Y: screenH / 2},
		iconSize:  draws.XY{X: screenW, Y: screenH},
		screenW:   screenW,
		screenH:   screenH,
	}
	w.anim.SnapOpen(draws.XY{X: screenW, Y: screenH})

	// Start goroutine; OnCreate acks. The app is already fully visible, so
	// fire OnResume right after.
	go proc.run(app.content, ctx)
	<-proc.ackCh
	proc.sendTick(tickMsg{kind: tickResume})
	return w
}

// Dismiss animates the window closed, shrinking back to the icon it launched from.
func (w *Window) Dismiss() {
	w.DismissTo(w.iconPos, w.iconSize)
}

// DismissTo animates the window closed, shrinking to an arbitrary target.
// Safe to call mid-open: the underlying Transition rebases from the current
// value, so the reversal is continuous. Fires OnPause on the goroutine.
func (w *Window) DismissTo(targetCenter, targetSize draws.XY) {
	if w.lifecycle != LifecycleShown && w.lifecycle != LifecycleShowing {
		return
	}
	w.lifecycle = LifecycleHiding
	w.anim.CloseTo(targetCenter, targetSize, DurationClosing)
	w.proc.sendTick(tickMsg{kind: tickPause})
}

// Destroy fires OnDestroy on the goroutine, joins it, and marks the window
// as destroyed. After this the windowProc is finished; cmdCh is left open
// (any late writes from app-spawned goroutines are silently buffered or
// dropped).
func (w *Window) Destroy() {
	w.proc.sendTick(tickMsg{kind: tickDestroy})
	close(w.proc.tickCh)
	w.lifecycle = LifecycleDestroyed
}

// Update advances animation state and, when an open animation completes,
// fires OnResume on the goroutine. The actual app Update is driven by the
// windowing server via UpdateApp — not here.
func (w *Window) Update() {
	switch w.lifecycle {
	case LifecycleShowing:
		if w.anim.Done() {
			w.lifecycle = LifecycleShown
			w.proc.sendTick(tickMsg{kind: tickResume})
		}
	case LifecycleHiding:
		if w.anim.Done() {
			w.lifecycle = LifecycleHidden
		}
	}
}

// UpdateApp routes the input frame to the goroutine and waits for the tick
// to ack. Call only when lifecycle == LifecycleShown. After the ack the
// server reads w.proc.shouldClose / drainLaunch() to handle command results.
func (w *Window) UpdateApp(frame mosapp.Frame) {
	w.proc.sendTick(tickMsg{kind: tickUpdate, frame: frame})
	w.proc.drain(w.ctx.ws)
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

func (w *Window) Draw(dst draws.Image) {
	if !w.lifecycle.Visible() {
		return
	}
	w.updateCanvas()
	s := draws.NewSprite(w.canvas)
	s.Position = w.anim.Pos()
	s.Size = w.anim.Size()
	s.Aligns = draws.CenterMiddle
	s.ColorScale.ScaleAlpha(float32(w.anim.Alpha()))
	s.Draw(dst)
}
