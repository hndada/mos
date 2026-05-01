package windowing

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/tween"
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

// Window owns its off-screen canvas. Every visual property is driven by its
// own Tween so any combination of properties can animate simultaneously.
type Window struct {
	app       *App
	canvas    draws.Image
	lifecycle Lifecycle
	clr       color.RGBA
	iconPos   draws.XY
	iconSize  draws.XY
	screenW   float64
	screenH   float64

	posX  tween.Tween
	posY  tween.Tween
	sizeW tween.Tween
	sizeH tween.Tween
	alpha tween.Tween
}

func newAnim(begin, end float64, d time.Duration) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(begin, end-begin, d, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func staticAnim(value float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(value, 0, time.Nanosecond, tween.EaseOutExponential)
	tw.Start()
	tw.Stop()
	return tw
}

// NewWindow creates a window that animates open from iconPos/iconSize.
func NewWindow(iconPos, iconSize draws.XY, clr color.RGBA, appID string, screenW, screenH float64, ctx *windowContext) *Window {
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(clr)
	if appID == "" {
		appID = AppIDColor
	}
	return &Window{
		app:       NewApp(appID, clr, ctx),
		canvas:    canvas,
		lifecycle: LifecycleShowing,
		clr:       clr,
		iconPos:   iconPos,
		iconSize:  iconSize,
		screenW:   screenW,
		screenH:   screenH,
		posX:      newAnim(iconPos.X, screenW/2, DurationOpening),
		posY:      newAnim(iconPos.Y, screenH/2, DurationOpening),
		sizeW:     newAnim(iconSize.X, screenW, DurationOpening),
		sizeH:     newAnim(iconSize.Y, screenH, DurationOpening),
		alpha:     newAnim(1, 1, DurationOpening),
	}
}

// NewRestoredWindow creates a window that is immediately fully open (no open animation).
// Used to re-display the active app after a display-mode change.
func NewRestoredWindow(state AppState, screenW, screenH float64, ctx *windowContext) *Window {
	if state.ID == "" {
		state.ID = AppIDColor
	}
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(state.Color)
	w := &Window{
		app:       NewApp(state.ID, state.Color, ctx),
		canvas:    canvas,
		lifecycle: LifecycleShown,
		clr:       state.Color,
		iconPos:   draws.XY{X: screenW / 2, Y: screenH / 2},
		iconSize:  draws.XY{X: screenW, Y: screenH},
		screenW:   screenW,
		screenH:   screenH,
		posX:      staticAnim(screenW / 2),
		posY:      staticAnim(screenH / 2),
		sizeW:     staticAnim(screenW),
		sizeH:     staticAnim(screenH),
		alpha:     staticAnim(1),
	}
	// The app is immediately visible, so fire OnResume right away.
	if lc, ok := w.app.content.(mosapp.Lifecycle); ok {
		lc.OnResume()
	}
	return w
}

// Dismiss animates the window closed, shrinking back to the icon it launched from.
func (w *Window) Dismiss() {
	w.DismissTo(w.iconPos, w.iconSize)
}

// DismissTo animates the window closed, shrinking to an arbitrary target.
func (w *Window) DismissTo(targetCenter, targetSize draws.XY) {
	if w.lifecycle != LifecycleShown && w.lifecycle != LifecycleShowing {
		return
	}
	w.lifecycle = LifecycleHiding
	w.posX = newAnim(w.posX.Value(), targetCenter.X, DurationClosing)
	w.posY = newAnim(w.posY.Value(), targetCenter.Y, DurationClosing)
	w.sizeW = newAnim(w.sizeW.Value(), targetSize.X, DurationClosing)
	w.sizeH = newAnim(w.sizeH.Value(), targetSize.Y, DurationClosing)
	w.alpha = newAnim(1, 0, DurationClosing)
	if lc, ok := w.app.content.(mosapp.Lifecycle); ok {
		lc.OnPause()
	}
}

// Destroy fires OnDestroy on the app content and marks the window as destroyed.
func (w *Window) Destroy() {
	if lc, ok := w.app.content.(mosapp.Lifecycle); ok {
		lc.OnDestroy()
	}
	w.lifecycle = LifecycleDestroyed
}

func (w *Window) Update() {
	w.posX.Update()
	w.posY.Update()
	w.sizeW.Update()
	w.sizeH.Update()
	w.alpha.Update()

	switch w.lifecycle {
	case LifecycleShowing:
		if w.sizeW.IsFinished() {
			w.lifecycle = LifecycleShown
			if lc, ok := w.app.content.(mosapp.Lifecycle); ok {
				lc.OnResume()
			}
		}
	case LifecycleShown:
		x, y := input.MouseCursorPosition()
		w.app.Update(draws.XY{X: x, Y: y})
		if w.app.ShouldClose() {
			w.Dismiss()
		}
	case LifecycleHiding:
		if w.sizeW.IsFinished() {
			w.lifecycle = LifecycleHidden
		}
	}
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
	s.Position = draws.XY{X: w.posX.Value(), Y: w.posY.Value()}
	s.Size = draws.XY{X: w.sizeW.Value(), Y: w.sizeH.Value()}
	s.Aligns = draws.CenterMiddle
	s.ColorScale.ScaleAlpha(float32(w.alpha.Value()))
	s.Draw(dst)
}
