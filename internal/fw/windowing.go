package fw

import (
	"time"

	"github.com/hndada/mos/sysapps"
)

const (
	DurationLaunch = 220 * time.Millisecond
	DurationSplash = 800 * time.Millisecond
)

type WindowingServer struct {
	home sysapps.Home
	hist sysapps.History
	kb   sysapps.Keyboard
}

func (ws *WindowingServer) ShowKeyboard() { ws.kb.Toggle() }
func (ws *WindowingServer) HideKeyboard() { ws.kb.Toggle() }
func (ws *WindowingServer) showSplash()   {}
