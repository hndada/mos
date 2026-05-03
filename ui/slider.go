package ui

import (
	"image/color"
	"math"

	mosapp "github.com/hndada/mos/internal/app"
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

func (s *Slider) thumbHit(p draws.XY) bool {
	tx := s.thumbCX()
	return math.Abs(p.X-tx) <= SliderThumbR*2 && math.Abs(p.Y-s.y) <= SliderThumbR*2
}

// Update handles drag interaction. Down inside the thumb starts a drag;
// subsequent Moves update Value; Up ends the drag.
func (s *Slider) Update(frame mosapp.Frame) {
	for _, ev := range frame.Events {
		switch ev.Kind {
		case input.EventDown:
			if s.thumbHit(ev.Pos) {
				s.dragging = true
				s.Value = clamp01((ev.Pos.X - s.x) / s.w)
			}
		case input.EventMove:
			if s.dragging {
				s.Value = clamp01((ev.Pos.X - s.x) / s.w)
			}
		case input.EventUp:
			s.dragging = false
		}
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
