package fw

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/tween"
)

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

// Window owns its canvas. Each visual property is driven by its own Tween so
// any of them can be animated independently at any point in the lifecycle.
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

func NewWindow(iconPos, iconSize draws.XY, clr color.RGBA, screenW, screenH float64) *Window {
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(clr)

	return &Window{
		app:       &App{},
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

func (w *Window) Dismiss() {
	w.DismissTo(w.iconPos, w.iconSize)
}

// DismissTo animates the window shrinking to an arbitrary target center and size.
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
		}
	case LifecycleHiding:
		if w.sizeW.IsFinished() {
			w.lifecycle = LifecycleHidden
		}
	}
}

func (w *Window) Draw(dst draws.Image) {
	if !w.lifecycle.Visible() {
		return
	}
	s := draws.NewSprite(w.canvas)
	s.Position = draws.XY{X: w.posX.Value(), Y: w.posY.Value()}
	s.Size = draws.XY{X: w.sizeW.Value(), Y: w.sizeH.Value()}
	s.Aligns = draws.CenterMiddle
	s.ColorScale.ScaleAlpha(float32(w.alpha.Value()))
	s.Draw(dst)
}
