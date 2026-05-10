package apps

import (
	"image/color"
	"time"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

// lockUnlockMinPx is the minimum upward swipe distance that releases the lock.
const lockUnlockMinPx = 80.0

// DefaultLock renders a full-screen lock overlay with a clock, date, and a
// "swipe up to unlock" hint. While locked, the windowing server routes
// input here only; an upward drag of more than lockUnlockMinPx unlocks.
type DefaultLock struct {
	screenW, screenH float64

	locked   bool
	lockedAt time.Time

	overlay draws.Sprite
	clock   draws.Text
	date    draws.Text
	hint    draws.Text

	// Swipe tracking. tracking is true between Down (anywhere on the
	// overlay) and the next Up; swipeStart caches the Down position.
	tracking   bool
	swipeStart draws.XY
}

func NewDefaultLock(screenW, screenH float64) *DefaultLock {
	overlayImg := draws.CreateImage(screenW, screenH)
	overlayImg.Fill(color.RGBA{0, 0, 0, 240})
	overlay := draws.NewSprite(overlayImg)
	overlay.Locate(0, 0, draws.LeftTop)

	clockOpts := draws.NewFaceOptions()
	clockOpts.Size = 64
	clock := draws.NewText("")
	clock.SetFace(clockOpts)
	clock.Locate(screenW/2, screenH*0.30, draws.CenterMiddle)

	dateOpts := draws.NewFaceOptions()
	dateOpts.Size = 18
	date := draws.NewText("")
	date.SetFace(dateOpts)
	date.Locate(screenW/2, screenH*0.30+50, draws.CenterMiddle)

	hintOpts := draws.NewFaceOptions()
	hintOpts.Size = 14
	hint := draws.NewText("Swipe up to unlock")
	hint.SetFace(hintOpts)
	hint.Locate(screenW/2, screenH*0.88, draws.CenterMiddle)

	return &DefaultLock{
		screenW: screenW,
		screenH: screenH,
		overlay: overlay,
		clock:   clock,
		date:    date,
		hint:    hint,
	}
}

func (l *DefaultLock) Lock() {
	l.locked = true
	l.lockedAt = time.Now()
	l.tracking = false
}

func (l *DefaultLock) Unlock() {
	l.locked = false
	l.tracking = false
}

func (l *DefaultLock) IsLocked() bool { return l.locked }

func (l *DefaultLock) Update(frame mosapp.Frame) {
	if !l.locked {
		return
	}
	now := time.Now()
	l.clock.Text = now.Format("15:04")
	l.date.Text = now.Format("Mon, Jan 2")

	for _, ev := range frame.Events {
		switch ev.Kind {
		case input.EventDown:
			l.tracking = true
			l.swipeStart = ev.Pos
		case input.EventMove:
			if l.tracking {
				dy := l.swipeStart.Y - ev.Pos.Y // positive when swiping up
				if dy >= lockUnlockMinPx {
					l.Unlock()
					return
				}
			}
		case input.EventUp:
			l.tracking = false
		}
	}
}

func (l *DefaultLock) Draw(dst draws.Image) {
	if !l.locked {
		return
	}
	l.overlay.Draw(dst)
	l.clock.Draw(dst)
	l.date.Draw(dst)
	l.hint.Draw(dst)
}
