package ui

import (
	"image/color"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
)

const (
	ListTileH       = 56.0 // fixed row height
	listTilePadX    = 16.0
	listTileSubSize = 12.0
	listTileTxtSize = 15.0
)

// ListTile is a single interactive row: an optional leading icon area,
// a title, an optional subtitle, and an optional trailing label.
// The entire row is tappable — Update returns true on a tap.
//
// Use inside a ScrollBox to build scrollable lists:
//
//	for i, tile := range tiles {
//	    tile.SetY(scroll.Offset().Y + float64(i)*ListTileH)
//	    if tile.Update(frame) { ... }
//	    tile.Draw(canvas)
//	}
type ListTile struct {
	// Data fields — set directly between frames to update content.
	Title        string
	Subtitle     string // empty = title is vertically centred
	TrailingText string // e.g. ">" or a value label

	gesture GestureDetector

	titleEl    draws.Text
	subtitleEl draws.Text
	trailEl    draws.Text

	x, y, w float64
}

func NewListTile(x, y, w float64, title, subtitle string) ListTile {
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = listTileTxtSize
	titleEl := draws.NewText(title)
	titleEl.SetFace(titleOpts)

	subOpts := draws.NewFaceOptions()
	subOpts.Size = listTileSubSize
	subEl := draws.NewText(subtitle)
	subEl.SetFace(subOpts)

	trailOpts := draws.NewFaceOptions()
	trailOpts.Size = listTileSubSize
	trailEl := draws.NewText("")
	trailEl.SetFace(trailOpts)

	t := ListTile{
		Title:      title,
		Subtitle:   subtitle,
		gesture:    NewGestureDetector(x, y, w, ListTileH),
		titleEl:    titleEl,
		subtitleEl: subEl,
		trailEl:    trailEl,
		x:          x,
		y:          y,
		w:          w,
	}
	t.layout()
	return t
}

// SetY repositions the tile vertically. Used when the parent ScrollBox
// shifts the content offset.
func (t *ListTile) SetY(y float64) {
	if t.y == y {
		return
	}
	t.y = y
	t.gesture.Area.Locate(t.x, y, draws.LeftTop)
	t.layout()
}

func (t *ListTile) layout() {
	cx := t.x + listTilePadX
	if t.Subtitle == "" {
		t.titleEl.Locate(cx, t.y+ListTileH/2, draws.LeftMiddle)
	} else {
		t.titleEl.Locate(cx, t.y+ListTileH/2-9, draws.LeftMiddle)
		t.subtitleEl.Locate(cx, t.y+ListTileH/2+10, draws.LeftMiddle)
	}
	t.trailEl.Locate(t.x+t.w-listTilePadX, t.y+ListTileH/2, draws.RightMiddle)
}

// Update returns true when the tile is tapped.
func (t *ListTile) Update(frame mosapp.Frame) bool {
	return t.gesture.Update(frame).Kind == GestureTap
}

func (t *ListTile) Draw(dst draws.Image) {
	// Press highlight.
	if t.gesture.IsHeld() {
		hl := draws.CreateImage(t.w, ListTileH)
		hl.Fill(color.RGBA{255, 255, 255, 18})
		hlSp := draws.NewSprite(hl)
		hlSp.Locate(t.x, t.y, draws.LeftTop)
		hlSp.Draw(dst)
	}

	t.titleEl.Text = t.Title
	t.titleEl.Draw(dst)

	if t.Subtitle != "" {
		t.subtitleEl.Text = t.Subtitle
		t.subtitleEl.ColorScale.Scale(0.6, 0.6, 0.6, 1)
		t.subtitleEl.Draw(dst)
	}

	if t.TrailingText != "" {
		t.trailEl.Text = t.TrailingText
		t.trailEl.ColorScale.Scale(0.55, 0.55, 0.55, 1)
		t.trailEl.Draw(dst)
	}
}

// Divider draws a horizontal hairline separator between list rows.
type Divider struct {
	sprite draws.Sprite
}

func NewDivider(x, y, w float64) Divider {
	img := draws.CreateImage(w, 1)
	img.Fill(color.RGBA{255, 255, 255, 28})
	sp := draws.NewSprite(img)
	sp.Locate(x, y, draws.LeftTop)
	return Divider{sprite: sp}
}

func (d *Divider) SetY(y float64) {
	d.sprite.Position.Y = y
}

func (d Divider) Draw(dst draws.Image) {
	d.sprite.Draw(dst)
}
