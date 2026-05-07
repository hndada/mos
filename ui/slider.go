package ui

import (
	"image/color"
	"math"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/ui/theme"
)

const (
	SliderTrackH = 4.0
	SliderThumbR = 9.0 // half-width of square thumb
)

// Slider is a horizontal drag control with Value in [0, 1].
// Position is the left edge of the track, vertically centred at y.
// Track and thumb colours are read from the active theme at draw time.
type Slider struct {
	Value    float64
	track    draws.Sprite // white image; tinted by theme.SurfaceWidget at draw time
	filled   draws.Sprite // white image; tinted by theme.Accent at draw time
	thumb    draws.Sprite // white circle; tinted by theme.Knob at draw time
	x, y, w  float64
	dragging bool
}

func NewSlider(x, y, w, val float64) Slider {
	// Create white images; colours are applied via ColorScale in Draw so that a
	// runtime theme change is reflected without rebuilding the widget.
	trackImg := draws.CreateImage(w, SliderTrackH)
	trackImg.Fill(color.White)
	trackSp := draws.NewSprite(trackImg)
	trackSp.Locate(x, y-SliderTrackH/2, draws.LeftTop)

	filledImg := draws.CreateImage(w, SliderTrackH)
	filledImg.Fill(color.White)
	filledSp := draws.NewSprite(filledImg)
	filledSp.Locate(x, y-SliderTrackH/2, draws.LeftTop)

	thumbSp := newCircleSprite(SliderThumbR*2, color.White)

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

// Draw renders the slider at its stored content-space position.
func (s Slider) Draw(dst draws.Image) { s.DrawAt(dst, 0) }

// DrawAt renders the slider with all Y coordinates shifted by yOffset.
// Use this when drawing into a VirtualList canvas: pass vl.ContentToCanvas(row.Y).
func (s Slider) DrawAt(dst draws.Image, yOffset float64) {
	// Track — tinted with the SurfaceWidget theme colour.
	track := s.track
	track.Position.Y += yOffset
	track.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.SurfaceWidget)))
	track.Draw(dst)

	// Filled portion — tinted with the Accent theme colour; clipped to Value.
	if s.Value > 0 {
		filled := s.filled
		filled.Position.Y += yOffset
		filled.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.Accent)))
		filled.Size.X = s.Value * s.w
		filled.Draw(dst)
	}

	// Thumb — tinted with the Knob theme colour.
	thumb := s.thumb
	thumb.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.Knob)))
	thumb.Position = draws.XY{
		X: s.thumbCX() - SliderThumbR,
		Y: s.y - SliderThumbR + yOffset,
	}
	thumb.Draw(dst)
}

func clamp01(v float64) float64 { return min(max(v, 0.0), 1.0) }
