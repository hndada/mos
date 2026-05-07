package ui

import (
	"image/color"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui/theme"
)

const (
	checkboxSize    = 22.0
	checkboxLabelGap = 10.0
	checkboxFontSize = 14.0

	radioSize    = 20.0
	radioGap     = 10.0
	radioItemH   = 44.0
	radioFontSize = 14.0
)

// ── Checkbox ──────────────────────────────────────────────────────────────────

// Checkbox is a boolean selector with a visible label.
// Update returns true when the value changes.
type Checkbox struct {
	Value bool
	Label string

	gesture GestureDetector

	boxOff draws.Image // unchecked
	boxOn  draws.Image // checked (filled + tick mark)
	labelEl draws.Text

	x, y float64
}

func NewCheckbox(x, y float64, label string, val bool) Checkbox {
	// Colours are sampled from the active theme at construction time. Rebuild
	// the widget (or call NewCheckbox again) after a theme switch to pick up
	// the new palette.
	off := draws.CreateImage(checkboxSize, checkboxSize)
	off.Fill(theme.Active().Color(theme.SurfaceWidget))

	on := draws.CreateImage(checkboxSize, checkboxSize)
	on.Fill(theme.Active().Color(theme.Accent))
	// Draw a simple tick "✓" as a text element baked into the image.
	tick := draws.NewText("✓")
	tickOpts := draws.NewFaceOptions()
	tickOpts.Size = 13
	tick.SetFace(tickOpts)
	tick.Locate(checkboxSize/2, checkboxSize/2, draws.CenterMiddle)
	tick.Draw(on)

	labelOpts := draws.NewFaceOptions()
	labelOpts.Size = checkboxFontSize
	labelEl := draws.NewText(label)
	labelEl.SetFace(labelOpts)
	labelEl.Locate(x+checkboxSize+checkboxLabelGap, y+checkboxSize/2, draws.LeftMiddle)

	hitW := checkboxSize + checkboxLabelGap + labelEl.Size().X + 4
	return Checkbox{
		Value:   val,
		Label:   label,
		gesture: NewGestureDetector(x, y, hitW, checkboxSize),
		boxOff:  off,
		boxOn:   on,
		labelEl: labelEl,
		x:       x,
		y:       y,
	}
}

// Update flips Value on tap and returns true when changed.
func (c *Checkbox) Update(frame mosapp.Frame) bool {
	if c.gesture.Update(frame).Kind == GestureTap {
		c.Value = !c.Value
		return true
	}
	return false
}

func (c Checkbox) Draw(dst draws.Image) {
	box := c.boxOff
	if c.Value {
		box = c.boxOn
	}
	sp := draws.NewSprite(box)
	sp.Locate(c.x, c.y, draws.LeftTop)
	sp.Draw(dst)

	c.labelEl.Text = c.Label
	c.labelEl.Draw(dst)
}

// ── RadioGroup ────────────────────────────────────────────────────────────────

// RadioGroup is a vertical list of mutually exclusive options.
// Selected holds the index of the active option.
// Update returns true when the selection changes.
type RadioGroup struct {
	Options  []string
	Selected int // index into Options

	items []radioItem
}

type radioItem struct {
	gesture GestureDetector
	labelEl draws.Text
	dotOff  draws.Image // outer ring
	dotOn   draws.Image // filled circle
	x, y    float64
}

// NewRadioGroup builds a vertical stack of radio options starting at (x,y).
// Each row is radioItemH pixels tall.
func NewRadioGroup(x, y float64, options []string, selected int) RadioGroup {
	items := make([]radioItem, len(options))
	for i, opt := range options {
		iy := y + float64(i)*radioItemH

		// Outer ring (off state). Colours come from the active theme at
		// construction time; rebuild after a theme switch to update.
		off := draws.CreateImage(radioSize, radioSize)
		off.Fill(theme.Active().Color(theme.SurfaceWidget))
		inner := draws.CreateImage(radioSize-4, radioSize-4)
		inner.Fill(theme.Active().Color(theme.Background)) // hollow centre
		innerSp := draws.NewSprite(inner)
		innerSp.Locate(radioSize/2, radioSize/2, draws.CenterMiddle)
		innerSp.Draw(off)

		// Filled (on state).
		on := draws.CreateImage(radioSize, radioSize)
		on.Fill(theme.Active().Color(theme.Accent))
		dot := draws.CreateImage(radioSize-8, radioSize-8)
		dot.Fill(color.White)
		dotSp := draws.NewSprite(dot)
		dotSp.Locate(radioSize/2, radioSize/2, draws.CenterMiddle)
		dotSp.Draw(on)

		labelOpts := draws.NewFaceOptions()
		labelOpts.Size = radioFontSize
		labelEl := draws.NewText(opt)
		labelEl.SetFace(labelOpts)
		labelEl.Locate(x+radioSize+radioGap, iy+radioItemH/2, draws.LeftMiddle)

		items[i] = radioItem{
			gesture: NewGestureDetector(x, iy, 260, radioItemH),
			labelEl: labelEl,
			dotOff:  off,
			dotOn:   on,
			x:       x,
			y:       iy,
		}
	}
	return RadioGroup{Options: options, Selected: selected, items: items}
}

// Update returns true when the selection changes.
func (r *RadioGroup) Update(frame mosapp.Frame) bool {
	for i := range r.items {
		if r.items[i].gesture.Update(frame).Kind == GestureTap {
			if r.Selected != i {
				r.Selected = i
				return true
			}
		}
	}
	return false
}

func (r RadioGroup) Draw(dst draws.Image) {
	for i, item := range r.items {
		dot := item.dotOff
		if i == r.Selected {
			dot = item.dotOn
		}
		sp := draws.NewSprite(dot)
		sp.Locate(item.x, item.y+radioItemH/2-radioSize/2, draws.LeftTop)
		sp.Draw(dst)

		item.labelEl.Draw(dst)
	}
}
