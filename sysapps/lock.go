package sysapps

import (
	"time"

	"github.com/hndada/mos/internal/draws"
)

type Lock interface {
	Lock()
	Unlock()
	IsLocked() bool
	Update()
	Draw(dst draws.Image)
}

type DefaultLock struct {
	locked   bool
	lockedAt time.Time
	clock    draws.Text
}

func NewDefaultLock(screenW, screenH float64) *DefaultLock {
	t := draws.NewText("")
	t.Locate(screenW/2, screenH/2, draws.CenterMiddle)
	return &DefaultLock{clock: t}
}

func (l *DefaultLock) Lock() {
	l.locked = true
	l.lockedAt = time.Now()
}

func (l *DefaultLock) Unlock()        { l.locked = false }
func (l *DefaultLock) IsLocked() bool { return l.locked }

func (l *DefaultLock) Update() {
	if l.locked {
		l.clock.Text = time.Now().Format("15:04")
	}
}

func (l *DefaultLock) Draw(dst draws.Image) {
	if !l.locked {
		return
	}
	l.clock.Draw(dst)
}
