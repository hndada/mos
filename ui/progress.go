package ui

import (
	"image/color"

	"github.com/hndada/mos/internal/draws"
)

// ProgressBar is a horizontal fill indicator. Value is clamped to [0, 1].
// It is display-only; drive Value directly from app state.
//
//	bar.Value = float64(downloaded) / float64(total)
//	bar.Draw(dst)
type ProgressBar struct {
	Value float64 // 0.0 = empty, 1.0 = full

	trackImg draws.Image // full-width background
	fillImg  draws.Image // accent colour; scaled horizontally to Value

	x, y, w, h float64
}

func NewProgressBar(x, y, w, h float64) ProgressBar {
	track := draws.CreateImage(w, h)
	track.Fill(color.RGBA{72, 72, 74, 255})

	fill := draws.CreateImage(w, h)
	fill.Fill(color.RGBA{10, 132, 255, 255})

	return ProgressBar{
		trackImg: track,
		fillImg:  fill,
		x:        x,
		y:        y,
		w:        w,
		h:        h,
	}
}

func (p ProgressBar) Draw(dst draws.Image) {
	track := draws.NewSprite(p.trackImg)
	track.Locate(p.x, p.y, draws.LeftTop)
	track.Draw(dst)

	v := clamp01(p.Value)
	if v > 0 {
		fill := draws.NewSprite(p.fillImg)
		fill.Locate(p.x, p.y, draws.LeftTop)
		fill.Size = draws.XY{X: p.w * v, Y: p.h}
		fill.Draw(dst)
	}
}
