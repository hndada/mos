package apps

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2/vector"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui"
)

type Call struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64
	started time.Time

	name     draws.Text
	number   draws.Text
	state    draws.Text
	duration draws.Text
	avatar   draws.Sprite
	end      draws.Sprite
	endText  draws.Text
	endTap   ui.TriggerButton
	ended    bool
}

func (c *Call) OnCreate(ctx mosapp.Context) { c.ctx = ctx }
func (c *Call) OnResume()                   { c.scheduleNextTick() }
func (c *Call) OnPause()                    {}
func (c *Call) OnDestroy()                  {}

func NewCall(screenW, screenH float64) *Call {
	c := &Call{
		screenW: screenW,
		screenH: screenH,
		started: time.Now(),
	}

	nameOpts := draws.NewFaceOptions()
	nameOpts.Size = 28
	c.name = draws.NewText("MOS Call")
	c.name.SetFace(nameOpts)
	c.name.Locate(screenW/2, screenH*0.30, draws.CenterMiddle)

	numberOpts := draws.NewFaceOptions()
	numberOpts.Size = 15
	c.number = draws.NewText("+1 555 010 0420")
	c.number.SetFace(numberOpts)
	c.number.Locate(screenW/2, screenH*0.30+34, draws.CenterMiddle)

	stateOpts := draws.NewFaceOptions()
	stateOpts.Size = 16
	c.state = draws.NewText("Incoming call")
	c.state.SetFace(stateOpts)
	c.state.Locate(screenW/2, screenH*0.30+72, draws.CenterMiddle)

	durationOpts := draws.NewFaceOptions()
	durationOpts.Size = 14
	c.duration = draws.NewText("00:00")
	c.duration.SetFace(durationOpts)
	c.duration.Locate(screenW/2, screenH*0.30+100, draws.CenterMiddle)

	avatarSize := min(screenW, screenH) * 0.22
	c.avatar = newCircleSprite(avatarSize, color.RGBA{38, 197, 107, 255})
	c.avatar.Locate(screenW/2, screenH*0.18, draws.CenterMiddle)

	endSize := min(screenW, screenH) * 0.15
	endX := screenW/2 - endSize/2
	endY := screenH*0.78 - endSize/2
	c.end = newCircleSprite(endSize, color.RGBA{255, 59, 48, 255})
	c.end.Locate(screenW/2, screenH*0.78, draws.CenterMiddle)
	c.endTap.SetRect(endX, endY, endSize, endSize)

	endOpts := draws.NewFaceOptions()
	endOpts.Size = 14
	c.endText = draws.NewText("End")
	c.endText.SetFace(endOpts)
	c.endText.Locate(screenW/2, screenH*0.78+endSize/2+18, draws.CenterMiddle)

	return c
}

func newCircleSprite(diameter float64, clr color.Color) draws.Sprite {
	img := draws.CreateImage(diameter, diameter)
	r := float32(diameter / 2)
	vector.DrawFilledCircle(img.Image, r, r, r, clr, true)
	return draws.NewSprite(img)
}

func (c *Call) Update(frame mosapp.Frame) {
	if c.endTap.Update(frame) {
		c.ended = true
		return
	}
	elapsed := time.Since(c.started).Truncate(time.Second)
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	c.duration.Text = fmt.Sprintf("%02d:%02d", minutes, seconds)
	if elapsed >= 2*time.Second {
		c.state.Text = "Ringing"
	}
	c.scheduleNextTick()
}

func (c *Call) scheduleNextTick() {
	if c.ctx == nil || c.ended {
		return
	}
	now := time.Now()
	c.ctx.WakeAt(now.Truncate(time.Second).Add(time.Second))
}

func (c *Call) ShouldClose() bool {
	return c.ended
}

func (c *Call) Draw(dst draws.Image) {
	dst.Fill(color.RGBA{12, 18, 18, 255})
	c.avatar.Draw(dst)
	c.name.Draw(dst)
	c.number.Draw(dst)
	c.state.Draw(dst)
	c.duration.Draw(dst)
	c.end.Draw(dst)
	c.endText.Draw(dst)
}
