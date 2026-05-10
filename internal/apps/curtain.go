package apps

import (
	"image/color"
	"time"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/tween"
	"github.com/hndada/mos/ui"
)

const curtainDuration = 260 * time.Millisecond

// curtainTile is one quick-settings square. Its TriggerButton owns the
// hit region (in panel-local coords); visuals are repainted each frame
// in Draw using the tile's current on/off state.
type curtainTile struct {
	label     string
	on        bool
	accent    color.RGBA
	btn       ui.TriggerButton
	labelText draws.Text
	stateText draws.Text
	publishFn func(c *DefaultCurtain, on bool)
}

type DefaultCurtain struct {
	screenW float64
	screenH float64
	panelH  float64
	bus     *event.Bus

	overlay draws.Sprite
	panel   draws.Sprite
	title   draws.Text
	date    draws.Text
	time    draws.Text
	tiles   []*curtainTile
	grabber draws.Sprite

	// Reusable per-tile background sprites; recoloured by Tile state at draw time.
	tileBgOff draws.Image
	tileBgOn  draws.Image

	// Notification list: notices posted via Context.PostNotice, newest first.
	notices         []noticeItem
	noticesTop      float64     // Y where the notice stack begins (panel-local)
	noticeAreaW     float64     // visible width for notice cards
	noticeMargin    float64     // left margin / horizontal padding
	noticeCardH     float64     // height of one notice card
	noticeCardGap   float64     // vertical gap between notice cards
	noticeCardBg    draws.Image // shared rounded-rect-ish card background
	noticeTitleText draws.Text  // reused per-frame
	noticeBodyText  draws.Text  // reused per-frame
	noticeTimeText  draws.Text  // reused per-frame

	shown bool
	y     tween.Tween

	// bgBlur is a blurred snapshot of the scene behind the panel.
	// Set by the windowing server via SetBackground each frame before Draw.
	bgBlur draws.Image
}

// noticeItem is the in-curtain projection of mosapp.Notice. The receive
// timestamp is captured at AddNotice time so the rendering can show
// "now / 14:32" style hints.
type noticeItem struct {
	title string
	body  string
	when  time.Time
}

const maxCurtainNotices = 8

func NewDefaultCurtain(screenW, screenH float64, bus *event.Bus) *DefaultCurtain {
	vp := draws.NewViewport(screenW, screenH)
	panelH := screenH * 0.68
	overlayImg := draws.CreateImage(screenW, screenH)
	overlayImg.Fill(color.RGBA{0, 0, 0, 118})
	overlay := draws.NewSprite(overlayImg)
	overlay.Locate(0, 0, draws.LeftTop)

	// Semi-transparent so the blurred background shows through (frosted glass).
	// When no blur is supplied the panel falls back to this solid-ish colour.
	panelImg := roundedRectImage(screenW, panelH+vp.Y(0.04), vp.U(0.075), color.RGBA{18, 21, 29, 226})
	panel := draws.NewSprite(panelImg)

	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 24
	title := draws.NewText("Control Center")
	title.SetFace(titleOpts)
	title.Locate(vp.X(0.055), vp.Y(0.038), draws.LeftTop)

	dateOpts := draws.NewFaceOptions()
	dateOpts.Size = 12
	date := draws.NewText("")
	date.SetFace(dateOpts)
	date.ColorScale.Scale(1, 1, 1, 0.62)
	date.Locate(vp.X(0.055), vp.Y(0.078), draws.LeftTop)

	timeOpts := draws.NewFaceOptions()
	timeOpts.Size = 18
	clock := draws.NewText("")
	clock.SetFace(timeOpts)
	clock.Locate(screenW-vp.X(0.055), vp.Y(0.045), draws.RightTop)

	grabberImg := roundedRectImage(vp.X(0.18), vp.Y(0.006), vp.Y(0.003), color.RGBA{255, 255, 255, 95})
	grabber := draws.NewSprite(grabberImg)
	grabber.Locate(screenW/2, vp.Y(0.016), draws.CenterMiddle)

	c := &DefaultCurtain{
		screenW: screenW,
		screenH: screenH,
		panelH:  panelH,
		bus:     bus,
		overlay: overlay,
		panel:   panel,
		title:   title,
		date:    date,
		time:    clock,
		grabber: grabber,
	}

	// Tile background images (one off-state, one on-state). Reused for every tile.
	const cols = 3
	tilePadX := vp.X(0.055)
	tilePadTop := vp.Y(0.125)
	tileGap := vp.X(0.028)
	tileW := (screenW - tilePadX*2 - tileGap*float64(cols-1)) / float64(cols)
	tileH := tileW * 0.76

	c.tileBgOff = roundedRectImage(tileW, tileH, vp.U(0.038), color.RGBA{48, 53, 65, 218})

	c.tileBgOn = roundedRectImage(tileW, tileH, vp.U(0.038), color.RGBA{50, 125, 238, 238})

	specs := []struct {
		label   string
		on      bool
		accent  color.RGBA
		publish func(c *DefaultCurtain, on bool)
	}{
		{"Wi-Fi", true, color.RGBA{52, 132, 220, 255}, nil},
		{"Bluetooth", false, color.RGBA{88, 86, 214, 255}, nil},
		{"Airplane", false, color.RGBA{255, 159, 64, 255}, nil},
		{"AOD", true, color.RGBA{255, 214, 80, 255}, func(c *DefaultCurtain, on bool) {
			if c.bus != nil {
				c.bus.Publish(event.System{Topic: event.TopicAOD, Value: on})
			}
		}},
		{"Dark Mode", false, color.RGBA{120, 86, 255, 255}, func(c *DefaultCurtain, on bool) {
			if c.bus != nil {
				c.bus.Publish(event.System{Topic: event.TopicDarkMode, Value: on})
			}
		}},
		{"Focus", false, color.RGBA{255, 149, 0, 255}, nil},
		{"DND", false, color.RGBA{170, 90, 210, 255}, nil},
		{"Rotate", true, color.RGBA{90, 200, 250, 255}, nil},
		{"Battery", false, color.RGBA{52, 199, 89, 255}, nil},
		{"Clear", false, color.RGBA{110, 118, 130, 255}, func(c *DefaultCurtain, on bool) {
			c.notices = nil
			c.setTileByLabel("Clear", false)
			c.feedback(mosapp.SoundSuccess, 10*time.Millisecond)
		}},
	}

	labelOpts := draws.NewFaceOptions()
	labelOpts.Size = 14
	stateOpts := draws.NewFaceOptions()
	stateOpts.Size = 11
	for i, sp := range specs {
		col := i % cols
		row := i / cols
		x := tilePadX + float64(col)*(tileW+tileGap)
		y := tilePadTop + float64(row)*(tileH+tileGap)

		t := &curtainTile{
			label:     sp.label,
			on:        sp.on,
			accent:    sp.accent,
			btn:       ui.NewTriggerButton(x, y, tileW, tileH),
			publishFn: sp.publish,
		}

		t.labelText = draws.NewText(sp.label)
		t.labelText.SetFace(labelOpts)
		t.labelText.Locate(x+tileW*0.12, y+tileH*0.56, draws.LeftMiddle)

		t.stateText = draws.NewText("")
		t.stateText.SetFace(stateOpts)
		t.stateText.ColorScale.Scale(1, 1, 1, 0.62)
		t.stateText.Locate(x+tileW*0.12, y+tileH*0.77, draws.LeftMiddle)

		c.tiles = append(c.tiles, t)
	}

	// Notification area: stacked card list below the tile rows.
	tileRows := (len(specs) + cols - 1) / cols
	c.noticeMargin = tilePadX
	c.noticeAreaW = screenW - tilePadX*2
	c.noticeCardH = vp.Y(0.077)
	c.noticeCardGap = vp.Y(0.010)
	c.noticesTop = tilePadTop + float64(tileRows)*(tileH+tileGap) + vp.Y(0.082)

	c.noticeCardBg = roundedRectImage(c.noticeAreaW, c.noticeCardH, vp.U(0.032), color.RGBA{32, 37, 49, 225})

	titleOpts2 := draws.NewFaceOptions()
	titleOpts2.Size = 13
	c.noticeTitleText = draws.NewText("")
	c.noticeTitleText.SetFace(titleOpts2)

	bodyOpts := draws.NewFaceOptions()
	bodyOpts.Size = 11
	c.noticeBodyText = draws.NewText("")
	c.noticeBodyText.SetFace(bodyOpts)
	c.noticeBodyText.ColorScale.Scale(1, 1, 1, 0.68)

	timeOpts2 := draws.NewFaceOptions()
	timeOpts2.Size = 10
	c.noticeTimeText = draws.NewText("")
	c.noticeTimeText.SetFace(timeOpts2)
	c.noticeTimeText.ColorScale.Scale(1, 1, 1, 0.55)

	c.y = curtainAnim(-panelH, -panelH, panelH)
	return c
}

// AddNotice prepends a posted notice to the curtain list, capped at
// maxCurtainNotices so the panel never overflows.
func (s *DefaultCurtain) AddNotice(n mosapp.Notice) {
	item := noticeItem{title: n.Title, body: n.Body, when: time.Now()}
	s.notices = append([]noticeItem{item}, s.notices...)
	if len(s.notices) > maxCurtainNotices {
		s.notices = s.notices[:maxCurtainNotices]
	}
	s.feedback(mosapp.SoundNotification, 18*time.Millisecond)
}

func curtainAnim(from, to, panelH float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	delta := to - from
	if delta < 0 {
		delta = -delta
	}
	d := time.Duration(float64(curtainDuration) * delta / panelH)
	if d < 16*time.Millisecond {
		d = 16 * time.Millisecond
	}
	tw.Add(from, to-from, d, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func (s *DefaultCurtain) Show() {
	if s.shown {
		return
	}
	s.shown = true
	s.y = curtainAnim(s.y.Value(), 0, s.panelH)
	s.feedback(mosapp.SoundTap, 10*time.Millisecond)
}

func (s *DefaultCurtain) Hide() {
	if !s.shown {
		return
	}
	s.shown = false
	s.y = curtainAnim(s.y.Value(), -s.panelH, s.panelH)
	s.feedback(mosapp.SoundTap, 8*time.Millisecond)
}

func (s *DefaultCurtain) Toggle() {
	if s.shown {
		s.Hide()
		return
	}
	s.Show()
}

func (s *DefaultCurtain) IsVisible() bool {
	return s.shown || s.y.Value() > -s.panelH
}

// SetBackground receives the pre-blurred scene snapshot from the windowing
// server. It is stored and composited as a frosted-glass backdrop behind the
// panel content in the next Draw call.
func (s *DefaultCurtain) SetBackground(bg draws.Image) {
	s.bgBlur = bg
}

// SubscribeBus wires curtain tiles to system events so external state
// changes (e.g. Settings toggles Dark Mode) keep the tile in sync.
// Call after construction; safe to call multiple times — duplicate
// subscriptions just lead to redundant updates, not errors.
func (s *DefaultCurtain) SubscribeBus() {
	if s.bus == nil {
		return
	}
	s.bus.Subscribe(event.KindSystem, func(e event.Event) {
		se, ok := e.(event.System)
		if !ok {
			return
		}
		switch se.Topic {
		case event.TopicDarkMode:
			if v, ok := se.Value.(bool); ok {
				s.setTileByLabel("Dark Mode", v)
			}
		case event.TopicAOD:
			if v, ok := se.Value.(bool); ok {
				s.setTileByLabel("AOD", v)
			}
		}
	})
}

func (s *DefaultCurtain) setTileByLabel(label string, on bool) {
	for _, t := range s.tiles {
		if t.label == label {
			t.on = on
			return
		}
	}
}

func (s *DefaultCurtain) Update(frame mosapp.Frame) {
	s.y.Update()
	s.time.Text = time.Now().Format("15:04")
	s.date.Text = time.Now().Format("Mon, Jan 2")

	// Don't process input until the panel has any pixels on screen, and
	// don't intercept after a Hide while the panel is sliding off.
	if !s.shown {
		return
	}

	// Translate the frame into panel-local coords so tile TriggerButtons
	// (whose hit boxes are stored in panel coords) work correctly during
	// the open/close slide.
	py := s.y.Value()
	panelFrame := mosapp.Frame{
		Cursor: draws.XY{X: frame.Cursor.X, Y: frame.Cursor.Y - py},
	}
	if len(frame.Events) > 0 {
		panelFrame.Events = make([]input.Event, len(frame.Events))
		for i, ev := range frame.Events {
			ev.Pos.Y -= py
			panelFrame.Events[i] = ev
		}
	}

	for _, t := range s.tiles {
		if t.btn.Update(panelFrame) {
			t.on = !t.on
			if t.on {
				s.feedback(mosapp.SoundToggleOn, 16*time.Millisecond)
			} else {
				s.feedback(mosapp.SoundToggleOff, 12*time.Millisecond)
			}
			if t.publishFn != nil {
				t.publishFn(s, t.on)
			}
		}
	}
}

func (s *DefaultCurtain) feedback(sound mosapp.Sound, duration time.Duration) {
	if s.bus != nil {
		s.bus.Publish(event.Custom{
			Topic: "mos/system/feedback",
			Data: map[string]any{
				"sound":     string(sound),
				"vibration": duration,
			},
		})
	}
}

func (s *DefaultCurtain) Draw(dst draws.Image) {
	if !s.IsVisible() {
		return
	}
	y := s.y.Value()
	alpha := float32((y + s.panelH) / s.panelH)
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}

	overlay := s.overlay
	overlay.ColorScale.ScaleAlpha(alpha)
	overlay.Draw(dst)

	// Frosted-glass backdrop: show the blurred scene snapshot clipped to the
	// visible portion of the panel, then draw the semi-transparent tint on top.
	if !s.bgBlur.IsEmpty() {
		screenW := int(s.bgBlur.Size().X)
		// Compute the visible panel region in screen space.
		visTop := int(y)
		if visTop < 0 {
			visTop = 0
		}
		visBottom := int(y + s.panelH)
		if visBottom > int(dst.Size().Y) {
			visBottom = int(dst.Size().Y)
		}
		if visBottom > visTop {
			sub := s.bgBlur.SubImage(0, visTop, screenW, visBottom)
			subSp := draws.NewSprite(sub)
			subSp.Locate(0, float64(visTop), draws.LeftTop)
			subSp.ColorScale.ScaleAlpha(alpha)
			subSp.Draw(dst)
		}
	}

	panel := s.panel
	panel.Locate(0, y-draws.NewViewport(s.screenW, s.screenH).Y(0.04), draws.LeftTop)
	panel.ColorScale.ScaleAlpha(alpha)
	panel.Draw(dst)

	vp := draws.NewViewport(s.screenW, s.screenH)
	s.grabber.Position.Y = y + vp.Y(0.016)
	s.grabber.ColorScale.ScaleAlpha(alpha)
	s.grabber.Draw(dst)

	s.title.Position.Y = y + vp.Y(0.038)
	s.date.Position.Y = y + vp.Y(0.078)
	s.time.Position.Y = y + vp.Y(0.045)
	s.title.Draw(dst)
	s.date.Draw(dst)
	s.time.Draw(dst)

	for _, t := range s.tiles {
		s.drawTile(dst, t, y)
	}

	s.drawSliders(dst, y, alpha)
	s.drawNotices(dst, y, alpha)
}

func (s *DefaultCurtain) drawSliders(dst draws.Image, panelY float64, alpha float32) {
	vp := draws.NewViewport(s.screenW, s.screenH)
	x := vp.X(0.055)
	w := s.screenW - x*2
	h := vp.Y(0.024)
	y := panelY + s.noticesTop - vp.Y(0.052)
	s.drawSlider(dst, x, y, w, h, 0.72, color.RGBA{255, 214, 80, 240}, alpha)
	s.drawSlider(dst, x, y+vp.Y(0.032), w, h, 0.48, color.RGBA{90, 200, 250, 240}, alpha)
}

func (s *DefaultCurtain) drawSlider(dst draws.Image, x, y, w, h, value float64, accent color.RGBA, alpha float32) {
	vp := draws.NewViewport(s.screenW, s.screenH)
	track := draws.NewSprite(roundedRectImage(w, h, h/2, color.RGBA{255, 255, 255, 36}))
	track.Locate(x, y, draws.LeftTop)
	track.ColorScale.ScaleAlpha(alpha)
	track.Draw(dst)

	fill := draws.NewSprite(roundedRectImage(w*value, h, h/2, accent))
	fill.Locate(x, y, draws.LeftTop)
	fill.ColorScale.ScaleAlpha(alpha)
	fill.Draw(dst)

	knob := draws.NewSprite(roundedRectImage(vp.U(0.038), vp.U(0.038), vp.U(0.019), color.RGBA{255, 255, 255, 235}))
	knob.Locate(x+w*value, y+h/2, draws.CenterMiddle)
	knob.ColorScale.ScaleAlpha(alpha)
	knob.Draw(dst)
}

// drawNotices renders only the notification cards visible in the panel.
func (s *DefaultCurtain) drawNotices(dst draws.Image, panelY float64, alpha float32) {
	if len(s.notices) == 0 {
		return
	}
	step := s.noticeCardH + s.noticeCardGap
	viewportH := s.panelH - s.noticesTop - 12
	start, end := ui.VisibleRange(0, viewportH, step, len(s.notices), 0)
	for i := start; i < end; i++ {
		n := s.notices[i]
		cardTop := panelY + s.noticesTop + float64(i)*step

		bg := draws.NewSprite(s.noticeCardBg)
		bg.Locate(s.noticeMargin, cardTop, draws.LeftTop)
		bg.ColorScale.ScaleAlpha(alpha)
		bg.Draw(dst)

		s.noticeTitleText.Text = n.title
		vp := draws.NewViewport(s.screenW, s.screenH)
		s.noticeTitleText.Locate(s.noticeMargin+vp.X(0.035), cardTop+vp.Y(0.010), draws.LeftTop)
		s.noticeTitleText.Draw(dst)

		s.noticeBodyText.Text = n.body
		s.noticeBodyText.Locate(s.noticeMargin+vp.X(0.035), cardTop+vp.Y(0.034), draws.LeftTop)
		s.noticeBodyText.Draw(dst)

		s.noticeTimeText.Text = n.when.Format("15:04")
		s.noticeTimeText.Locate(s.noticeMargin+s.noticeAreaW-vp.X(0.035), cardTop+vp.Y(0.012), draws.RightTop)
		s.noticeTimeText.Draw(dst)
	}
}

func (s *DefaultCurtain) drawTile(dst draws.Image, t *curtainTile, panelY float64) {
	vp := draws.NewViewport(s.screenW, s.screenH)
	bgImg := s.tileBgOff
	if t.on {
		bgImg = s.tileBgOn
	}
	sp := draws.NewSprite(bgImg)
	sp.Locate(t.btn.X(), t.btn.Y()+panelY, draws.LeftTop)
	sp.Draw(dst)

	iconSize := vp.U(0.072)
	icon := roundedRectImage(iconSize, iconSize, iconSize/2, color.RGBA{255, 255, 255, 44})
	if t.on {
		icon = roundedRectImage(iconSize, iconSize, iconSize/2, t.accent)
	}
	iconSp := draws.NewSprite(icon)
	iconSp.Locate(t.btn.X()+t.btn.W()*0.18, t.btn.Y()+t.btn.H()*0.26+panelY, draws.CenterMiddle)
	iconSp.Draw(dst)

	glyph := draws.NewText(tileGlyph(t.label))
	glyphOpts := draws.NewFaceOptions()
	glyphOpts.Size = 13
	glyph.SetFace(glyphOpts)
	glyph.Locate(t.btn.X()+t.btn.W()*0.18, t.btn.Y()+t.btn.H()*0.26+panelY, draws.CenterMiddle)
	glyph.Draw(dst)

	t.labelText.Position.Y = (t.btn.Y() + t.btn.H()*0.56) + panelY
	t.labelText.Draw(dst)

	if t.on {
		t.stateText.Text = "On"
	} else {
		t.stateText.Text = "Off"
	}
	t.stateText.Position.Y = (t.btn.Y() + t.btn.H()*0.77) + panelY
	t.stateText.Draw(dst)
}

func tileGlyph(label string) string {
	switch label {
	case "Wi-Fi":
		return "W"
	case "Bluetooth":
		return "B"
	case "AOD":
		return "A"
	case "Dark Mode":
		return "D"
	case "Focus":
		return "F"
	case "Battery":
		return "%"
	case "Airplane":
		return "P"
	case "DND":
		return "!"
	case "Rotate":
		return "R"
	case "Clear":
		return "X"
	default:
		return label[:1]
	}
}
