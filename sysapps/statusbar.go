package sysapps

import (
	"time"

	"github.com/hndada/mos/internal/draws"
)

type StatusBar interface {
	Draw(dst draws.Image)
}

type DefaultStatusBar struct {
	clock func() time.Time
}

func NewDefaultStatusBar() *DefaultStatusBar {
	return &DefaultStatusBar{clock: time.Now}
}

func (sb *DefaultStatusBar) Draw(_ draws.Image) {
	// renders clock, battery, signal indicators
	_ = sb.clock()
}
