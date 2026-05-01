package apps

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
	hCardMaxWFrac = 0.84 // max card width as fraction of screenW
	hCardMaxHFrac = 0.65 // max card height as fraction of screenH
	hStepFrac     = 0.74 // step between cards; < hCardWFrac ??cards overlap + adjacent cards peek
	cardBorderW   = 3.0
	durationShow  = 350 * time.Millisecond
)

type historyState int

const (
	historyHidden historyState = iota
	historyShowing
	historyShown
	historyHiding
)

type HistoryEntry struct {
	AppID    string
	Color    color.RGBA
	Snapshot draws.Image
}

type histCard struct {
	bg    draws.Sprite
	entry HistoryEntry
}

// DefaultHistory shows recent apps as a horizontally scrollable card carousel.
// Card 0 (most recent) is centered in the viewport at scroll offset 0.
type DefaultHistory struct {
	cards       []histCard
	scroll      ui.ScrollBox
	overlay     draws.Sprite
	alpha       tween.Tween
	state       historyState
	tappedPos   draws.XY
	tappedSize  draws.XY
	tappedEntry HistoryEntry
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
	return NewDefaultHistoryWithCardAspect(screenW, screenH, screenW, screenH)
}

func NewDefaultHistoryWithCardAspect(screenW, screenH, aspectW, aspectH float64) *DefaultHistory {
	maxCardW := screenW * hCardMaxWFrac
	maxCardH := screenH * hCardMaxHFrac
	cardW, cardH := fitAspect(maxCardW, maxCardH, aspectW, aspectH)
	statusBarH := statusBarHeight
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

func fitAspect(maxW, maxH, aspectW, aspectH float64) (w, h float64) {
	if aspectW <= 0 || aspectH <= 0 {
		return maxW, maxH
	}
	ratio := aspectW / aspectH
	w = maxW
	h = w / ratio
	if h > maxH {
		h = maxH
		w = h * ratio
	}
	return w, h
}

func (h *DefaultHistory) Show() {
	// Snap scroll to card 0 so the most-recent card is centered on entry.
	off := h.scroll.Offset()
	h.scroll.ScrollBy(draws.XY{X: -off.X, Y: -off.Y})
	h.state = historyShowing
	h.alpha = h.newAlphaTween(h.alphaValue(), 1)
}

func (h *DefaultHistory) Hide() {
	if h.state == historyHidden || h.state == historyHiding {
		return
	}
	h.state = historyHiding
	h.alpha = h.newAlphaTween(h.alphaValue(), 0)
}

func (h *DefaultHistory) IsVisible() bool {
	return h.state != historyHidden || h.alphaValue() > 0
}

func (h *DefaultHistory) IsInteractive() bool {
	return h.state == historyShowing || h.state == historyShown
}

func (h *DefaultHistory) alphaValue() float64 {
	if len(h.alpha.Units) == 0 {
		return 0
	}
	return h.alpha.Value()
}

func (h *DefaultHistory) newAlphaTween(from, to float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(from, to-from, durationShow, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func (h *DefaultHistory) AddCard(entry HistoryEntry) {
	for i, c := range h.cards {
		if c.entry.AppID == entry.AppID && c.entry.Color == entry.Color {
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
	inner.Fill(entry.Color)
	if !entry.Snapshot.IsEmpty() {
		shot := draws.NewSprite(entry.Snapshot)
		shotSize := entry.Snapshot.Size()
		innerSize := inner.Size()
		scale := min(innerSize.X/shotSize.X, innerSize.Y/shotSize.Y)
		shot.Size = shotSize.Scale(scale)
		shot.Locate(innerSize.X/2, innerSize.Y/2, draws.CenterMiddle)
		shot.Draw(inner)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cardBorderW, cardBorderW)
	img.DrawImage(inner.Image, op)

	sp := draws.NewSprite(img)
	h.cards = append([]histCard{{bg: sp, entry: entry}}, h.cards...)
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

// Entries returns app history newest-first for persistence across screen changes.
func (h *DefaultHistory) Entries() []HistoryEntry {
	entries := make([]HistoryEntry, len(h.cards))
	for i, c := range h.cards {
		entries[i] = c.entry
	}
	return entries
}

func (h *DefaultHistory) TappedCard() (pos, size draws.XY, entry HistoryEntry, ok bool) {
	return h.tappedPos, h.tappedSize, h.tappedEntry, h.hasTap
}

func (h *DefaultHistory) Update() {
	h.hasTap = false
	if h.state == historyHidden {
		return
	}
	h.alpha.Update()
	switch h.state {
	case historyShowing:
		if h.alpha.IsFinished() {
			h.state = historyShown
		}
	case historyHiding:
		if h.alpha.IsFinished() {
			h.state = historyHidden
		}
	}
	if !h.IsInteractive() {
		h.dragActive = false
		return
	}

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
			h.tappedEntry = card.entry
			h.hasTap = true
			h.cards = append(h.cards[:i], h.cards[i+1:]...)
			h.recalcPositions()
			return
		}
	}
}

func (h *DefaultHistory) Draw(dst draws.Image) {
	if !h.IsVisible() {
		return
	}
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
