package ui

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/tween"
)

const (
	ToggleW = 48.0
	ToggleH = 26.0

	toggleAnimDuration = 180 * time.Millisecond
)

// Toggle is an on/off switch. Position is the top-left corner in caller coordinates.
type Toggle struct {
	gesture  GestureDetector
	Value    bool
	trackOff draws.Sprite
	trackOn  draws.Sprite
	knob     draws.Sprite
	knobX    tween.Tween
	onAlpha  tween.Tween
	x, y     float64
}

func NewToggle(x, y float64, val bool) Toggle {
	kSize := ToggleH - 4

	offImg := draws.CreateImage(ToggleW, ToggleH)
	offImg.Fill(color.RGBA{72, 72, 74, 255})
	offSp := draws.NewSprite(offImg)
	offSp.Locate(x, y, draws.LeftTop)

	onImg := draws.CreateImage(ToggleW, ToggleH)
	onImg.Fill(color.RGBA{52, 199, 89, 255})
	onSp := draws.NewSprite(onImg)
	onSp.Locate(x, y, draws.LeftTop)

	knobSp := newCircleSprite(kSize, color.RGBA{255, 255, 255, 255})

	t := Toggle{
		gesture:  NewGestureDetector(x, y, ToggleW, ToggleH),
		Value:    val,
		trackOff: offSp,
		trackOn:  onSp,
		knob:     knobSp,
		x:        x,
		y:        y,
	}
	t.knobX = newToggleAnim(t.targetKnobX())
	t.onAlpha = newToggleAnim(t.targetOnAlpha())
	t.placeKnob()
	return t
}

func newToggleAnim(value float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(value, 0, time.Nanosecond, tween.EaseOutExponential)
	tw.Start()
	tw.Stop()
	return tw
}

func startToggleAnim(from, to float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(from, to-from, toggleAnimDuration, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func (t *Toggle) targetKnobX() float64 {
	kSize := ToggleH - 4
	if t.Value {
		return t.x + ToggleW - kSize - 2
	}
	return t.x + 2
}

func (t *Toggle) targetOnAlpha() float64 {
	if t.Value {
		return 1
	}
	return 0
}

func (t *Toggle) placeKnob() {
	t.knob.Position = draws.XY{X: t.knobX.Value(), Y: t.y + 2}
}

// Update flips Value on tap and returns true when the value changed.
func (t *Toggle) Update(cursor draws.XY) bool {
	t.knobX.Update()
	t.onAlpha.Update()
	t.placeKnob()
	if t.gesture.Update(cursor).Kind == GestureTap {
		t.Value = !t.Value
		t.knobX = startToggleAnim(t.knobX.Value(), t.targetKnobX())
		t.onAlpha = startToggleAnim(t.onAlpha.Value(), t.targetOnAlpha())
		return true
	}
	return false
}

func (t Toggle) Draw(dst draws.Image) {
	t.trackOff.Draw(dst)
	on := t.trackOn
	on.ColorScale.ScaleAlpha(float32(t.onAlpha.Value()))
	on.Draw(dst)
	t.knob.Draw(dst)
}
