package fw

import (
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/sysapps"
)

const (
	DurationSplash = 500 * time.Millisecond
)

type WindowingServer struct {
	ScreenW float64
	ScreenH float64

	wallpaper sysapps.Wallpaper
	home      sysapps.Home
	hist      sysapps.History
	kb        sysapps.Keyboard
	statusBar sysapps.StatusBar
	lock      sysapps.Lock

	windows []*Window
}

func (ws *WindowingServer) SetHome(h sysapps.Home)           { ws.home = h }
func (ws *WindowingServer) SetWallpaper(w sysapps.Wallpaper) { ws.wallpaper = w }
func (ws *WindowingServer) SetHistory(h sysapps.History)     { ws.hist = h }
func (ws *WindowingServer) SetKeyboard(k sysapps.Keyboard)   { ws.kb = k }
func (ws *WindowingServer) SetStatusBar(s sysapps.StatusBar) { ws.statusBar = s }
func (ws *WindowingServer) SetLock(l sysapps.Lock)           { ws.lock = l }

func (ws *WindowingServer) ShowKeyboard() {
	if ws.kb != nil && !ws.kb.IsVisible() {
		ws.kb.Show()
	}
}

func (ws *WindowingServer) HideKeyboard() {
	if ws.kb != nil && ws.kb.IsVisible() {
		ws.kb.Hide()
	}
}

func (ws *WindowingServer) launchApp(iconPos, iconSize draws.XY) {
	ws.windows = append(ws.windows, NewWindow(iconPos, iconSize, ws.ScreenW, ws.ScreenH))
}

func (ws *WindowingServer) hasVisibleWindow() bool {
	for _, w := range ws.windows {
		if w.lifecycle.Visible() {
			return true
		}
	}
	return false
}

func (ws *WindowingServer) Update() {
	if ws.home != nil {
		ws.home.Update()
		if pos, size, ok := ws.home.TappedIcon(); ok {
			if !ws.hasVisibleWindow() {
				ws.launchApp(pos, size)
			}
		}
	}
	if ws.hist != nil {
		ws.hist.Update()
	}
	if ws.kb != nil {
		ws.kb.Update()
	}
	if ws.statusBar != nil {
		ws.statusBar.Update()
	}
	if ws.lock != nil {
		ws.lock.Update()
	}
	for _, w := range ws.windows {
		w.Update()
	}
}

// Draw composites back-to-front: wallpaper → home/history → windows → keyboard → status bar → lock.
func (ws *WindowingServer) Draw(dst draws.Image) {
	if ws.wallpaper != nil {
		ws.wallpaper.Draw(dst)
	}
	if ws.home != nil {
		ws.home.Draw(dst)
	}
	if ws.hist != nil {
		ws.hist.Draw(dst)
	}
	for _, w := range ws.windows {
		w.Draw(dst)
	}
	if ws.kb != nil {
		ws.kb.Draw(dst)
	}
	if ws.statusBar != nil {
		ws.statusBar.Draw(dst)
	}
	if ws.lock != nil {
		ws.lock.Draw(dst)
	}
}
