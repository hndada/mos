package ui

import (
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

const wheelScrollSpeed = 40.0

// ScrollBox is a viewport over content that may exceed it in either or both axes.
// Scrolling is enabled per-axis automatically when ContentSize exceeds the viewport size.
// Call HandleInput each frame, then use Offset() to shift children before drawing.
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

// HandleInput processes mouse wheel (both axes) and drag-to-scroll. Call once per frame.
func (s *ScrollBox) HandleInput(cursor draws.XY) {
	wx, wy := input.MouseWheelPosition()
	if wx != 0 || wy != 0 {
		s.ScrollBy(draws.XY{X: -wx * wheelScrollSpeed, Y: -wy * wheelScrollSpeed})
	}

	if input.IsMouseButtonPressed(input.MouseButtonLeft) && s.In(cursor) {
		if s.scrolling {
			s.ScrollBy(s.prevCursor.Sub(cursor))
		}
		s.scrolling = true
		s.prevCursor = cursor
	} else {
		s.scrolling = false
	}
}
