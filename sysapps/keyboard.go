package sysapps

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
)

const DurationKB = 220 * time.Millisecond

var keyboardRowsQwerty = [][]string{
	{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
	{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
	{"z", "x", "c", "v", "b", "n", "m"},
	{"space"},
}

type Keyboard interface {
	Show()
	Hide()
	IsVisible() bool
	Update()
	Draw(dst draws.Image)
}

type kbKey struct {
	bg    draws.Sprite
	label draws.Text
}

type DefaultKeyboard struct {
	visible bool
	bg      draws.Sprite
	rows    [][]kbKey
}

func NewDefaultKeyboard(screenW, screenH float64) *DefaultKeyboard {
	const numCols = 10
	const keyPad = 4.0

	kbH := screenH * 0.35
	kbY := screenH - kbH
	keyW := screenW / numCols
	keyH := kbH / float64(len(keyboardRowsQwerty))

	bgImg := draws.CreateImage(screenW, kbH)
	bgImg.Fill(color.RGBA{28, 28, 30, 255})
	bg := draws.NewSprite(bgImg)
	bg.Locate(screenW/2, kbY+kbH/2, draws.CenterMiddle)

	rows := make([][]kbKey, len(keyboardRowsQwerty))
	for r, row := range keyboardRowsQwerty {
		rows[r] = make([]kbKey, len(row))
		totalW := float64(len(row)) * keyW
		xStart := (screenW - totalW) / 2
		cy := kbY + (float64(r)+0.5)*keyH

		for c, label := range row {
			w := keyW
			x := xStart + (float64(c)+0.5)*keyW
			if label == "space" {
				w = screenW * 0.6
				x = screenW / 2
			}

			kImg := draws.CreateImage(w-keyPad, keyH-keyPad)
			kImg.Fill(color.RGBA{72, 72, 74, 255})
			sp := draws.NewSprite(kImg)
			sp.Locate(x, cy, draws.CenterMiddle)

			txt := draws.NewText(label)
			txt.Locate(x, cy, draws.CenterMiddle)

			rows[r][c] = kbKey{bg: sp, label: txt}
		}
	}

	return &DefaultKeyboard{bg: bg, rows: rows}
}

func (k *DefaultKeyboard) Show()           { k.visible = true }
func (k *DefaultKeyboard) Hide()           { k.visible = false }
func (k *DefaultKeyboard) IsVisible() bool { return k.visible }

func (k *DefaultKeyboard) Update() {}

func (k *DefaultKeyboard) Draw(dst draws.Image) {
	if !k.visible {
		return
	}
	k.bg.Draw(dst)
	for _, row := range k.rows {
		for _, key := range row {
			key.bg.Draw(dst)
			key.label.Draw(dst)
		}
	}
}
