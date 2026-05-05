package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

const (
	TextFieldH    = 44.0
	textFieldPadX = 12.0
)

// TextField is a single-line text-input widget.
//
// Tap to focus; any EventDown outside the field blurs it. While focused,
// PollKeyboard reads printable characters and Backspace/Delete from
// Ebiten's physical-keyboard APIs — call it once per frame after Update.
//
// Real-OS analogue: in a production multi-process OS typed characters
// arrive through an IME / system keyboard service. PollKeyboard is a
// simulator convenience that makes the physical keyboard drive the field
// without wiring up the on-screen keyboard routing.
type TextField struct {
	Value       string
	Placeholder string
	MaxLen      int // 0 = unlimited

	focused bool
	gesture GestureDetector

	bgNormal  draws.Image
	bgFocused draws.Image
	textEl    draws.Text
	hintEl    draws.Text

	x, y, w float64
}

func NewTextField(x, y, w float64, placeholder string) TextField {
	bgN := draws.CreateImage(w, TextFieldH)
	bgN.Fill(color.RGBA{44, 46, 56, 255})

	bgF := draws.CreateImage(w, TextFieldH)
	bgF.Fill(color.RGBA{44, 46, 56, 255})
	// Accent underline painted into the focused background image.
	underline := draws.CreateImage(w, 2)
	underline.Fill(color.RGBA{10, 132, 255, 255})
	ulSp := draws.NewSprite(underline)
	ulSp.Locate(w/2, TextFieldH-1, draws.CenterMiddle)
	ulSp.Draw(bgF)

	valueOpts := draws.NewFaceOptions()
	valueOpts.Size = 15
	textEl := draws.NewText("")
	textEl.SetFace(valueOpts)
	textEl.Locate(x+textFieldPadX, y+TextFieldH/2, draws.LeftMiddle)

	hintOpts := draws.NewFaceOptions()
	hintOpts.Size = 15
	hintEl := draws.NewText(placeholder)
	hintEl.SetFace(hintOpts)
	hintEl.Locate(x+textFieldPadX, y+TextFieldH/2, draws.LeftMiddle)

	return TextField{
		Placeholder: placeholder,
		gesture:     NewGestureDetector(x, y, w, TextFieldH),
		bgNormal:    bgN,
		bgFocused:   bgF,
		textEl:      textEl,
		hintEl:      hintEl,
		x:           x,
		y:           y,
		w:           w,
	}
}

// Update processes tap events to change focus. Returns true when the focus
// state changes so the caller can show/hide the system keyboard.
func (tf *TextField) Update(frame mosapp.Frame) (focusChanged bool) {
	for _, ev := range frame.Events {
		if ev.Kind != input.EventDown {
			continue
		}
		prev := tf.focused
		tf.focused = tf.gesture.Area.In(ev.Pos)
		if tf.focused != prev {
			focusChanged = true
		}
	}
	return
}

func (tf *TextField) IsFocused() bool { return tf.focused }
func (tf *TextField) Focus()          { tf.focused = true }
func (tf *TextField) Blur()           { tf.focused = false }
func (tf *TextField) Clear()          { tf.Value = "" }

// PollKeyboard reads physical keyboard input and appends it to Value.
// Call once per frame while IsFocused() is true.
func (tf *TextField) PollKeyboard() {
	if !tf.focused {
		return
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if tf.MaxLen > 0 && len([]rune(tf.Value)) >= tf.MaxLen {
			break
		}
		tf.Value += string(r)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		runes := []rune(tf.Value)
		if len(runes) > 0 {
			tf.Value = string(runes[:len(runes)-1])
		}
	}
}

func (tf *TextField) Draw(dst draws.Image) {
	bg := tf.bgNormal
	if tf.focused {
		bg = tf.bgFocused
	}
	bgSp := draws.NewSprite(bg)
	bgSp.Locate(tf.x, tf.y, draws.LeftTop)
	bgSp.Draw(dst)

	if tf.Value == "" {
		h := tf.hintEl
		h.ColorScale.Scale(0.55, 0.55, 0.55, 1)
		h.Draw(dst)
		return
	}
	display := tf.Value
	if tf.focused {
		display += "|"
	}
	tf.textEl.Text = display
	tf.textEl.Draw(dst)
}
