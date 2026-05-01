package sysapps

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/tween"
	"github.com/hndada/mos/ui"
)

const MaxHistory = 50

const (
	hCardWFrac   = 0.84 // card width as fraction of screenW
	hCardHFrac   = 0.65 // card height as fraction of screenH
	hStepFrac    = 0.74 // step between cards; < hCardWFrac → cards overlap + adjacent cards peek
	cardBorderW  = 3.0
	durationShow = 350 * time.Millisecond
)

type History interface {
	AddCard(clr color.RGBA)
	RemoveCard()
	Show()
	TappedCard() (pos, size draws.XY, clr color.RGBA, ok bool)
	// CardRect returns the center and size of the card that would appear at
	// position 0 (newest) when the carousel is fully reset. Used to animate
	// windows shrinking toward the card on dismiss.
	CardRect() (center, size draws.XY)
	// Colors returns card colors newest-first, for persisting across screen changes.
	Colors() []color.RGBA
	Update()
	Draw(dst draws.Image)
}

type histCard struct {
	bg  draws.Sprite
	clr color.RGBA
}

// DefaultHistory shows recent apps as a horizontally scrollable card carousel.
// Card 0 (most recent) is centered in the viewport at scroll offset 0.
type DefaultHistory struct {
	cards       []histCard
	scroll      ui.ScrollBox
	overlay     draws.Sprite
	alpha       tween.Tween
	tappedPos   draws.XY
	tappedSize  draws.XY
	tappedColor color.RGBA
	hasTap      bool
	screenW     float64
	screenH     float64
	cardW       float64
	cardH       float64
	cardTopY    float64 // top Y of cards (vertically centered below status bar)
	hStep       float64 // horizontal distance between card left edges

	dragActive bool
	dragStartX float64
	dragPrevX  float64
}

func NewDefaultHistory(screenW, screenH float64) *DefaultHistory {
	cardW := screenW * hCardWFrac
	cardH := screenH * hCardHFrac
	statusBarH := screenH * statusBarFrac
	cardTopY := statusBarH + (screenH-statusBarH-cardH)/2
	hStep := screenW * hStepFrac

	h := &DefaultHistory{
		screenW:  screenW,
		screenH:  screenH,
		cardW:    cardW,
		cardH:    cardH,
		cardTopY: cardTopY,
		hStep:    hStep,
	}
	h.scroll.Size = draws.XY{X: screenW, Y: screenH}
	h.scroll.Locate(0, 0, draws.LeftTop)

	ovImg := draws.CreateImage(screenW, screenH)
	ovImg.Fill(color.RGBA{0, 0, 0, 200})
	ov := draws.NewSprite(ovImg)
	ov.Locate(0, 0, draws.LeftTop)
	h.overlay = ov

	return h
}

func (h *DefaultHistory) Show() {
	// Snap scroll to card 0 so the most-recent card is centered on entry.
	off := h.scroll.Offset()
	h.scroll.ScrollBy(draws.XY{X: -off.X, Y: -off.Y})

	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(0, 1, durationShow, tween.EaseOutExponential)
	tw.Start()
	h.alpha = tw
}

func (h *DefaultHistory) AddCard(clr color.RGBA) {
	for i, c := range h.cards {
		if c.clr == clr {
			h.cards = append(h.cards[:i], h.cards[i+1:]...)
			break
		}
	}
	if len(h.cards) >= MaxHistory {
		h.cards = h.cards[:MaxHistory-1]
	}

	img := draws.CreateImage(h.cardW, h.cardH)
	img.Fill(color.RGBA{210, 210, 215, 255})
	inner := draws.CreateImage(h.cardW-2*cardBorderW, h.cardH-2*cardBorderW)
	inner.Fill(clr)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cardBorderW, cardBorderW)
	img.DrawImage(inner.Image, op)

	sp := draws.NewSprite(img)
	h.cards = append([]histCard{{bg: sp, clr: clr}}, h.cards...)
	h.recalcPositions()
}

func (h *DefaultHistory) RemoveCard() {
	if len(h.cards) == 0 {
		return
	}
	h.cards = h.cards[:len(h.cards)-1]
	h.recalcPositions()
}

// recalcPositions lays cards out horizontally so that card 0 is centered
// in the viewport when scroll offset is 0, and card n-1 when at max offset.
func (h *DefaultHistory) recalcPositions() {
	leftX0 := (h.screenW - h.cardW) / 2
	for i := range h.cards {
		h.cards[i].bg.Position = draws.XY{
			X: leftX0 + float64(i)*h.hStep,
			Y: h.cardTopY,
		}
	}
	n := len(h.cards)
	if n <= 1 {
		h.scroll.ContentSize.X = 0
	} else {
		// maxOffset = (n-1)*hStep so the last card centers at max scroll.
		h.scroll.ContentSize.X = h.screenW + float64(n-1)*h.hStep
	}
	h.scroll.ContentSize.Y = 0
}

// CardRect returns the center and size of the card at index 0 when scroll is reset.
func (h *DefaultHistory) CardRect() (center, size draws.XY) {
	center = draws.XY{
		X: h.screenW / 2,
		Y: h.cardTopY + h.cardH/2,
	}
	size = draws.XY{X: h.cardW, Y: h.cardH}
	return
}

// Colors returns card colors newest-first for persistence across screen changes.
func (h *DefaultHistory) Colors() []color.RGBA {
	clrs := make([]color.RGBA, len(h.cards))
	for i, c := range h.cards {
		clrs[i] = c.clr
	}
	return clrs
}

func (h *DefaultHistory) TappedCard() (pos, size draws.XY, clr color.RGBA, ok bool) {
	return h.tappedPos, h.tappedSize, h.tappedColor, h.hasTap
}

func (h *DefaultHistory) Update() {
	h.hasTap = false
	x, y := input.MouseCursorPosition()
	cursor := draws.XY{X: x, Y: y}

	// Both wheel axes map to horizontal scroll for the carousel.
	wx, wy := input.MouseWheelPosition()
	if wx != 0 || wy != 0 {
		h.scroll.ScrollBy(draws.XY{X: (-wx - wy) * 40})
	}

	// Horizontal drag scroll. Track distance to distinguish tap from drag.
	if input.IsMouseButtonPressed(input.MouseButtonLeft) && h.scroll.In(cursor) {
		if h.dragActive {
			h.scroll.ScrollBy(draws.XY{X: h.dragPrevX - x})
		} else {
			h.dragStartX = x
		}
		h.dragActive = true
		h.dragPrevX = x
	} else {
		h.dragActive = false
	}

	h.alpha.Update()

	// Register a tap only if the button was just pressed and the finger
	// hasn't moved far enough to count as a scroll drag.
	if !input.IsMouseButtonJustPressed(input.MouseButtonLeft) {
		return
	}
	off := h.scroll.Offset()
	for i, card := range h.cards {
		c := card.bg
		c.Position = c.Position.Sub(draws.XY{X: off.X})
		if c.In(cursor) {
			h.tappedPos = draws.XY{
				X: c.Position.X + c.Size.X/2,
				Y: c.Position.Y + c.Size.Y/2,
			}
			h.tappedSize = c.Size
			h.tappedColor = card.clr
			h.hasTap = true
			h.cards = append(h.cards[:i], h.cards[i+1:]...)
			h.recalcPositions()
			return
		}
	}
}

func (h *DefaultHistory) Draw(dst draws.Image) {
	a := float32(h.alpha.Value())
	if a <= 0 {
		return
	}

	ov := h.overlay
	ov.ColorScale.ScaleAlpha(a)
	ov.Draw(dst)

	off := h.scroll.Offset()
	// Determine which card is closest to center so it renders on top.
	focusIdx := 0
	if h.hStep > 0 && len(h.cards) > 1 {
		f := int(off.X/h.hStep + 0.5)
		if f < 0 {
			f = 0
		} else if f >= len(h.cards) {
			f = len(h.cards) - 1
		}
		focusIdx = f
	}
	// First pass: all cards except the focused one.
	for i, card := range h.cards {
		if i == focusIdx {
			continue
		}
		c := card.bg
		c.Position = c.Position.Sub(draws.XY{X: off.X})
		c.ColorScale.ScaleAlpha(a)
		c.Draw(dst)
	}
	// Second pass: focused card on top.
	if len(h.cards) > 0 {
		c := h.cards[focusIdx].bg
		c.Position = c.Position.Sub(draws.XY{X: off.X})
		c.ColorScale.ScaleAlpha(a)
		c.Draw(dst)
	}
}
