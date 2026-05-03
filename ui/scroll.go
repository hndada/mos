package ui

import (
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

const wheelScrollSpeed = 40.0

// ScrollBox is a viewport over content that may exceed it in either or both axes.
// Scrolling is enabled per-axis automatically when ContentSize exceeds the viewport size.
// Call Update each frame, then use Offset() to shift children before drawing.
type ScrollBox struct {
	Box
	ContentSize draws.XY

	offset     draws.XY
	prevCursor draws.XY
	scrolling  bool
}

func (s *ScrollBox) maxOffset() draws.XY {
	return draws.XY{
		X: max(s.ContentSize.X-s.W(), 0),
		Y: max(s.ContentSize.Y-s.H(), 0),
	}
}

func (s *ScrollBox) Offset() draws.XY { return s.offset }

func (s *ScrollBox) ScrollBy(d draws.XY) {
	m := s.maxOffset()
	s.offset.X = min(max(s.offset.X+d.X, 0), m.X)
	s.offset.Y = min(max(s.offset.Y+d.Y, 0), m.Y)
}

// Update processes wheel and drag-to-scroll events from the frame.
// Drag tracking starts only when a Down event lands inside the box; Move
// events (with the button held, indicated by an in-flight tracking state)
// pan the offset.
func (s *ScrollBox) Update(frame mosapp.Frame) {
	for _, ev := range frame.Events {
		switch ev.Kind {
		case input.EventWheel:
			s.ScrollBy(draws.XY{
				X: -ev.Wheel.X * wheelScrollSpeed,
				Y: -ev.Wheel.Y * wheelScrollSpeed,
			})
		case input.EventDown:
			if s.In(ev.Pos) {
				s.scrolling = true
				s.prevCursor = ev.Pos
			}
		case input.EventMove:
			if s.scrolling {
				s.ScrollBy(s.prevCursor.Sub(ev.Pos))
				s.prevCursor = ev.Pos
			}
		case input.EventUp:
			s.scrolling = false
		}
	}
}
