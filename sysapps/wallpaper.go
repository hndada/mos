package sysapps

import "github.com/hndada/mos/internal/draws"

type Wallpaper interface {
	Draw(dst draws.Image)
}

type DefaultWallpaper struct {
	Sprite draws.Sprite
}

func (w *DefaultWallpaper) Draw(dst draws.Image) { w.Sprite.Draw(dst) }
