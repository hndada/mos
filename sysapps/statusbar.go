package sysapps

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
)

type StatusBar interface {
	Update()
	Draw(dst draws.Image)
}

type DefaultStatusBar struct {
	bg    draws.Sprite
	clock draws.Text
}

func NewDefaultStatusBar(screenW, screenH float64) *DefaultStatusBar {
	barH := screenH * 0.06

	bg := draws.CreateImage(screenW, barH)
	bg.Fill(color.RGBA{0, 0, 0, 160})
	bgSp := draws.NewSprite(bg)
	bgSp.Locate(0, 0, draws.LeftTop)

	t := draws.NewText("")
	t.Locate(screenW-screenW*0.04, barH/2, draws.RightMiddle)

	return &DefaultStatusBar{bg: bgSp, clock: t}
}

func (sb *DefaultStatusBar) Update() {
	sb.clock.Text = time.Now().Format("15:04")
}

func (sb *DefaultStatusBar) Draw(dst draws.Image) {
	sb.bg.Draw(dst)
	sb.clock.Draw(dst)
}
