package apps

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2/vector"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/tween"
)

const sceneTransitionDuration = 520 * time.Millisecond

type SceneTest struct {
	screenW float64
	screenH float64

	scenes  []draws.Image
	sprites []draws.Sprite
	index   int
	from    int
	to      int
	anim    tween.Tween
}

func NewSceneTest(screenW, screenH float64) *SceneTest {
	scenes := []draws.Image{
		newTestScene(screenW, screenH, 1, "Overview", "Home -> detail surface", color.RGBA{22, 87, 120, 255}, color.RGBA{46, 196, 182, 255}),
		newTestScene(screenW, screenH, 2, "Details", "Shared element movement", color.RGBA{90, 56, 130, 255}, color.RGBA{255, 202, 58, 255}),
		newTestScene(screenW, screenH, 3, "Complete", "Scene transition settled", color.RGBA{38, 105, 70, 255}, color.RGBA{255, 111, 97, 255}),
	}
	sprites := make([]draws.Sprite, len(scenes))
	for i, scene := range scenes {
		sprites[i] = draws.NewSprite(scene)
	}
	anim := sceneAnim(1, 1)
	return &SceneTest{
		screenW: screenW,
		screenH: screenH,
		scenes:  scenes,
		sprites: sprites,
		to:      0,
		anim:    anim,
	}
}

func newTestScene(screenW, screenH float64, n int, title, subtitle string, bg, accent color.RGBA) draws.Image {
	img := draws.CreateImage(screenW, screenH)
	img.Fill(bg)

	top := float32(screenH * 0.18)
	vector.DrawFilledCircle(img.Image, float32(screenW*0.78), top, float32(min(screenW, screenH)*0.18), accent, true)
	vector.DrawFilledRect(img.Image, float32(screenW*0.08), float32(screenH*0.55), float32(screenW*0.84), float32(screenH*0.22), color.RGBA{255, 255, 255, 38}, true)
	vector.DrawFilledRect(img.Image, float32(screenW*0.08), float32(screenH*0.80), float32(screenW*0.38), float32(screenH*0.055), color.RGBA{255, 255, 255, 46}, true)
	vector.DrawFilledRect(img.Image, float32(screenW*0.54), float32(screenH*0.80), float32(screenW*0.38), float32(screenH*0.055), color.RGBA{255, 255, 255, 46}, true)

	numOpts := draws.NewFaceOptions()
	numOpts.Size = min(screenW, screenH) * 0.18
	num := draws.NewText(fmt.Sprintf("%d", n))
	num.SetFace(numOpts)
	num.Locate(screenW*0.16, screenH*0.16, draws.LeftTop)
	num.Draw(img)

	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = min(screenW, screenH) * 0.075
	t := draws.NewText(title)
	t.SetFace(titleOpts)
	t.Locate(screenW*0.08, screenH*0.42, draws.LeftTop)
	t.Draw(img)

	subOpts := draws.NewFaceOptions()
	subOpts.Size = min(screenW, screenH) * 0.04
	sub := draws.NewText(subtitle)
	sub.SetFace(subOpts)
	sub.Locate(screenW*0.08, screenH*0.49, draws.LeftTop)
	sub.Draw(img)

	hintOpts := draws.NewFaceOptions()
	hintOpts.Size = min(screenW, screenH) * 0.032
	hint := draws.NewText("Tap anywhere")
	hint.SetFace(hintOpts)
	hint.Locate(screenW/2, screenH*0.92, draws.CenterMiddle)
	hint.Draw(img)

	return img
}

func sceneAnim(from, to float64) tween.Tween {
	var tw tween.Tween
	tw.MaxLoop = 1
	tw.Add(from, to-from, sceneTransitionDuration, tween.EaseOutExponential)
	tw.Start()
	return tw
}

func (s *SceneTest) Update(frame mosapp.Frame) {
	s.anim.Update()
	if s.anim.IsFinished() {
		s.index = s.to
	}
	pressed := false
	for _, ev := range frame.Events {
		if ev.Kind == input.EventDown {
			pressed = true
			break
		}
	}
	if !pressed {
		return
	}
	if !s.anim.IsFinished() {
		return
	}
	s.from = s.index
	s.to = (s.index + 1) % len(s.scenes)
	s.anim = sceneAnim(0, 1)
}

func (s *SceneTest) Draw(dst draws.Image) {
	if len(s.scenes) == 0 {
		return
	}
	if s.anim.IsFinished() {
		sp := s.sprites[s.index]
		sp.Locate(0, 0, draws.LeftTop)
		sp.Draw(dst)
		return
	}

	t := s.anim.Value()
	out := s.sprites[s.from]
	out.Locate(-s.screenW*t, 0, draws.LeftTop)
	out.Draw(dst)

	in := s.sprites[s.to]
	in.Locate(s.screenW*(1-t), 0, draws.LeftTop)
	in.Draw(dst)
}
