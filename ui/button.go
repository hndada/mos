package ui

import (
	"image/color"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

// Button is a tappable rectangle with a text label.
// All positions are in the coordinate space the caller uses (e.g. content-space).
type Button struct {
	gesture GestureDetector
	bg      draws.Sprite
	label   draws.Text
}

func NewButton(label string, fontSize, x, y, w, h float64, bg color.RGBA) Button {
	img := draws.CreateImage(w, h)
	img.Fill(bg)
	sp := draws.NewSprite(img)
	sp.Locate(x, y, draws.LeftTop)

	t := draws.NewText(label)
	opts := draws.NewFaceOptions()
	opts.Size = fontSize
	t.SetFace(opts)
	t.Locate(x+w/2, y+h/2, draws.CenterMiddle)

	return Button{
		gesture: NewGestureDetector(x, y, w, h),
		bg:      sp,
		label:   t,
	}
}

// Update returns true on the frame the button is released as a tap.
func (b *Button) Update(cursor draws.XY) bool {
	return b.gesture.Update(cursor).Kind == GestureTap
}

func (b *Button) isHeld(cursor draws.XY) bool {
	return input.IsMouseButtonPressed(input.MouseButtonLeft) && b.gesture.Area.In(cursor)
}

func (b *Button) Draw(dst draws.Image, cursor draws.XY) {
	sp := b.bg
	if b.isHeld(cursor) {
		sp.ColorScale.ScaleAlpha(0.55)
	}
	sp.Draw(dst)
	b.label.Draw(dst)
}
