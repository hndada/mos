package apps

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

type homeIcon struct {
	sprite draws.Sprite
	color  color.RGBA
	appID  string
}

// DefaultHome renders a grid of placeholder app icon slots and detects taps.
type DefaultHome struct {
	icons       []homeIcon
	tappedPos   draws.XY
	tappedSize  draws.XY
	tappedColor color.RGBA
	tappedAppID string
	hasTap      bool
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

	icons := make([]homeIcon, 0, cols*rows)
	for r := range rows {
		for c := range cols {
			cx := (float64(c) + 0.5) * cellW
			cy := screenH*topPad + (float64(r)+0.5)*cellH

			clr := iconColors[(r*cols+c)%len(iconColors)]
			img := newRoundedIconImage(side, clr)

			sp := draws.NewSprite(img)
			sp.Locate(cx, cy, draws.CenterMiddle)
			idx := r*cols + c
			appID := "color"
			switch idx {
			case 0:
				appID = "gallery"
			case 1:
				appID = "settings"
			case 2:
				appID = "call"
			case 3:
				appID = "scene-test"
			case 4:
				appID = "hello"
			case 5:
				appID = "showcase"
			case 6:
				appID = "message"
			}
			icons = append(icons, homeIcon{sprite: sp, color: clr, appID: appID})
		}
	}

	return &DefaultHome{icons: icons}
}

func newRoundedIconImage(side float64, clr color.RGBA) draws.Image {
	img := draws.CreateImage(side, side)
	s := float32(side)
	r := s * 0.24
	vector.DrawFilledRect(img.Image, r, 0, s-2*r, s, clr, true)
	vector.DrawFilledRect(img.Image, 0, r, s, s-2*r, clr, true)
	vector.DrawFilledCircle(img.Image, r, r, r, clr, true)
	vector.DrawFilledCircle(img.Image, s-r, r, r, clr, true)
	vector.DrawFilledCircle(img.Image, r, s-r, r, clr, true)
	vector.DrawFilledCircle(img.Image, s-r, s-r, r, clr, true)
	return img
}

func (h *DefaultHome) Update() {
	h.hasTap = false
	if !input.IsMouseButtonJustPressed(input.MouseButtonLeft) {
		return
	}
	x, y := input.MouseCursorPosition()
	cursor := draws.XY{X: x, Y: y}
	for _, icon := range h.icons {
		if icon.sprite.In(cursor) {
			h.tappedPos = icon.sprite.Position
			h.tappedSize = icon.sprite.Size
			h.tappedColor = icon.color
			h.tappedAppID = icon.appID
			h.hasTap = true
			return
		}
	}
}

func (h *DefaultHome) TappedIcon() (pos, size draws.XY, clr color.RGBA, appID string, ok bool) {
	return h.tappedPos, h.tappedSize, h.tappedColor, h.tappedAppID, h.hasTap
}

func (h *DefaultHome) Draw(dst draws.Image) {
	for _, icon := range h.icons {
		icon.sprite.Draw(dst)
	}
}
