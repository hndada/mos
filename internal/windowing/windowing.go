package windowing

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
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

	// inputProducer turns Ebiten polling state into discrete events that
	// are routed to the active window's goroutine each frame.
	inputProducer input.Producer
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

// Lock / Unlock delegate to the bound Lock implementation, if any.
// IsLocked reports the current lock state (false when no Lock is bound).
func (ws *WindowingServer) Lock() {
	if ws.lock != nil && !ws.lock.IsLocked() {
		ws.lock.Lock()
		ws.log("lock")
	}
}

func (ws *WindowingServer) Unlock() {
	if ws.lock != nil && ws.lock.IsLocked() {
		ws.lock.Unlock()
		ws.log("unlock")
	}
}

func (ws *WindowingServer) IsLocked() bool {
	return ws.lock != nil && ws.lock.IsLocked()
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

// SafeArea computes the per-edge offsets reserved by system UI for the
// current frame. Visible status bar reserves the top; visible keyboard
// reserves the bottom.
func (ws *WindowingServer) SafeArea() mosapp.SafeArea {
	var sa mosapp.SafeArea
	if ws.statusBar != nil {
		sa.Top = ws.statusBar.Height()
	}
	if ws.kb != nil {
		sa.Bottom = ws.kb.Height()
	}
	return sa
}

func (ws *WindowingServer) launchApp(iconPos, iconSize draws.XY, clr color.RGBA, appID string) {
	w := NewWindow(iconPos, iconSize, clr, appID, ws.ScreenW, ws.ScreenH, ws)
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
	w := NewRestoredWindow(state, ws.ScreenW, ws.ScreenH, ws)
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

// Shutdown destroys every live window and joins its goroutine. Call this
// before discarding a WindowingServer instance (e.g. on display-mode
// change), otherwise the per-window goroutines are leaked.
func (ws *WindowingServer) Shutdown() {
	for _, w := range ws.windows {
		if w.lifecycle != LifecycleDestroyed {
			w.Destroy()
		}
	}
	ws.windows = nil
}

// Update is the per-frame entry point. Order of operations:
//
//  1. Produce input events from this frame's Ebiten state.
//  2. Update each window's published SafeArea.
//  3. Run main-thread system UI (home, recents, keyboard, status bar, curtain, lock).
//  4. For each window: animate; tick its goroutine if Shown; drain commands.
//  5. Purge fully hidden windows (firing OnDestroy on their goroutine).
func (ws *WindowingServer) Update() {
	events := ws.inputProducer.Poll()
	cx, cy := input.MouseCursorPosition()
	cursor := draws.XY{X: cx, Y: cy}
	locked := ws.IsLocked()

	sa := ws.SafeArea()
	for _, w := range ws.windows {
		w.ctx.setSafeArea(sa)
	}

	// While locked, the lock owns the entire input stream. Skip input-
	// processing system UI updates so taps don't bleed through to the
	// home / recents / curtain layers behind the overlay. The status bar
	// still updates so the wall clock keeps ticking.
	if !locked {
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
		if ws.curtain != nil {
			ws.curtain.Update()
		}
	}
	if ws.statusBar != nil {
		ws.statusBar.Update()
	}
	if ws.lock != nil {
		var lockFrame mosapp.Frame
		if locked {
			lockFrame = mosapp.Frame{Cursor: cursor, Events: events}
		}
		ws.lock.Update(lockFrame)
	}

	// Snapshot the slice: launchApp inside the loop would otherwise grow it.
	current := make([]*Window, len(ws.windows))
	copy(current, ws.windows)
	active, _ := ws.activeWindow()

	for _, w := range current {
		before := w.lifecycle
		w.Update()
		ws.logLifecycleChange(w, before)

		if w.lifecycle == LifecycleShown {
			// Only the active window receives input events; others tick with
			// just the cursor position so animations driven by Update still
			// see a coherent value. While locked, no app receives events.
			frame := mosapp.Frame{Cursor: cursor}
			if w == active && !locked {
				frame.Events = events
			}
			w.UpdateApp(frame)

			if w.proc.shouldClose {
				before := w.lifecycle
				w.Dismiss()
				ws.logLifecycleChange(w, before)
				w.proc.shouldClose = false
			}
		}

		if id := w.proc.drainLaunch(); id != "" {
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

	// Purge fully hidden windows: fire OnDestroy on their goroutine, then remove.
	live := ws.windows[:0]
	for _, w := range ws.windows {
		if w.lifecycle == LifecycleHidden {
			before := w.lifecycle
			w.Destroy() // sendTick(tickDestroy) → goroutine returns → marked Destroyed
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
//
// Each Window's content.Draw is invoked here on the main goroutine. Lock-step
// from Update() guarantees every app goroutine has acked its tick before
// Draw runs, so reading app state requires no mutex.
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
