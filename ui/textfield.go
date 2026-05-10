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
	textMenuH     = 28.0
	textMenuW     = 176.0
)

type TextEditAction int

const (
	TextEditNone TextEditAction = iota
	TextEditCopy
	TextEditCut
	TextEditPaste
	TextEditSelectAll
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

	focused   bool
	cursor    int
	selStart  int
	selEnd    int
	selecting bool
	pending   TextEditAction
	gesture   GestureDetector

	bgNormal  draws.Image
	bgFocused draws.Image
	textEl    draws.Text
	hintEl    draws.Text
	selectImg draws.Image
	handleImg draws.Image
	cursorImg draws.Image
	menuBg    draws.Image

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

	selectImg := draws.CreateImage(4, TextFieldH-14)
	selectImg.Fill(color.RGBA{10, 132, 255, 90})
	handleImg := draws.CreateImage(10, 10)
	handleImg.Fill(color.RGBA{10, 132, 255, 255})
	cursorImg := draws.CreateImage(2, TextFieldH-16)
	cursorImg.Fill(color.RGBA{10, 132, 255, 255})
	menuBg := draws.CreateImage(textMenuW, textMenuH)
	menuBg.Fill(color.RGBA{20, 23, 31, 240})

	return TextField{
		Placeholder: placeholder,
		gesture:     NewGestureDetector(x, y, w, TextFieldH),
		bgNormal:    bgN,
		bgFocused:   bgF,
		textEl:      textEl,
		hintEl:      hintEl,
		selectImg:   selectImg,
		handleImg:   handleImg,
		cursorImg:   cursorImg,
		menuBg:      menuBg,
		x:           x,
		y:           y,
		w:           w,
	}
}

// Update processes tap events to change focus. Returns true when the focus
// state changes so the caller can show/hide the system keyboard.
func (tf *TextField) Update(frame mosapp.Frame) (focusChanged bool) {
	for _, ev := range frame.Events {
		inside := tf.gesture.Area.In(ev.Pos)
		switch ev.Kind {
		case input.EventDown:
			if tf.focused && tf.handleMenuDown(ev.Pos) {
				tf.selecting = false
				continue
			}
			prev := tf.focused
			tf.focused = inside
			if tf.focused != prev {
				focusChanged = true
			}
			if inside {
				tf.cursor = tf.indexAt(ev.Pos.X)
				tf.selStart = tf.cursor
				tf.selEnd = tf.cursor
				tf.selecting = true
			} else {
				tf.selecting = false
				tf.clearSelection()
			}
		case input.EventMove:
			if tf.focused && tf.selecting {
				tf.selEnd = tf.indexAt(ev.Pos.X)
				tf.cursor = tf.selEnd
			}
		case input.EventUp:
			tf.selecting = false
		}
	}
	tf.clampState()
	return
}

func (tf *TextField) IsFocused() bool { return tf.focused }
func (tf *TextField) Focus() {
	tf.focused = true
	tf.cursor = len([]rune(tf.Value))
	tf.clearSelection()
}
func (tf *TextField) Blur() {
	tf.focused = false
	tf.selecting = false
	tf.clearSelection()
}
func (tf *TextField) Clear() {
	tf.Value = ""
	tf.cursor = 0
	tf.clearSelection()
}

func (tf *TextField) ConsumeAction() TextEditAction {
	a := tf.pending
	tf.pending = TextEditNone
	return a
}

func (tf *TextField) SelectedText() string {
	if !tf.hasSelection() {
		return ""
	}
	runes := []rune(tf.Value)
	start, end := tf.selectionBounds()
	return string(runes[start:end])
}

func (tf *TextField) DeleteSelection() string {
	text := tf.SelectedText()
	if text != "" {
		tf.replaceSelection("")
	}
	return text
}

func (tf *TextField) InsertText(text string) {
	for _, r := range []rune(text) {
		if tf.MaxLen > 0 && len([]rune(tf.Value)) >= tf.MaxLen {
			break
		}
		tf.replaceSelection(string(r))
	}
}

func (tf *TextField) SelectAll() {
	tf.selStart = 0
	tf.selEnd = len([]rune(tf.Value))
	tf.cursor = tf.selEnd
}

// PollKeyboard reads physical keyboard input and appends it to Value.
// Call once per frame while IsFocused() is true.
func (tf *TextField) PollKeyboard() {
	if !tf.focused {
		return
	}
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	if ctrl {
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyA):
			tf.pending = TextEditSelectAll
		case inpututil.IsKeyJustPressed(ebiten.KeyC):
			tf.pending = TextEditCopy
		case inpututil.IsKeyJustPressed(ebiten.KeyX):
			tf.pending = TextEditCut
		case inpututil.IsKeyJustPressed(ebiten.KeyV):
			tf.pending = TextEditPaste
		}
		return
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if tf.MaxLen > 0 && len([]rune(tf.Value)) >= tf.MaxLen {
			break
		}
		tf.replaceSelection(string(r))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		if tf.hasSelection() {
			tf.replaceSelection("")
		} else {
			runes := []rune(tf.Value)
			if tf.cursor > 0 && len(runes) > 0 {
				runes = append(runes[:tf.cursor-1], runes[tf.cursor:]...)
				tf.cursor--
				tf.Value = string(runes)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		tf.moveCursor(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		tf.moveCursor(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		tf.cursor = 0
		tf.clearSelection()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		tf.cursor = len([]rune(tf.Value))
		tf.clearSelection()
	}
	tf.clampState()
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
		if tf.focused {
			tf.drawCursor(dst)
			tf.drawMenu(dst)
		}
		return
	}
	if tf.focused {
		tf.drawSelection(dst)
	}
	tf.textEl.Text = tf.Value
	tf.textEl.Draw(dst)
	if tf.focused {
		tf.drawCursor(dst)
		tf.drawMenu(dst)
	}
}

func (tf *TextField) hasSelection() bool { return tf.selStart != tf.selEnd }

func (tf *TextField) clearSelection() {
	tf.selStart = tf.cursor
	tf.selEnd = tf.cursor
}

func (tf *TextField) selectionBounds() (int, int) {
	a, b := tf.selStart, tf.selEnd
	if a > b {
		a, b = b, a
	}
	return a, b
}

func (tf *TextField) replaceSelection(s string) {
	runes := []rune(tf.Value)
	start, end := tf.selectionBounds()
	if !tf.hasSelection() {
		start, end = tf.cursor, tf.cursor
	}
	insert := []rune(s)
	next := append([]rune{}, runes[:start]...)
	next = append(next, insert...)
	next = append(next, runes[end:]...)
	tf.Value = string(next)
	tf.cursor = start + len(insert)
	tf.clearSelection()
}

func (tf *TextField) moveCursor(delta int) {
	tf.cursor += delta
	tf.clampState()
	tf.clearSelection()
}

func (tf *TextField) clampState() {
	n := len([]rune(tf.Value))
	if tf.cursor < 0 {
		tf.cursor = 0
	}
	if tf.cursor > n {
		tf.cursor = n
	}
	if tf.selStart < 0 {
		tf.selStart = 0
	}
	if tf.selStart > n {
		tf.selStart = n
	}
	if tf.selEnd < 0 {
		tf.selEnd = 0
	}
	if tf.selEnd > n {
		tf.selEnd = n
	}
}

func (tf *TextField) indexAt(px float64) int {
	runes := []rune(tf.Value)
	cell := tf.charW()
	idx := int((px - tf.x - textFieldPadX + cell/2) / cell)
	if idx < 0 {
		return 0
	}
	if idx > len(runes) {
		return len(runes)
	}
	return idx
}

func (tf *TextField) xForIndex(idx int) float64 {
	return tf.x + textFieldPadX + float64(idx)*tf.charW()
}

func (tf *TextField) charW() float64 { return 8.0 }

func (tf *TextField) drawCursor(dst draws.Image) {
	if tf.hasSelection() {
		return
	}
	sp := draws.NewSprite(tf.cursorImg)
	sp.Locate(tf.xForIndex(tf.cursor), tf.y+TextFieldH/2, draws.CenterMiddle)
	sp.Draw(dst)
}

func (tf *TextField) drawSelection(dst draws.Image) {
	if !tf.hasSelection() {
		return
	}
	start, end := tf.selectionBounds()
	x1 := tf.xForIndex(start)
	x2 := tf.xForIndex(end)
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	w := max(2, x2-x1)
	sp := draws.NewSprite(tf.selectImg)
	sp.Locate(x1, tf.y+7, draws.LeftTop)
	sp.Size = draws.XY{X: w, Y: TextFieldH - 14}
	sp.Draw(dst)

	for _, x := range []float64{x1, x2} {
		h := draws.NewSprite(tf.handleImg)
		h.Locate(x, tf.y+TextFieldH-5, draws.CenterMiddle)
		h.Draw(dst)
	}
}

func (tf *TextField) menuRect() (x, y, w, h float64) {
	x = tf.x
	y = tf.y - textMenuH - 6
	if y < 0 {
		y = tf.y + TextFieldH + 6
	}
	return x, y, textMenuW, textMenuH
}

func (tf *TextField) handleMenuDown(pos draws.XY) bool {
	x, y, w, h := tf.menuRect()
	if pos.X < x || pos.X >= x+w || pos.Y < y || pos.Y >= y+h {
		return false
	}
	idx := int((pos.X - x) / (w / 4))
	switch idx {
	case 0:
		tf.pending = TextEditCopy
	case 1:
		tf.pending = TextEditCut
	case 2:
		tf.pending = TextEditPaste
	default:
		tf.pending = TextEditSelectAll
	}
	return true
}

func (tf *TextField) drawMenu(dst draws.Image) {
	x, y, w, h := tf.menuRect()
	sp := draws.NewSprite(tf.menuBg)
	sp.Locate(x, y, draws.LeftTop)
	sp.Draw(dst)

	labels := []string{"Copy", "Cut", "Paste", "All"}
	opts := draws.NewFaceOptions()
	opts.Size = 11
	cellW := w / float64(len(labels))
	for i, label := range labels {
		t := draws.NewText(label)
		t.SetFace(opts)
		t.Locate(x+cellW*(float64(i)+0.5), y+h/2, draws.CenterMiddle)
		if (label == "Copy" || label == "Cut") && !tf.hasSelection() {
			t.ColorScale.Scale(1, 1, 1, 0.42)
		}
		t.Draw(dst)
	}
}
