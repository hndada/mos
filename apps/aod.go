package apps

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
)

// DefaultAOD renders an Always-On Display: when the screen is "powered off"
// the simulator paints this instead of a black rectangle, showing a dim
// clock and date. Stays input-inert — wake the device with the Power key.
type DefaultAOD struct {
	bg    draws.Sprite
	clock draws.Text
	date  draws.Text
}

func NewDefaultAOD(screenW, screenH float64) *DefaultAOD {
	bgImg := draws.CreateImage(screenW, screenH)
	bgImg.Fill(color.RGBA{0, 0, 0, 255})
	bg := draws.NewSprite(bgImg)
	bg.Locate(0, 0, draws.LeftTop)

	clockOpts := draws.NewFaceOptions()
	clockOpts.Size = 56
	clock := draws.NewText("")
	clock.SetFace(clockOpts)
	clock.Locate(screenW/2, screenH*0.42, draws.CenterMiddle)
	// Dim grey so the clock looks low-power, not full-brightness.
	clock.ColorScale.ScaleWithColor(color.RGBA{170, 170, 170, 255})

	dateOpts := draws.NewFaceOptions()
	dateOpts.Size = 16
	date := draws.NewText("")
	date.SetFace(dateOpts)
	date.Locate(screenW/2, screenH*0.42+44, draws.CenterMiddle)
	date.ColorScale.ScaleWithColor(color.RGBA{120, 120, 120, 255})

	return &DefaultAOD{bg: bg, clock: clock, date: date}
}

func (a *DefaultAOD) Update() {
	now := time.Now()
	a.clock.Text = now.Format("15:04")
	a.date.Text = now.Format("Mon, Jan 2")
}

func (a *DefaultAOD) Draw(dst draws.Image) {
	a.bg.Draw(dst)
	a.clock.Draw(dst)
	a.date.Draw(dst)
}
