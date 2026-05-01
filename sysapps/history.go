package sysapps

import (
	"image/color"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/ui"
)

const MaxHistory = 50

const (
	cardH   = 200.0
	cardGap = 16.0
)

var cardColors = []color.RGBA{
	{52, 120, 246, 255},
	{52, 199, 89, 255},
	{255, 149, 0, 255},
	{255, 59, 48, 255},
	{175, 82, 222, 255},
}

type History interface {
	AddCard()
	RemoveCard()
	Update()
	Draw(dst draws.Image)
}

type histCard struct {
	bg draws.Sprite
}

// DefaultHistory shows recent apps as a vertically scrollable card stack.
type DefaultHistory struct {
	cards   []histCard
	scroll  ui.ScrollBox
	screenW float64
	screenH float64
}

func NewDefaultHistory(screenW, screenH float64) *DefaultHistory {
	h := &DefaultHistory{screenW: screenW, screenH: screenH}
	h.scroll.Size = draws.XY{X: screenW, Y: screenH}
	h.scroll.Locate(0, 0, draws.LeftTop)
	return h
}

func (h *DefaultHistory) AddCard() {
	if len(h.cards) >= MaxHistory {
		return
	}
	n := len(h.cards)
	cx := h.screenW / 2
	cy := float64(n)*(cardH+cardGap) + cardH/2

	img := draws.CreateImage(h.screenW*0.85, cardH)
	img.Fill(cardColors[n%len(cardColors)])

	sp := draws.NewSprite(img)
	sp.Locate(cx, cy, draws.CenterMiddle)

	h.cards = append(h.cards, histCard{bg: sp})
	h.scroll.ContentSize.Y = float64(len(h.cards))*(cardH+cardGap) - cardGap
}

func (h *DefaultHistory) RemoveCard() {
	if len(h.cards) == 0 {
		return
	}
	h.cards = h.cards[:len(h.cards)-1]
	h.scroll.ContentSize.Y = max(float64(len(h.cards))*(cardH+cardGap)-cardGap, 0)
}

func (h *DefaultHistory) Update() {
	x, y := input.MouseCursorPosition()
	h.scroll.HandleInput(draws.XY{X: x, Y: y})
}

func (h *DefaultHistory) Draw(dst draws.Image) {
	if len(h.cards) == 0 {
		return
	}
	off := h.scroll.Offset()
	for _, card := range h.cards {
		c := card.bg
		c.Position = c.Position.Sub(off)
		c.Draw(dst)
	}
}
