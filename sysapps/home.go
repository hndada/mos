package sysapps

import (
	"image/color"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

type Home interface {
	Update()
	Draw(dst draws.Image)
	// TappedIcon returns the center position and size of the icon tapped this frame.
	TappedIcon() (pos, size draws.XY, ok bool)
}

// DefaultHome renders a grid of placeholder app icon slots and detects taps.
type DefaultHome struct {
	icons      []draws.Sprite
	tappedPos  draws.XY
	tappedSize draws.XY
	hasTap     bool
}

var iconColors = []color.RGBA{
	{88, 86, 214, 255},  // indigo
	{255, 59, 48, 255},  // red
	{52, 199, 89, 255},  // green
	{255, 149, 0, 255},  // orange
	{0, 122, 255, 255},  // blue
	{175, 82, 222, 255}, // purple
	{255, 45, 85, 255},  // pink
	{90, 200, 250, 255}, // teal
}

func NewDefaultHome(screenW, screenH float64) *DefaultHome {
	const cols = 4
	const rows = 5
	const iconScale = 0.65
	const topPad = 0.1

	cellW := screenW / cols
	cellH := (screenH * (1 - topPad)) / rows
	side := min(cellW, cellH) * iconScale

	icons := make([]draws.Sprite, 0, cols*rows)
	for r := range rows {
		for c := range cols {
			cx := (float64(c) + 0.5) * cellW
			cy := screenH*topPad + (float64(r)+0.5)*cellH

			clr := iconColors[(r*cols+c)%len(iconColors)]
			img := draws.CreateImage(side, side)
			img.Fill(clr)

			sp := draws.NewSprite(img)
			sp.Locate(cx, cy, draws.CenterMiddle)
			icons = append(icons, sp)
		}
	}

	return &DefaultHome{icons: icons}
}

func (h *DefaultHome) Update() {
	h.hasTap = false
	if !input.IsMouseButtonJustPressed(input.MouseButtonLeft) {
		return
	}
	x, y := input.MouseCursorPosition()
	cursor := draws.XY{X: x, Y: y}
	for _, icon := range h.icons {
		if icon.In(cursor) {
			h.tappedPos = icon.Position
			h.tappedSize = icon.Size
			h.hasTap = true
			return
		}
	}
}

func (h *DefaultHome) TappedIcon() (pos, size draws.XY, ok bool) {
	return h.tappedPos, h.tappedSize, h.hasTap
}

func (h *DefaultHome) Draw(dst draws.Image) {
	for _, icon := range h.icons {
		icon.Draw(dst)
	}
}
