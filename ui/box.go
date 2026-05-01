package ui

import "github.com/hndada/mos/internal/draws"

type Box struct {
	draws.Box
	ID        uint64
	isFocused bool

	isDraggable bool
	isDragging  bool
	dragOffset  draws.XY // cursor offset relative to box origin at drag start
}

func (b *Box) BeginDrag(cursor draws.XY) {
	b.isDragging = true
	b.dragOffset = cursor.Sub(b.Position)
}

// UpdateDrag repositions the box to follow the cursor.
func (b *Box) UpdateDrag(cursor draws.XY) {
	if !b.isDragging {
		return
	}
	b.Position = cursor.Sub(b.dragOffset)
}

func (b *Box) EndDrag()         { b.isDragging = false }
func (b *Box) IsDragging() bool { return b.isDragging }

// GhostSprite returns a half-alpha sprite at the drag position for the compositor
// to paint as the ghost window while dragging.
func (b Box) GhostSprite(src draws.Image) draws.Sprite {
	s := draws.NewSprite(src)
	s.Position = b.Position
	s.Size = b.Size
	s.Aligns = b.Aligns
	s.ColorScale.ScaleAlpha(0.5)
	return s
}
