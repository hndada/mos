package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hndada/mos/internal/draws"
)

func newCircleSprite(diameter float64, clr color.Color) draws.Sprite {
	img := draws.CreateImage(diameter, diameter)
	r := float32(diameter / 2)
	vector.DrawFilledCircle(img.Image, r, r, r, clr, true)
	return draws.NewSprite(img)
}
