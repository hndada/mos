package ui

import (
	"image/color"
	"math"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

const (
	SliderTrackH = 4.0
	SliderThumbR = 9.0 // half-width of square thumb
)

// Slider is a horizontal drag control with Value in [0, 1].
// Position is the left edge of the track, vertically centred at y.
type Slider struct {
	Value    float64
	track    draws.Sprite
	filled   draws.Sprite
	thumb    draws.Sprite
	x, y, w  float64
	dragging bool
}

func NewSlider(x, y, w, val float64) Slider {
	trackImg := draws.CreateImage(w, SliderTrackH)
	trackImg.Fill(color.RGBA{72, 72, 74, 255})
	trackSp := draws.NewSprite(trackImg)
	trackSp.Locate(x, y-SliderTrackH/2, draws.LeftTop)

	filledImg := draws.CreateImage(w, SliderTrackH)
	filledImg.Fill(color.RGBA{10, 132, 255, 255})
	filledSp := draws.NewSprite(filledImg)
	filledSp.Locate(x, y-SliderTrackH/2, draws.LeftTop)

	thumbSp := newCircleSprite(SliderThumbR*2, color.RGBA{255, 255, 255, 255})

	s := Slider{
		Value:  clamp01(val),
		track:  trackSp,
		filled: filledSp,
		thumb:  thumbSp,
		x:      x,
		y:      y,
		w:      w,
	}
	return s
}

func (s *Slider) thumbCX() float64 { return s.x + s.Value*s.w }

// Update handles drag interaction. cursor must be in the same space as the slider.
func (s *Slider) Update(cursor draws.XY) {
	if input.IsMouseButtonJustPressed(input.MouseButtonLeft) {
		tx := s.thumbCX()
		if math.Abs(cursor.X-tx) <= SliderThumbR*2 && math.Abs(cursor.Y-s.y) <= SliderThumbR*2 {
			s.dragging = true
		}
	}
	if !input.IsMouseButtonPressed(input.MouseButtonLeft) {
		s.dragging = false
	}
	if s.dragging {
		s.Value = clamp01((cursor.X - s.x) / s.w)
	}
}

func (s Slider) Draw(dst draws.Image) {
	s.track.Draw(dst)

	if s.Value > 0 {
		f := s.filled
		f.Size.X = s.Value * s.w
		f.Draw(dst)
	}

	th := s.thumb
	th.Position = draws.XY{
		X: s.thumbCX() - SliderThumbR,
		Y: s.y - SliderThumbR,
	}
	th.Draw(dst)
}

func clamp01(v float64) float64 { return min(max(v, 0.0), 1.0) }
