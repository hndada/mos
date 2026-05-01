package apps

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
)

// statusBarHeight is shared with history.go so card layout starts below the bar.
const statusBarHeight = 24.0

type DefaultStatusBar struct {
	bg      draws.Sprite
	carrier draws.Text
	clock   draws.Text
	system  draws.Text
}

func NewDefaultStatusBar(screenW, screenH float64) *DefaultStatusBar {
	barH := statusBarHeight

	bg := draws.CreateImage(screenW, barH)
	bg.Fill(color.RGBA{0, 0, 0, 180})
	bgSp := draws.NewSprite(bg)
	bgSp.Locate(0, 0, draws.LeftTop)

	clockOpts := draws.NewFaceOptions()
	clockOpts.Size = barH * 0.60
	clock := draws.NewText("")
	clock.SetFace(clockOpts)
	clock.Locate(screenW/2, barH/2, draws.CenterMiddle)

	infoOpts := draws.NewFaceOptions()
	infoOpts.Size = barH * 0.48

	carrier := draws.NewText("MOS")
	carrier.SetFace(infoOpts)
	carrier.Locate(screenW*0.03, barH/2, draws.LeftMiddle)

	system := draws.NewText("5G  Wi-Fi  87%")
	system.SetFace(infoOpts)
	system.Locate(screenW-screenW*0.03, barH/2, draws.RightMiddle)

	return &DefaultStatusBar{
		bg:      bgSp,
		carrier: carrier,
		clock:   clock,
		system:  system,
	}
}

func (sb *DefaultStatusBar) Update() {
	sb.clock.Text = time.Now().Format("15:04")
}

func (sb *DefaultStatusBar) Draw(dst draws.Image) {
	sb.bg.Draw(dst)
	sb.carrier.Draw(dst)
	sb.clock.Draw(dst)
	sb.system.Draw(dst)
}
