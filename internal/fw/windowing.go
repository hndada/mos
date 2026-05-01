package fw

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
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
	Logger  func(string)

	wallpaper      sysapps.Wallpaper
	home           sysapps.Home
	hist           sysapps.History
	kb             sysapps.Keyboard
	statusBar      sysapps.StatusBar
	lock           sysapps.Lock
	showingRecents bool

	windows     []*Window
	screenshots []draws.Image
}

func (ws *WindowingServer) SetLogger(logger func(string)) { ws.Logger = logger }

func (ws *WindowingServer) log(msg string) {
	if ws.Logger != nil {
		ws.Logger(msg)
	}
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
		ws.log("keyboard show")
	}
}

func (ws *WindowingServer) HideKeyboard() {
	if ws.kb != nil && ws.kb.IsVisible() {
		ws.kb.Hide()
		ws.log("keyboard hide")
	}
}

func (ws *WindowingServer) ToggleKeyboard() {
	if ws.kb == nil {
		return
	}
	if ws.kb.IsVisible() {
		ws.kb.Hide()
		ws.log("keyboard hide")
	} else {
		ws.kb.Show()
		ws.log("keyboard show")
	}
}

func (ws *WindowingServer) SetScreenshots(shots []draws.Image) { ws.screenshots = shots }

func (ws *WindowingServer) Screenshots() []draws.Image { return ws.screenshots }

func (ws *WindowingServer) AddScreenshot(src draws.Image) {
	if src.IsEmpty() {
		return
	}
	size := src.Size()
	shot := draws.CreateImage(size.X, size.Y)
	shot.DrawImage(src.Image, &ebiten.DrawImageOptions{})
	ws.screenshots = append(ws.screenshots, shot)
	ws.log("screenshot captured")
}

func (ws *WindowingServer) launchApp(iconPos, iconSize draws.XY, clr color.RGBA, appID string) {
	w := NewWindow(iconPos, iconSize, clr, appID, ws.ScreenW, ws.ScreenH)
	ws.windows = append(ws.windows, w)
	ws.log("launch " + appID)
	ws.log("window " + w.AppID() + ": " + LifecycleInitializing.String() + " -> " + w.lifecycle.String())
}

func (ws *WindowingServer) ReceiveCall() {
	if w, ok := ws.activeWindow(); ok && w.app.ID == AppIDCall {
		ws.log("call already active")
		return
	}
	iconSize := draws.XY{X: ws.ScreenW * 0.18, Y: ws.ScreenW * 0.18}
	iconPos := draws.XY{X: ws.ScreenW / 2, Y: ws.ScreenH - iconSize.Y}
	ws.launchApp(iconPos, iconSize, color.RGBA{38, 197, 107, 255}, AppIDCall)
	ws.showingRecents = false
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Hide()
	}
	ws.log("incoming call")
}

func (ws *WindowingServer) StartCall() {
	ws.ReceiveCall()
}

func (ws *WindowingServer) RestoreActiveApp(state AppState) {
	if state.ID == "" {
		return
	}
	w := NewRestoredWindow(state, ws.ScreenW, ws.ScreenH)
	ws.windows = append(ws.windows, w)
	ws.log("restore " + state.ID)
	ws.log("window " + w.AppID() + ": " + LifecycleInitializing.String() + " -> " + w.lifecycle.String())
}

func (ws *WindowingServer) activeWindow() (*Window, bool) {
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			return w, true
		}
	}
	return nil, false
}

func (ws *WindowingServer) ActiveAppState() (AppState, bool) {
	w, ok := ws.activeWindow()
	if !ok {
		return AppState{}, false
	}
	return w.AppState(), true
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
			before := w.lifecycle
			w.Dismiss()
			ws.logLifecycleChange(w, before)
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
			before := w.lifecycle
			w.DismissTo(center, size)
			ws.logLifecycleChange(w, before)
			return
		}
	}
}

func (ws *WindowingServer) logLifecycleChange(w *Window, before Lifecycle) {
	if before != w.lifecycle {
		ws.log("window " + w.AppID() + ": " + before.String() + " -> " + w.lifecycle.String())
	}
}

func (ws *WindowingServer) GoHome() {
	ws.showingRecents = false
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Hide()
	}
	if w, ok := ws.activeWindow(); ok {
		if ws.hist != nil {
			ws.hist.AddCard(w.HistoryEntry(ws.screenshots))
		}
		ws.dismissTopWindowToCard()
	}
	ws.log("home")
}

func (ws *WindowingServer) GoBack() {
	if ws.kb != nil && ws.kb.IsVisible() {
		ws.kb.Hide()
		ws.log("back: keyboard hide")
		return
	}
	if ws.showingRecents || (ws.hist != nil && ws.hist.IsVisible()) {
		if ws.hist != nil {
			ws.hist.Hide()
		}
		ws.showingRecents = false
		ws.log("back: recents hide")
		return
	}
	if w, ok := ws.activeWindow(); ok {
		if ws.hist != nil {
			ws.hist.AddCard(w.HistoryEntry(ws.screenshots))
		}
		ws.dismissTopWindow()
		ws.log("back")
		return
	}
	ws.log("back: no-op")
}

func (ws *WindowingServer) GoRecents() {
	if ws.showingRecents {
		if ws.hist != nil {
			ws.hist.Hide()
		}
		ws.showingRecents = false
		ws.log("recents hide")
		return
	}
	ws.showingRecents = true
	if w, ok := ws.activeWindow(); ok {
		if ws.hist != nil {
			ws.hist.AddCard(w.HistoryEntry(ws.screenshots))
		}
		ws.dismissTopWindowToCard()
	}
	if ws.hist != nil {
		ws.hist.Show()
	}
	ws.log("recents show")
}

// HistoryEntries returns current history records newest-first, for persisting
// across screen or device-mode changes.
func (ws *WindowingServer) HistoryEntries() []sysapps.HistoryEntry {
	if ws.hist == nil {
		return nil
	}
	return ws.hist.Entries()
}

func (ws *WindowingServer) Update() {
	if ws.home != nil && !ws.showingRecents {
		ws.home.Update()
		if pos, size, clr, appID, ok := ws.home.TappedIcon(); ok {
			if !ws.hasVisibleWindow() {
				ws.launchApp(pos, size, clr, appID)
			}
		}
	}
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Update()
	}
	if ws.hist != nil && ws.showingRecents && ws.hist.IsInteractive() {
		if pos, size, entry, ok := ws.hist.TappedCard(); ok {
			ws.launchApp(pos, size, entry.Color, entry.AppID)
			ws.hist.Hide()
			ws.showingRecents = false
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
		before := w.lifecycle
		w.Update()
		ws.logLifecycleChange(w, before)
	}
	// Once the window launched from a history card is fully open, hide history.
	if ws.showingRecents {
		for _, w := range ws.windows {
			if w.lifecycle == LifecycleShown {
				if ws.hist != nil {
					ws.hist.Hide()
				}
				ws.showingRecents = false
				break
			}
		}
	}
	// purge fully hidden windows
	live := ws.windows[:0]
	for _, w := range ws.windows {
		if w.lifecycle == LifecycleHidden {
			before := w.lifecycle
			w.lifecycle = LifecycleDestroyed
			ws.logLifecycleChange(w, before)
			continue
		}
		if w.lifecycle != LifecycleDestroyed {
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
	if ws.home != nil {
		ws.home.Draw(dst)
	}
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Draw(dst)
	}
	for _, w := range ws.windows {
		w.Draw(dst, ws.screenshots)
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
