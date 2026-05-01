package apps

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/ui"
)

// Layout constants (all in content-space pixels).
const (
	settingsStatusH = 24.0 // must match sysapps.statusBarHeight
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
type Settings struct {
	rows     []*settingsRow
	screenW  float64
	screenH  float64
	contentH float64
	// scroll state
	scrollOff float64
	dragging  bool
	dragPrevY float64
	// pre-allocated canvas for the content area
	canvas   draws.Image
	titleBg  draws.Sprite
	title    draws.Text
	headerBg draws.Sprite
	rowBg    draws.Sprite
	sep      draws.Sprite
}

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
	s.canvas = draws.CreateImage(screenW, s.contentH)
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

func (s *Settings) viewportH() float64 {
	return s.screenH - settingsStatusH - settingsTitleH
}

func (s *Settings) maxScroll() float64 {
	return max(0, s.contentH-s.viewportH())
}

func (s *Settings) scrollBy(dy float64) {
	s.scrollOff = min(max(s.scrollOff+dy, 0), s.maxScroll())
}

// contentCursor transforms a screen-space cursor into content-space.
func (s *Settings) contentCursor(screen draws.XY) draws.XY {
	return draws.XY{
		X: screen.X,
		Y: screen.Y - settingsStatusH - settingsTitleH + s.scrollOff,
	}
}

func (s *Settings) Update(cursor draws.XY) {
	// Mouse-wheel scroll.
	_, wy := input.MouseWheelPosition()
	if wy != 0 {
		s.scrollBy(-wy * 40)
	}
	// Drag scroll (vertical).
	if input.IsMouseButtonPressed(input.MouseButtonLeft) {
		_, y := input.MouseCursorPosition()
		if s.dragging {
			s.scrollBy(s.dragPrevY - y)
		}
		s.dragging = true
		s.dragPrevY = y
	} else {
		s.dragging = false
	}

	cc := s.contentCursor(cursor)
	for _, r := range s.rows {
		if r.toggle != nil {
			r.toggle.Update(cc)
		}
		if r.slider != nil {
			r.slider.Update(cc)
		}
	}
}

func (s *Settings) Draw(dst draws.Image) {
	// ── title bar ─────────────────────────────────────────────────────────────
	s.titleBg.Draw(dst)
	s.title.Draw(dst)

	// ── content rows onto canvas ──────────────────────────────────────────────
	s.canvas.Fill(settingsBg)

	for _, r := range s.rows {
		s.drawRow(r)
	}

	// ── blit visible portion to screen ───────────────────────────────────────
	clipY := int(s.scrollOff)
	clipH := int(min(s.viewportH(), s.contentH-s.scrollOff))
	if clipH <= 0 {
		return
	}
	sub := s.canvas.Image.SubImage(
		image.Rect(0, clipY, int(s.screenW), clipY+clipH),
	).(*ebiten.Image)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, settingsStatusH+settingsTitleH)
	dst.DrawImage(sub, op)
}

func (s *Settings) drawRow(r *settingsRow) {
	switch r.kind {
	case kindHeader:
		hdrSp := s.headerBg
		hdrSp.Locate(0, r.y, draws.LeftTop)
		hdrSp.Draw(s.canvas)

		r.labelText.Draw(s.canvas)

	case kindToggle, kindSlider, kindNav:
		rowSp := s.rowBg
		rowSp.Locate(0, r.y, draws.LeftTop)
		rowSp.Draw(s.canvas)

		// Separator line at bottom of row.
		sepSp := s.sep
		sepSp.Locate(settingsPad, r.y+settingsRowH-1, draws.LeftTop)
		sepSp.Draw(s.canvas)

		r.labelText.Draw(s.canvas)

		if r.toggle != nil {
			r.toggle.Draw(s.canvas)
		}
		if r.slider != nil {
			r.slider.Draw(s.canvas)
		}
		if r.kind == kindNav {
			r.detailText.Draw(s.canvas)
		}
	}
}
