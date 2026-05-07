package ui

import (
	"image/color"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui/theme"
)

// ProgressBar is a horizontal fill indicator. Value is clamped to [0, 1].
// It is display-only; drive Value directly from app state.
//
//	bar.Value = float64(downloaded) / float64(total)
//	bar.Draw(dst)
//
// Track and fill colours are read from the active theme at draw time, so a
// theme switch is reflected on the very next frame without reconstructing the widget.
type ProgressBar struct {
	Value float64 // 0.0 = empty, 1.0 = full

	trackImg draws.Image // white; tinted by theme.SurfaceWidget at draw time
	fillImg  draws.Image // white; tinted by theme.Accent at draw time

	x, y, w, h float64
}

func NewProgressBar(x, y, w, h float64) ProgressBar {
	// Create white images; colours are applied via ColorScale in Draw so that a
	// runtime theme change is reflected without reconstructing the widget.
	track := draws.CreateImage(w, h)
	track.Fill(color.White)

	fill := draws.CreateImage(w, h)
	fill.Fill(color.White)

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
	// Track — tinted with the SurfaceWidget theme colour.
	track := draws.NewSprite(p.trackImg)
	track.Locate(p.x, p.y, draws.LeftTop)
	track.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.SurfaceWidget)))
	track.Draw(dst)

	// Fill — tinted with the Accent theme colour; clipped to Value.
	v := clamp01(p.Value)
	if v > 0 {
		fill := draws.NewSprite(p.fillImg)
		fill.Locate(p.x, p.y, draws.LeftTop)
		fill.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.Accent)))
		fill.Size = draws.XY{X: p.w * v, Y: p.h}
		fill.Draw(dst)
	}
}
