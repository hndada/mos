package windowing

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
)

const (
	DurationOpening = 500 * time.Millisecond
	DurationClosing = 800 * time.Millisecond
)

// WindowingServer is the OS compositor. It owns the layer stack and dispatches
// navigation, lifecycle, and system events via the Bus.
type WindowingServer struct {
	ScreenW float64
	ScreenH float64

	// Bus is the OS-wide event bus. Apps subscribe here; the server publishes here.
	Bus *event.Bus

	Logger func(string)

	wallpaper Wallpaper
	home      Home
	hist      History
	kb        Keyboard
	statusBar StatusBar
	curtain   Curtain
	lock      Lock

	showingRecents bool
	windows        []*Window
	screenshots    []draws.Image
}

func (ws *WindowingServer) SetLogger(logger func(string)) { ws.Logger = logger }

func (ws *WindowingServer) log(msg string) {
	if ws.Logger != nil {
		ws.Logger(msg)
	}
}

func (ws *WindowingServer) publish(e event.Event) {
	if ws.Bus != nil {
		ws.Bus.Publish(e)
	}
}

func (ws *WindowingServer) SetHome(h Home)           { ws.home = h }
func (ws *WindowingServer) SetWallpaper(w Wallpaper) { ws.wallpaper = w }
func (ws *WindowingServer) SetHistory(h History)     { ws.hist = h }
func (ws *WindowingServer) SetKeyboard(k Keyboard)   { ws.kb = k }
func (ws *WindowingServer) SetStatusBar(s StatusBar) { ws.statusBar = s }
func (ws *WindowingServer) SetCurtain(s Curtain)     { ws.curtain = s }
func (ws *WindowingServer) SetLock(l Lock)           { ws.lock = l }

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

func (ws *WindowingServer) ToggleCurtain() {
	if ws.curtain == nil {
		return
	}
	ws.curtain.Toggle()
	if ws.curtain.IsVisible() {
		ws.log("curtain show")
	} else {
		ws.log("curtain hide")
	}
}

func (ws *WindowingServer) SetScreenshots(shots []draws.Image) { ws.screenshots = shots }
func (ws *WindowingServer) Screenshots() []draws.Image         { return ws.screenshots }

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
	ctx := newWindowContext(ws)
	w := NewWindow(iconPos, iconSize, clr, appID, ws.ScreenW, ws.ScreenH, ctx)
	ws.windows = append(ws.windows, w)
	ws.log("launch " + appID)
	ws.log("window " + w.AppID() + ": " + LifecycleInitializing.String() + " -> " + w.lifecycle.String())
	ws.publish(event.Lifecycle{AppID: appID, Phase: event.PhaseCreated})
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

func (ws *WindowingServer) StartCall() { ws.ReceiveCall() }

func (ws *WindowingServer) RestoreActiveApp(state AppState) {
	if state.ID == "" {
		return
	}
	ctx := newWindowContext(ws)
	w := NewRestoredWindow(state, ws.ScreenW, ws.ScreenH, ctx)
	ws.windows = append(ws.windows, w)
	ws.log("restore " + state.ID)
	ws.log("window " + w.AppID() + ": " + LifecycleInitializing.String() + " -> " + w.lifecycle.String())
	ws.publish(event.Lifecycle{AppID: state.ID, Phase: event.PhaseCreated})
	ws.publish(event.Lifecycle{AppID: state.ID, Phase: event.PhaseResumed})
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
	if before == w.lifecycle {
		return
	}
	ws.log("window " + w.AppID() + ": " + before.String() + " -> " + w.lifecycle.String())

	switch w.lifecycle {
	case LifecycleShown:
		ws.publish(event.Lifecycle{AppID: w.AppID(), Phase: event.PhaseResumed})
	case LifecycleHiding:
		ws.publish(event.Lifecycle{AppID: w.AppID(), Phase: event.PhasePaused})
	case LifecycleDestroyed:
		ws.publish(event.Lifecycle{AppID: w.AppID(), Phase: event.PhaseDestroyed})
	}
}

func (ws *WindowingServer) GoHome() {
	ws.showingRecents = false
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Hide()
	}
	if w, ok := ws.activeWindow(); ok {
		if ws.hist != nil {
			ws.hist.AddCard(w.HistoryEntry())
		}
		ws.dismissTopWindowToCard()
	}
	ws.publish(event.Navigation{Action: event.NavHome})
	ws.log("home")
}

func (ws *WindowingServer) GoBack() {
	if ws.curtain != nil && ws.curtain.IsVisible() {
		ws.curtain.Hide()
		ws.log("back: curtain hide")
		return
	}
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
			ws.hist.AddCard(w.HistoryEntry())
		}
		ws.dismissTopWindow()
		ws.publish(event.Navigation{Action: event.NavBack})
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
			ws.hist.AddCard(w.HistoryEntry())
		}
		ws.dismissTopWindowToCard()
	}
	if ws.hist != nil {
		ws.hist.Show()
	}
	ws.publish(event.Navigation{Action: event.NavRecents})
	ws.log("recents show")
}

// HistoryEntries returns current history records newest-first, for persisting
// across screen or device-mode changes.
func (ws *WindowingServer) HistoryEntries() []apps.HistoryEntry {
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
	if ws.curtain != nil {
		ws.curtain.Update()
	}
	if ws.lock != nil {
		ws.lock.Update()
	}

	for _, w := range ws.windows {
		before := w.lifecycle
		w.Update()
		ws.logLifecycleChange(w, before)

		// Handle app-initiated launches (ctx.Launch).
		if id := w.app.ctx.drainLaunch(); id != "" {
			center := draws.XY{X: ws.ScreenW / 2, Y: ws.ScreenH / 2}
			size := draws.XY{X: ws.ScreenW * 0.5, Y: ws.ScreenH * 0.5}
			ws.launchApp(center, size, w.app.Color, id)
		}
	}

	// Once a card-launched window is fully open, hide the recents overlay.
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

	// Purge fully hidden windows: fire OnDestroy, then remove from the list.
	live := ws.windows[:0]
	for _, w := range ws.windows {
		if w.lifecycle == LifecycleHidden {
			before := w.lifecycle
			w.Destroy() // fires OnDestroy, sets LifecycleDestroyed
			ws.logLifecycleChange(w, before)
			continue
		}
		if w.lifecycle != LifecycleDestroyed {
			live = append(live, w)
		}
	}
	ws.windows = live
}

// Draw composites back-to-front:
// wallpaper → home | recents → windows → keyboard → status bar → curtain → lock.
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
		w.Draw(dst)
	}
	if ws.kb != nil {
		ws.kb.Draw(dst)
	}
	if ws.statusBar != nil {
		ws.statusBar.Draw(dst)
	}
	if ws.curtain != nil && ws.curtain.IsVisible() {
		ws.curtain.Draw(dst)
	}
	if ws.lock != nil {
		ws.lock.Draw(dst)
	}
}
