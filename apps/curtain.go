package apps

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/tween"
)

const curtainDuration = 260 * time.Millisecond

type DefaultCurtain struct {
	panelH float64

	overlay draws.Sprite
	panel   draws.Sprite
	title   draws.Text
	time    draws.Text
	body    []draws.Text

	shown bool
	y     tween.Tween
}

func NewDefaultCurtain(screenW, screenH float64) *DefaultCurtain {
	panelH := screenH * 0.62
	overlayImg := draws.CreateImage(screenW, screenH)
	overlayImg.Fill(color.RGBA{0, 0, 0, 96})
	overlay := draws.NewSprite(overlayImg)
	overlay.Locate(0, 0, draws.LeftTop)

	panelImg := draws.CreateImage(screenW, panelH)
	panelImg.Fill(color.RGBA{32, 34, 40, 245})
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

	bodyOpts := draws.NewFaceOptions()
	bodyOpts.Size = 15
	lines := []string{
		"Wi-Fi    On        Bluetooth    On",
		"Brightness          72%",
		"Incoming calls and system events appear here.",
	}
	body := make([]draws.Text, len(lines))
	for i, line := range lines {
		body[i] = draws.NewText(line)
		body[i].SetFace(bodyOpts)
		body[i].Locate(18, 74+float64(i)*34, draws.LeftTop)
	}

	y := curtainAnim(-panelH, -panelH, panelH)
	return &DefaultCurtain{
		panelH:  panelH,
		overlay: overlay,
		panel:   panel,
		title:   title,
		time:    clock,
		body:    body,
		y:       y,
	}
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

func (s *DefaultCurtain) Update() {
	s.y.Update()
	s.time.Text = time.Now().Format("15:04")
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

	panel := s.panel
	panel.Locate(0, y, draws.LeftTop)
	panel.Draw(dst)

	s.title.Position.Y = y + 22
	s.time.Position.Y = y + 25
	s.title.Draw(dst)
	s.time.Draw(dst)
	for i := range s.body {
		s.body[i].Position.Y = y + 74 + float64(i)*34
		s.body[i].Draw(dst)
	}
}
