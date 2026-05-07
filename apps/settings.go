package apps

import (
	"image/color"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/ui"
)

// Layout constants (all in content-space pixels).
const (
	settingsStatusH = 24.0 // must match statusBarHeight
	settingsTitleH  = 48.0
	settingsRowH    = 52.0
	settingsHdrH    = 36.0
	settingsPad     = 20.0
)

// Colors.
var (
	settingsBg     = color.RGBA{18, 18, 23, 255}
	settingsHdrBg  = color.RGBA{10, 10, 14, 255}
	settingsRowBg  = color.RGBA{28, 28, 35, 255}
	settingsSepClr = color.RGBA{50, 50, 62, 255}
	settingsGray   = color.RGBA{160, 160, 175, 255}
)

type settingsKind int

const (
	kindHeader settingsKind = iota
	kindToggle
	kindSlider
	kindNav
)

type settingsRow struct {
	kind       settingsKind
	label      string
	detail     string // nav rows only
	toggle     *ui.Toggle
	slider     *ui.Slider
	y          float64 // top of row in content-space
	labelText  draws.Text
	detailText draws.Text
}

// Settings is an app content that renders a scrollable settings list.
// Content rows are rendered lazily: only rows that intersect the visible
// viewport are drawn each frame (O(visible rows), not O(all rows)).
type Settings struct {
	ctx      mosapp.Context
	rows     []*settingsRow
	screenW  float64
	screenH  float64
	contentH float64

	// vl manages the scroll offset and viewport-sized canvas.
	// It replaces the old scrollOff/dragging/canvas fields.
	vl ui.VirtualList

	titleBg  draws.Sprite
	title    draws.Text
	headerBg draws.Sprite
	rowBg    draws.Sprite
	sep      draws.Sprite
}

// OnCreate stores the context so toggle rows can publish system events.
func (s *Settings) OnCreate(ctx mosapp.Context) { s.ctx = ctx }
func (s *Settings) OnResume()                   {}
func (s *Settings) OnPause()                    {}
func (s *Settings) OnDestroy()                  {}

func NewSettings(screenW, screenH float64) *Settings {
	s := &Settings{screenW: screenW, screenH: screenH}

	type spec struct {
		kind   settingsKind
		label  string
		detail string
		val    float64
	}
	specs := []spec{
		{kindHeader, "Network", "", 0},
		{kindToggle, "Wi-Fi", "", 1},
		{kindToggle, "Bluetooth", "", 0},
		{kindToggle, "Mobile Data", "", 1},
		{kindHeader, "Sound", "", 0},
		{kindSlider, "Volume", "", 0.7},
		{kindToggle, "Vibration", "", 1},
		{kindHeader, "Display", "", 0},
		{kindSlider, "Brightness", "", 0.5},
		{kindToggle, "Dark Mode", "", 1},
		{kindHeader, "General", "", 0},
		{kindNav, "Language", "English", 0},
		{kindNav, "Storage", "32 GB", 0},
		{kindNav, "About", "", 0},
	}

	y := 0.0
	for _, sp := range specs {
		r := &settingsRow{kind: sp.kind, label: sp.label, detail: sp.detail, y: y}
		h := settingsRowH
		if sp.kind == kindHeader {
			h = settingsHdrH
		}
		if sp.kind == kindToggle {
			tx := screenW - settingsPad - ui.ToggleW
			ty := y + (settingsRowH-ui.ToggleH)/2
			tog := ui.NewToggle(tx, ty, sp.val > 0)
			r.toggle = &tog
		}
		if sp.kind == kindSlider {
			sx := settingsPad + screenW*0.38
			sy := y + settingsRowH/2
			sw := screenW - sx - settingsPad
			sl := ui.NewSlider(sx, sy, sw, sp.val)
			r.slider = &sl
		}
		s.rows = append(s.rows, r)
		y += h
	}
	s.contentH = y

	viewportY := settingsStatusH + settingsTitleH
	viewportH := screenH - viewportY
	s.vl = ui.VirtualList{
		X:      0,
		Y:      viewportY,
		W:      screenW,
		H:      viewportH,
		TotalH: s.contentH,
	}

	s.initAssets()
	return s
}

func (s *Settings) initAssets() {
	titleBg := draws.CreateImage(s.screenW, settingsTitleH)
	titleBg.Fill(color.RGBA{22, 22, 28, 255})
	s.titleBg = draws.NewSprite(titleBg)
	s.titleBg.Locate(0, settingsStatusH, draws.LeftTop)

	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 18
	s.title = draws.NewText("Settings")
	s.title.SetFace(titleOpts)
	s.title.Locate(settingsPad, settingsStatusH+settingsTitleH/2, draws.LeftMiddle)

	headerBg := draws.CreateImage(s.screenW, settingsHdrH)
	headerBg.Fill(settingsHdrBg)
	s.headerBg = draws.NewSprite(headerBg)

	rowBg := draws.CreateImage(s.screenW, settingsRowH)
	rowBg.Fill(settingsRowBg)
	s.rowBg = draws.NewSprite(rowBg)

	sep := draws.CreateImage(s.screenW-settingsPad, 1)
	sep.Fill(settingsSepClr)
	s.sep = draws.NewSprite(sep)

	labelOpts := draws.NewFaceOptions()
	labelOpts.Size = 15
	detailOpts := draws.NewFaceOptions()
	detailOpts.Size = 13
	for _, r := range s.rows {
		r.labelText = draws.NewText(r.label)
		r.labelText.SetFace(labelOpts)
		if r.kind == kindHeader {
			r.labelText.Locate(settingsPad, r.y+settingsHdrH/2, draws.LeftMiddle)
			continue
		}
		r.labelText.Locate(settingsPad, r.y+settingsRowH/2, draws.LeftMiddle)
		if r.kind == kindNav {
			detail := r.detail + "  >"
			if r.detail == "" {
				detail = ">"
			}
			r.detailText = draws.NewText(detail)
			r.detailText.SetFace(detailOpts)
			r.detailText.Locate(s.screenW-settingsPad, r.y+settingsRowH/2, draws.RightMiddle)
		}
	}
}

// contentCursor converts a screen-space position into content-space by
// undoing the title offset and adding the current scroll offset.
func (s *Settings) contentCursor(screen draws.XY) draws.XY {
	return draws.XY{
		X: screen.X,
		Y: screen.Y - settingsStatusH - settingsTitleH + s.vl.ScrollY,
	}
}

func (s *Settings) Update(frame mosapp.Frame) {
	// VirtualList handles wheel + drag-to-scroll; no manual scroll logic needed.
	s.vl.Update(frame)

	// Build a content-space frame so child widgets (toggle, slider) receive
	// coordinates in the same space they were constructed in.
	cc := s.contentCursor(frame.Cursor)
	childFrame := mosapp.Frame{Cursor: cc}
	if len(frame.Events) > 0 {
		childFrame.Events = make([]input.Event, len(frame.Events))
		for i, ev := range frame.Events {
			ev.Pos = s.contentCursor(ev.Pos)
			childFrame.Events[i] = ev
		}
	}
	// Update all rows — even off-screen ones — to preserve gesture state
	// (e.g. a slider drag that started before the row scrolled out of view).
	for _, r := range s.rows {
		if r.toggle != nil {
			if r.toggle.Update(childFrame) {
				s.onToggleChanged(r)
			}
		}
		if r.slider != nil {
			r.slider.Update(childFrame)
		}
	}
}

// onToggleChanged maps a row label to a system event topic and broadcasts
// the toggle's new value so subscribers (Hello, the future theme system,
// etc.) can react. Unknown labels are silently ignored.
func (s *Settings) onToggleChanged(r *settingsRow) {
	if s.ctx == nil || r.toggle == nil {
		return
	}
	bus := s.ctx.Bus()
	if bus == nil {
		return
	}
	switch r.label {
	case "Dark Mode":
		bus.Publish(event.System{Topic: event.TopicDarkMode, Value: r.toggle.Value})
	}
}

func (s *Settings) Draw(dst draws.Image) {
	// Title bar — always fully visible; drawn directly to the screen.
	s.titleBg.Draw(dst)
	s.title.Draw(dst)

	// Clear the viewport canvas and paint only the visible rows.
	s.vl.Begin(settingsBg)
	for _, r := range s.rows {
		rowH := settingsRowH
		if r.kind == kindHeader {
			rowH = settingsHdrH
		}
		if !s.vl.InViewport(r.y, r.y+rowH) {
			continue // row is entirely off-screen — skip it
		}
		s.drawRow(r, s.vl.ContentToCanvas(r.y))
	}

	// Blit the viewport canvas to its screen position.
	s.vl.Draw(dst)
}

// drawRow paints one row into the VirtualList canvas.
// yc is the canvas-local Y of the row's top edge (== content Y - ScrollY).
func (s *Settings) drawRow(r *settingsRow, yc float64) {
	canvas := s.vl.Canvas()
	switch r.kind {
	case kindHeader:
		hdrSp := s.headerBg
		hdrSp.Locate(0, yc, draws.LeftTop)
		hdrSp.Draw(canvas)

		// Re-locate label to canvas-local Y (cheap value-copy operation).
		lbl := r.labelText
		lbl.Locate(settingsPad, yc+settingsHdrH/2, draws.LeftMiddle)
		lbl.Draw(canvas)

	case kindToggle, kindSlider, kindNav:
		rowSp := s.rowBg
		rowSp.Locate(0, yc, draws.LeftTop)
		rowSp.Draw(canvas)

		sepSp := s.sep
		sepSp.Locate(settingsPad, yc+settingsRowH-1, draws.LeftTop)
		sepSp.Draw(canvas)

		lbl := r.labelText
		lbl.Locate(settingsPad, yc+settingsRowH/2, draws.LeftMiddle)
		lbl.Draw(canvas)

		// DrawAt shifts all sprite Y positions by the canvas offset so that
		// widgets stored in content-space appear at the right canvas position.
		yOff := s.vl.ContentToCanvas(0) // == -ScrollY
		if r.toggle != nil {
			r.toggle.DrawAt(canvas, yOff)
		}
		if r.slider != nil {
			r.slider.DrawAt(canvas, yOff)
		}
		if r.kind == kindNav {
			det := r.detailText
			det.Locate(s.screenW-settingsPad, yc+settingsRowH/2, draws.RightMiddle)
			det.Draw(canvas)
		}
	}
}
