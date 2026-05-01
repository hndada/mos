package sysapps

import (
	"image/color"

	"github.com/hndada/mos/internal/draws"
)

type Wallpaper interface {
	Draw(dst draws.Image)
}

type DefaultWallpaper struct {
	Sprite draws.Sprite
}

func NewDefaultWallpaper(screenW, screenH float64) *DefaultWallpaper {
	img := draws.CreateImage(screenW, screenH)
	img.Fill(color.RGBA{18, 32, 68, 255}) // deep navy
	sp := draws.NewSprite(img)
	sp.Locate(0, 0, draws.LeftTop)
	return &DefaultWallpaper{Sprite: sp}
}

func (w *DefaultWallpaper) Draw(dst draws.Image) { w.Sprite.Draw(dst) }
