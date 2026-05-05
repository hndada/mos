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
	btn       ui.TriggerButton
	labelText draws.Text
	stateText draws.Text
	publishFn func(bus *event.Bus, on bool)
}

type DefaultCurtain struct {
	panelH float64
	bus    *event.Bus

	overlay draws.Sprite
	panel   draws.Sprite
	title   draws.Text
	time    draws.Text
	tiles   []*curtainTile

	// Reusable per-tile background sprites; recoloured by Tile state at draw time.
	tileBgOff draws.Image
	tileBgOn  draws.Image

	shown bool
	y     tween.Tween

	// bgBlur is a blurred snapshot of the scene behind the panel.
	// Set by the windowing server via SetBackground each frame before Draw.
	bgBlur draws.Image
}

func NewDefaultCurtain(screenW, screenH float64, bus *event.Bus) *DefaultCurtain {
	panelH := screenH * 0.62
	overlayImg := draws.CreateImage(screenW, screenH)
	overlayImg.Fill(color.RGBA{0, 0, 0, 96})
	overlay := draws.NewSprite(overlayImg)
	overlay.Locate(0, 0, draws.LeftTop)

	// Semi-transparent so the blurred background shows through (frosted glass).
	// When no blur is supplied the panel falls back to this solid-ish colour.
	panelImg := draws.CreateImage(screenW, panelH)
	panelImg.Fill(color.RGBA{22, 24, 32, 200})
	panel := draws.NewSprite(panelImg)

	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 22
	title := draws.NewText("Curtain")
	title.SetFace(titleOpts)
	title.Locate(18, 22, draws.LeftTop)

	timeOpts := draws.NewFaceOptions()
	timeOpts.Size = 14
	clock := draws.NewText("")
	clock.SetFace(timeOpts)
	clock.Locate(screenW-18, 25, draws.RightTop)

	c := &DefaultCurtain{
		panelH:  panelH,
		bus:     bus,
		overlay: overlay,
		panel:   panel,
		title:   title,
		time:    clock,
	}

	// Tile background images (one off-state, one on-state). Reused for every tile.
	const cols = 3
	const tilePadX = 18.0
	const tilePadTop = 70.0
	const tileGap = 12.0
	tileW := (screenW - tilePadX*2 - tileGap*float64(cols-1)) / float64(cols)
	tileH := tileW * 0.78

	offImg := draws.CreateImage(tileW, tileH)
	offImg.Fill(color.RGBA{56, 60, 70, 255})
	c.tileBgOff = offImg

	onImg := draws.CreateImage(tileW, tileH)
	onImg.Fill(color.RGBA{52, 132, 220, 255})
	c.tileBgOn = onImg

	specs := []struct {
		label   string
		on      bool
		publish func(bus *event.Bus, on bool)
	}{
		{"Wi-Fi", true, nil},
		{"Bluetooth", false, nil},
		{"Dark Mode", false, func(bus *event.Bus, on bool) {
			if bus != nil {
				bus.Publish(event.System{Topic: event.TopicDarkMode, Value: on})
			}
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
			btn:       ui.NewTriggerButton(x, y, tileW, tileH),
			publishFn: sp.publish,
		}

		t.labelText = draws.NewText(sp.label)
		t.labelText.SetFace(labelOpts)
		t.labelText.Locate(x+tileW/2, y+tileH/2-8, draws.CenterMiddle)

		t.stateText = draws.NewText("")
		t.stateText.SetFace(stateOpts)
		t.stateText.Locate(x+tileW/2, y+tileH/2+12, draws.CenterMiddle)

		c.tiles = append(c.tiles, t)
	}

	c.y = curtainAnim(-panelH, -panelH, panelH)
	return c
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
}

func (s *DefaultCurtain) Hide() {
	if !s.shown {
		return
	}
	s.shown = false
	s.y = curtainAnim(s.y.Value(), -s.panelH, s.panelH)
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
			if t.publishFn != nil {
				t.publishFn(s.bus, t.on)
			}
		}
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
	panel.Locate(0, y, draws.LeftTop)
	panel.ColorScale.ScaleAlpha(alpha)
	panel.Draw(dst)

	s.title.Position.Y = y + 22
	s.time.Position.Y = y + 25
	s.title.Draw(dst)
	s.time.Draw(dst)

	for _, t := range s.tiles {
		s.drawTile(dst, t, y)
	}
}

func (s *DefaultCurtain) drawTile(dst draws.Image, t *curtainTile, panelY float64) {
	bgImg := s.tileBgOff
	if t.on {
		bgImg = s.tileBgOn
	}
	sp := draws.NewSprite(bgImg)
	sp.Locate(t.btn.X(), t.btn.Y()+panelY, draws.LeftTop)
	sp.Draw(dst)

	t.labelText.Position.Y = (t.btn.Y() + t.btn.H()/2 - 8) + panelY
	t.labelText.Draw(dst)

	if t.on {
		t.stateText.Text = "On"
	} else {
		t.stateText.Text = "Off"
	}
	t.stateText.Position.Y = (t.btn.Y() + t.btn.H()/2 + 12) + panelY
	t.stateText.Draw(dst)
}
