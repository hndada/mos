package ui

import (
	"image/color"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
)

// ControlPanel sizing constants. Exported so callers can reason about the
// panel footprint when laying out surrounding chrome.
const (
	ControlPanelButtonW = 68.0
	ControlPanelButtonH = 22.0
	ControlPanelGap     = 4.0
	ControlPanelPad     = 8.0
	controlPanelFont    = 10.0
)

var (
	controlPanelBgColor  = color.RGBA{0, 0, 0, 150}
	controlPanelButtonBg = color.RGBA{255, 255, 255, 48}
	controlPanelText     = color.RGBA{18, 18, 18, 255}
)

// ControlAction is one entry in a ControlPanel: the label shown on the
// button and the function invoked when it is tapped. Handler may be nil.
type ControlAction struct {
	Label   string
	Handler func()
}

// ControlPanel is a grid of labelled action buttons backed by a translucent
// panel background. It owns layout, hit-testing, and rendering — callers
// only supply the list of actions and where to position the panel.
//
// Use cases include developer / simulator chrome and quick-action shelves
// inside apps. The panel is anchored at its top-left and grows downward
// and rightward; call Size() to read the resulting footprint.
type ControlPanel struct {
	bg      draws.Sprite
	entries []controlPanelEntry
}

type controlPanelEntry struct {
	btn     Button
	handler func()
}

// NewControlPanel builds a panel anchored at (x, y) with the given column
// count. Buttons are laid out left-to-right, top-to-bottom in the order
// they appear in actions. cols < 1 is treated as 1.
func NewControlPanel(x, y float64, cols int, actions []ControlAction) ControlPanel {
	if cols < 1 {
		cols = 1
	}
	rows := (len(actions) + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}

	panelW := ControlPanelButtonW*float64(cols) + ControlPanelGap*float64(cols-1) + ControlPanelPad*2
	panelH := ControlPanelButtonH*float64(rows) + ControlPanelGap*float64(rows-1) + ControlPanelPad*2

	bgImg := draws.CreateImage(panelW, panelH)
	bgImg.Fill(controlPanelBgColor)
	bg := draws.NewSprite(bgImg)
	bg.Locate(x, y, draws.LeftTop)

	entries := make([]controlPanelEntry, len(actions))
	for i, a := range actions {
		col := i % cols
		row := i / cols
		bx := x + ControlPanelPad + float64(col)*(ControlPanelButtonW+ControlPanelGap)
		by := y + ControlPanelPad + float64(row)*(ControlPanelButtonH+ControlPanelGap)
		btn := NewButton(a.Label, controlPanelFont, bx, by,
			ControlPanelButtonW, ControlPanelButtonH, controlPanelButtonBg)
		btn.SetLabelColor(controlPanelText)
		entries[i] = controlPanelEntry{btn: btn, handler: a.Handler}
	}

	return ControlPanel{bg: bg, entries: entries}
}

// Update polls each button and invokes the matching handler on tap.
func (p *ControlPanel) Update(frame mosapp.Frame) {
	for i := range p.entries {
		if p.entries[i].btn.Update(frame) && p.entries[i].handler != nil {
			p.entries[i].handler()
		}
	}
}

// Size returns the panel's total dimensions for caller-side layout.
func (p ControlPanel) Size() draws.XY { return p.bg.Size }

// Draw renders the panel background and every button.
func (p *ControlPanel) Draw(dst draws.Image) {
	p.bg.Draw(dst)
	for i := range p.entries {
		p.entries[i].btn.Draw(dst)
	}
}
