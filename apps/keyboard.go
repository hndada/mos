package apps

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/tween"
)

const DurationKB = 220 * time.Millisecond

var keyboardRowsQwerty = [][]string{
	{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
	{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
	{"z", "x", "c", "v", "b", "n", "m"},
	{"space"},
}

// DefaultKeyboard renders a QWERTY layout that slides up from the bottom of the screen.
type DefaultKeyboard struct {
	canvas  draws.Image
	screenW float64
	screenH float64
	kbH     float64
	slideY  tween.Tween // 0 = fully on-screen, kbH = fully off-screen below
	shown   bool
}

func kbAnim(from, to, kbH float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	delta := to - from
	if delta < 0 {
		delta = -delta
	}
	// Duration scales with distance so interrupted animations feel natural.
	d := time.Duration(float64(DurationKB) * delta / kbH)
	if d < 16*time.Millisecond {
		d = 16 * time.Millisecond
	}
	tw.Add(from, to-from, d, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func NewDefaultKeyboard(screenW, screenH float64) *DefaultKeyboard {
	const numCols = 10
	const keyPad = 4.0

	kbH := screenH * 0.35
	keyW := screenW / numCols
	keyH := kbH / float64(len(keyboardRowsQwerty))

	// Pre-render all keys into a canvas whose top-left is the keyboard top.
	canvas := draws.CreateImage(screenW, kbH)
	canvas.Fill(color.RGBA{28, 28, 30, 255})

	for r, row := range keyboardRowsQwerty {
		totalW := float64(len(row)) * keyW
		xStart := (screenW - totalW) / 2
		cy := (float64(r) + 0.5) * keyH

		for c, label := range row {
			kw := keyW
			kx := xStart + (float64(c)+0.5)*keyW
			if label == "space" {
				kw = screenW * 0.6
				kx = screenW / 2
			}

			kImg := draws.CreateImage(kw-keyPad, keyH-keyPad)
			kImg.Fill(color.RGBA{72, 72, 74, 255})
			kSp := draws.NewSprite(kImg)
			kSp.Locate(kx, cy, draws.CenterMiddle)
			kSp.Draw(canvas)

			txt := draws.NewText(label)
			txt.Locate(kx, cy, draws.CenterMiddle)
			txt.Draw(canvas)
		}
	}

	// Start fully off-screen (slideY = kbH).
	slideY := kbAnim(kbH, kbH, kbH)

	return &DefaultKeyboard{
		canvas:  canvas,
		screenW: screenW,
		screenH: screenH,
		kbH:     kbH,
		slideY:  slideY,
		shown:   false,
	}
}

func (k *DefaultKeyboard) Show() {
	if k.shown {
		return
	}
	k.shown = true
	k.slideY = kbAnim(k.slideY.Value(), 0, k.kbH)
}

func (k *DefaultKeyboard) Hide() {
	if !k.shown {
		return
	}
	k.shown = false
	k.slideY = kbAnim(k.slideY.Value(), k.kbH, k.kbH)
}

func (k *DefaultKeyboard) IsVisible() bool { return k.shown }

func (k *DefaultKeyboard) Update() {
	k.slideY.Update()
}

func (k *DefaultKeyboard) Draw(dst draws.Image) {
	y := k.slideY.Value()
	if y >= k.kbH {
		return
	}
	sp := draws.NewSprite(k.canvas)
	sp.Locate(0, k.screenH-k.kbH+y, draws.LeftTop)
	sp.Draw(dst)
}
