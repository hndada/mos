package windowing

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/apps"
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
// placement equals the screen rect. In split mode the canvas stays full-size
// and is clipped into the split pane. PiP/float modes scale the canvas into
// the current rect. Input events are transformed back to canvas coordinates
// before being sent to the app (see ToCanvasFrame and ToSplitCanvasFrame).
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
	secure      bool
	canvasDirty bool // true = updateCanvas must run before next composite
	appResumed  bool
	crashLogged bool
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
	w.appResumed = true
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
	w.pauseApp()
}

// Destroy fires OnDestroy and marks the window as destroyed. cmdCh is left
// open so late writes from app-owned goroutines are silently buffered or
// dropped instead of panicking.
func (w *Window) Destroy() {
	w.pauseApp()
	w.proc.destroy(w.app.content)
	w.proc.drain(w.ctx.ws, w)
	w.lifecycle = LifecycleDestroyed
}

func (w *Window) pauseApp() {
	if !w.appResumed {
		return
	}
	w.proc.pause(w.app.content)
	w.proc.drain(w.ctx.ws, w)
	w.appResumed = false
}

// Update advances animation state and, when an open animation completes,
// fires OnResume on the app. The actual app Update is driven by the
// windowing server via UpdateApp — not here.
func (w *Window) Update() {
	w.reportCrash()
	if w.proc.crashed {
		return
	}
	switch w.lifecycle {
	case LifecycleShowing:
		if w.anim.Done() {
			w.lifecycle = LifecycleShown
			if !w.appResumed {
				w.proc.resume(w.app.content)
				w.proc.drain(w.ctx.ws, w)
				w.appResumed = true
			}
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
	w.reportCrash()
	w.canvasDirty = true
}

func (w *Window) updateCanvas() {
	if w.proc.crashed {
		w.drawCrashCanvas()
		return
	}
	defer func() {
		if r := recover(); r != nil {
			w.proc.crashed = true
			w.proc.crashMessage = "Draw: " + sprintAny(r)
			w.drawCrashCanvas()
		}
	}()
	w.app.Draw(w.canvas)
}

func (w *Window) reportCrash() {
	if !w.proc.crashed || w.crashLogged {
		return
	}
	w.crashLogged = true
	w.ctx.ws.log("app crash " + w.AppID() + ": " + w.proc.crashMessage)
	w.ctx.ws.PostNotice(mosapp.Notice{
		Title: "App stopped",
		Body:  w.AppID() + " crashed",
	})
}

func (w *Window) drawCrashCanvas() {
	w.canvas.Fill(color.RGBA{28, 18, 22, 255})
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 22
	title := draws.NewText("App stopped")
	title.SetFace(titleOpts)
	title.Locate(w.screenW/2, w.screenH*0.42, draws.CenterMiddle)
	title.Draw(w.canvas)

	bodyOpts := draws.NewFaceOptions()
	bodyOpts.Size = 13
	body := draws.NewText(w.AppID() + " crashed")
	body.SetFace(bodyOpts)
	body.ColorScale.Scale(1, 1, 1, 0.68)
	body.Locate(w.screenW/2, w.screenH*0.49, draws.CenterMiddle)
	body.Draw(w.canvas)

	hint := draws.NewText("Press Back or Home")
	hint.SetFace(bodyOpts)
	hint.ColorScale.Scale(1, 1, 1, 0.54)
	hint.Locate(w.screenW/2, w.screenH*0.55, draws.CenterMiddle)
	hint.Draw(w.canvas)
}

func drawRedacted(dst draws.Image, label string) {
	dst.Fill(color.RGBA{8, 8, 10, 255})
	opts := draws.NewFaceOptions()
	opts.Size = 16
	txt := draws.NewText(label)
	txt.SetFace(opts)
	txt.ColorScale.Scale(1, 1, 1, 0.62)
	size := dst.Size()
	txt.Locate(size.X/2, size.Y/2, draws.CenterMiddle)
	txt.Draw(dst)
}

func sprintAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return "panic"
	}
}

// HistoryEntry renders the current app frame into a snapshot for the recents carousel.
func (w *Window) HistoryEntry() apps.HistoryEntry {
	w.updateCanvas()
	size := w.canvas.Size()
	snapshot := draws.CreateImage(size.X, size.Y)
	if w.secure {
		drawRedacted(snapshot, "Secure content")
	} else {
		snapshot.DrawImage(w.canvas.Image, &ebiten.DrawImageOptions{})
	}
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
		KeyEvents: frame.KeyEvents,
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

// ToSplitCanvasFrame translates screen-space coordinates to full-size canvas
// coordinates for split panes. Unlike ToCanvasFrame, it does not scale; split
// panes are clipped windows onto the native-size app surface.
func (w *Window) ToSplitCanvasFrame(frame mosapp.Frame) mosapp.Frame {
	center := w.anim.Pos()
	size := w.anim.Size()
	minX := center.X - size.X/2
	minY := center.Y - size.Y/2

	out := mosapp.Frame{
		Cursor: draws.XY{
			X: frame.Cursor.X - minX,
			Y: frame.Cursor.Y - minY,
		},
		KeyEvents: frame.KeyEvents,
	}
	if len(frame.Events) > 0 {
		out.Events = make([]input.Event, len(frame.Events))
		for i, ev := range frame.Events {
			ev.Pos = draws.XY{
				X: ev.Pos.X - minX,
				Y: ev.Pos.Y - minY,
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
	if w.mode == ModeSplit {
		w.drawSplit(dst)
		return
	}
	s := draws.NewSprite(w.canvas)
	s.Position = w.anim.Pos()
	s.Size = w.anim.Size()
	s.Aligns = draws.CenterMiddle
	s.ColorScale.ScaleAlpha(float32(w.anim.Alpha()))
	s.Draw(dst)
}

func (w *Window) drawSplit(dst draws.Image) {
	center := w.anim.Pos()
	size := w.anim.Size()
	paneW := min(size.X, w.screenW)
	paneH := min(size.Y, w.screenH)
	if paneW <= 0 || paneH <= 0 {
		return
	}

	sub := w.canvas.SubImage(0, 0, int(paneW), int(paneH))
	if sub.IsEmpty() {
		return
	}
	s := draws.NewSprite(sub)
	s.Locate(center.X-size.X/2, center.Y-size.Y/2, draws.LeftTop)
	s.ColorScale.ScaleAlpha(float32(w.anim.Alpha()))
	s.Draw(dst)
}
