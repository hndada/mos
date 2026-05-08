package ui

import (
	"image/color"
	"time"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/tween"
	"github.com/hndada/mos/ui/theme"
)

const (
	ToggleW = 48.0
	ToggleH = 26.0

	toggleAnimDuration = 180 * time.Millisecond
)

// Toggle is an on/off switch. Position is the top-left corner in caller coordinates.
// The track and knob colours are read from the active theme at draw time, so a
// theme switch is reflected on the very next frame without reconstructing the widget.
type Toggle struct {
	gesture  GestureDetector
	Value    bool
	trackOff draws.Sprite // white image; tinted by theme.SurfaceWidget at draw time
	trackOn  draws.Sprite // white image; tinted by theme.AccentSuccess at draw time
	knob     draws.Sprite // white circle; tinted by theme.Knob at draw time
	knobX    tween.Transition
	onAlpha  tween.Transition
	x, y     float64
}

func NewToggle(x, y float64, val bool) Toggle {
	kSize := ToggleH - 4

	// Create white images; colours are applied via ColorScale in Draw so that
	// a runtime theme change is reflected without rebuilding the widget.
	offImg := draws.CreateImage(ToggleW, ToggleH)
	offImg.Fill(color.White)
	offSp := draws.NewSprite(offImg)
	offSp.Locate(x, y, draws.LeftTop)

	onImg := draws.CreateImage(ToggleW, ToggleH)
	onImg.Fill(color.White)
	onSp := draws.NewSprite(onImg)
	onSp.Locate(x, y, draws.LeftTop)

	knobSp := newCircleSprite(kSize, color.White)

	t := Toggle{
		gesture:  NewGestureDetector(x, y, ToggleW, ToggleH),
		Value:    val,
		trackOff: offSp,
		trackOn:  onSp,
		knob:     knobSp,
		x:        x,
		y:        y,
	}
	t.knobX.Snap(t.targetKnobX())
	t.onAlpha.Snap(t.targetOnAlpha())
	t.placeKnob()
	return t
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
func (t *Toggle) Update(frame mosapp.Frame) bool {
	t.placeKnob()
	if t.gesture.Update(frame).Kind == GestureTap {
		t.Value = !t.Value
		t.knobX.To(t.targetKnobX(), toggleAnimDuration, tween.EaseOutExponential)
		t.onAlpha.To(t.targetOnAlpha(), toggleAnimDuration, tween.EaseOutExponential)
		return true
	}
	return false
}

func (t Toggle) Draw(dst draws.Image) {
	t.DrawOffset(dst, 0)
}

// DrawOffset draws the toggle shifted vertically without changing its hitbox.
func (t Toggle) DrawOffset(dst draws.Image, dy float64) {
	// Off track - tinted with the SurfaceWidget theme colour.
	off := t.trackOff
	off.Position.Y += dy
	off.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.SurfaceWidget)))
	off.Draw(dst)

	// On track - tinted with AccentSuccess; faded by onAlpha during animation.
	on := t.trackOn
	on.Position.Y += dy
	on.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.AccentSuccess)))
	on.ColorScale.ScaleAlpha(float32(t.onAlpha.Value()))
	on.Draw(dst)

	// Knob - tinted with the Knob theme colour.
	knob := t.knob
	knob.Position.Y += dy
	knob.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.Knob)))
	knob.Draw(dst)
}
