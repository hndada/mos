package fw

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/sysapps"
)

const (
	DurationOpening = 500 * time.Millisecond
	DurationClosing = 800 * time.Millisecond
)

type WindowingServer struct {
	ScreenW float64
	ScreenH float64

	wallpaper      sysapps.Wallpaper
	home           sysapps.Home
	hist           sysapps.History
	kb             sysapps.Keyboard
	statusBar      sysapps.StatusBar
	lock           sysapps.Lock
	showingRecents bool

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

func (ws *WindowingServer) ToggleKeyboard() {
	if ws.kb == nil {
		return
	}
	if ws.kb.IsVisible() {
		ws.kb.Hide()
	} else {
		ws.kb.Show()
	}
}

func (ws *WindowingServer) launchApp(iconPos, iconSize draws.XY, clr color.RGBA) {
	ws.windows = append(ws.windows, NewWindow(iconPos, iconSize, clr, ws.ScreenW, ws.ScreenH))
}

// activeWindowColor returns the color of the topmost window that is still
// opening or fully shown (not yet dismissing). The second return value is
// false when no such window exists, preventing a fallback black card.
func (ws *WindowingServer) activeWindowColor() (color.RGBA, bool) {
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			return w.clr, true
		}
	}
	return color.RGBA{}, false
}

func (ws *WindowingServer) hasVisibleWindow() bool {
	for _, w := range ws.windows {
		if w.lifecycle.Visible() {
			return true
		}
	}
	return false
}

func (ws *WindowingServer) dismissTopWindow() {
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			w.Dismiss()
			return
		}
	}
}

func (ws *WindowingServer) dismissTopWindowToCard() {
	if ws.hist == nil {
		ws.dismissTopWindow()
		return
	}
	center, size := ws.hist.CardRect()
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			w.DismissTo(center, size)
			return
		}
	}
}

func (ws *WindowingServer) GoHome() {
	ws.showingRecents = false
	if clr, ok := ws.activeWindowColor(); ok {
		if ws.hist != nil {
			ws.hist.AddCard(clr)
		}
		ws.dismissTopWindowToCard()
	}
}

func (ws *WindowingServer) GoRecents() {
	if ws.showingRecents {
		ws.showingRecents = false
		return
	}
	ws.showingRecents = true
	if clr, ok := ws.activeWindowColor(); ok {
		if ws.hist != nil {
			ws.hist.AddCard(clr)
		}
		ws.dismissTopWindowToCard()
	}
	if ws.hist != nil {
		ws.hist.Show()
	}
}

// HistoryColors returns the current card colors newest-first, for persisting
// across screen or device-mode changes.
func (ws *WindowingServer) HistoryColors() []color.RGBA {
	if ws.hist == nil {
		return nil
	}
	return ws.hist.Colors()
}

func (ws *WindowingServer) Update() {
	if ws.home != nil && !ws.showingRecents {
		ws.home.Update()
		if pos, size, clr, ok := ws.home.TappedIcon(); ok {
			if !ws.hasVisibleWindow() {
				ws.launchApp(pos, size, clr)
			}
		}
	}
	if ws.hist != nil && ws.showingRecents {
		ws.hist.Update()
		if pos, size, clr, ok := ws.hist.TappedCard(); ok {
			ws.launchApp(pos, size, clr)
		}
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
	// Once the window launched from a history card is fully open, hide history.
	if ws.showingRecents {
		for _, w := range ws.windows {
			if w.lifecycle == LifecycleShown {
				ws.showingRecents = false
				break
			}
		}
	}
	// purge fully hidden windows
	live := ws.windows[:0]
	for _, w := range ws.windows {
		if w.lifecycle != LifecycleHidden && w.lifecycle != LifecycleDestroyed {
			live = append(live, w)
		}
	}
	ws.windows = live
}

// Draw composites back-to-front: wallpaper → home|recents → windows → keyboard → status bar → lock.
func (ws *WindowingServer) Draw(dst draws.Image) {
	if ws.wallpaper != nil {
		ws.wallpaper.Draw(dst)
	}
	if ws.showingRecents {
		if ws.hist != nil {
			ws.hist.Draw(dst)
		}
	} else {
		if ws.home != nil {
			ws.home.Draw(dst)
		}
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
