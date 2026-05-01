package fw

import (
	"image/color"

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

	posX  tween.Tween
	posY  tween.Tween
	sizeW tween.Tween
	sizeH tween.Tween
	alpha tween.Tween
}

func newAnim(begin, end float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(begin, end-begin, DurationSplash, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func NewWindow(iconPos, iconSize draws.XY, screenW, screenH float64) *Window {
	canvas := draws.CreateImage(screenW, screenH)
	canvas.Fill(color.RGBA{30, 30, 30, 255})

	return &Window{
		app:       &App{},
		canvas:    canvas,
		lifecycle: LifecycleShowing,
		posX:      newAnim(iconPos.X, screenW/2),
		posY:      newAnim(iconPos.Y, screenH/2),
		sizeW:     newAnim(iconSize.X, screenW),
		sizeH:     newAnim(iconSize.Y, screenH),
		alpha:     newAnim(1, 1),
	}
}

func (w *Window) Update() {
	w.posX.Update()
	w.posY.Update()
	w.sizeW.Update()
	w.sizeH.Update()
	w.alpha.Update()
	if w.lifecycle == LifecycleShowing && w.sizeW.IsFinished() {
		w.lifecycle = LifecycleShown
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
