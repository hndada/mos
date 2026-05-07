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

	// Multi-window state. mwm is nil when all windows are fullscreen.
	// focusedWindow is the window that receives pointer/touch input in split
	// and freeform modes; in fullscreen mode activeWindow() is used.
	mwm           *multiWindowManager
	focusedWindow *Window

	// keyboardFocus is the window that holds keyboard and IME focus. It is
	// managed independently from focusedWindow so that in PiP mode the small
	// overlay window can hold focus while the main window receives pointer
	// events. In fullscreen mode it always equals the active window.
	keyboardFocus *Window

	// Blur infrastructure for frosted-glass overlays (curtain, etc.).
	// blurSnap holds a copy of the scene before translucent layers are drawn;
	// blurOut holds the blurred result handed to the curtain via SetBackground.
	blurSnap draws.Image
	blurOut  draws.Image
	sceneBlur *draws.Blur

	// Timer-based activity tracking.
	// lastFrameActive is set to true during Update whenever there is real work
	// to do (input events, in-flight animations, explicit invalidations, or a
	// clock-minute boundary crossing). Callers read it via WasActive() after
	// Update returns and use it to throttle the Ebiten TPS when idle.
	lastFrameActive bool
	lastMinute      int // previous time.Now().Minute(); used for clock-tick detection
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

// PostNotice delivers a notice posted by an app to the curtain's notification
// list. Called on the main goroutine after the producing window's tick has
// acked, so it is safe to mutate curtain state here.
func (ws *WindowingServer) PostNotice(n mosapp.Notice) {
	if ws.curtain != nil {
		ws.curtain.AddNotice(n)
	}
	ws.log("notice: " + n.Title + " — " + n.Body)
}

// grantFocus moves keyboard/IME focus to w, notifying the previous holder.
// Passing nil clears the focus slot entirely. Called from drain (on the main
// goroutine) in response to CmdRequestFocus, and automatically by the server
// when a window becomes the sole active window in fullscreen mode.
func (ws *WindowingServer) grantFocus(w *Window) {
	if ws.keyboardFocus == w {
		return
	}
	if ws.keyboardFocus != nil {
		ws.keyboardFocus.ctx.hasFocus.Store(false)
	}
	ws.keyboardFocus = w
	if w != nil {
		w.ctx.hasFocus.Store(true)
		ws.log("keyboard focus -> " + w.AppID())
	} else {
		ws.log("keyboard focus cleared")
	}
}

// releaseFocus clears keyboard focus if w currently holds it.
// Called from drain in response to CmdReleaseFocus.
func (ws *WindowingServer) releaseFocus(w *Window) {
	if ws.keyboardFocus == w {
		ws.grantFocus(nil)
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

// ── Multi-window public API ────────────────────────────────────────────────

// EnterSplit places the top two shown windows side-by-side. If only one
// window exists a colour-placeholder is launched as the secondary. Calling
// EnterSplit while already in split mode exits split instead (toggle).
func (ws *WindowingServer) EnterSplit() {
	if ws.mwm != nil && ws.mwm.mode == multiModeSplit {
		ws.ExitMultiWindow()
		return
	}
	var shown []*Window
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			shown = append(shown, w)
			if len(shown) == 2 {
				break
			}
		}
	}
	if len(shown) == 0 {
		ws.log("split: no windows")
		return
	}
	if len(shown) == 1 {
		// Need a second window — launch a placeholder.
		center := draws.XY{X: ws.ScreenW * 0.75, Y: ws.ScreenH / 2}
		size := draws.XY{X: ws.ScreenW / 2, Y: ws.ScreenH}
		ws.launchApp(center, size, color.RGBA{50, 70, 110, 255}, "")
		shown = append(shown, ws.windows[len(ws.windows)-1])
	}
	if ws.mwm == nil {
		ws.mwm = newMultiWindowManager(ws.ScreenW, ws.ScreenH)
	}
	// shown[0] = topmost (right/bottom), shown[1] = older (left/top).
	// Place the older window as primary and the frontmost as secondary so
	// the app the user was looking at ends up on the right.
	primary, secondary := shown[len(shown)-1], shown[0]
	ws.mwm.enterSplit(primary, secondary)
	ws.focusedWindow = primary
	ws.log("split enter")
}

// EnterPip shrinks the top window into a corner overlay. The window behind
// it (if any) continues as fullscreen background. Calling EnterPip while
// already in pip mode exits pip instead (toggle).
func (ws *WindowingServer) EnterPip() {
	if ws.mwm != nil && ws.mwm.mode == multiModePip {
		ws.ExitMultiWindow()
		return
	}
	var pip *Window
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			pip = w
			break
		}
	}
	if pip == nil {
		ws.log("pip: no window")
		return
	}
	if ws.mwm == nil {
		ws.mwm = newMultiWindowManager(ws.ScreenW, ws.ScreenH)
	}
	ws.mwm.enterPip(pip, pipCornerBottomRight)
	ws.focusedWindow = pip
	ws.log("pip enter")
}

// EnterFreeform makes all shown windows draggable floating rectangles.
// Calling EnterFreeform while already in freeform mode exits it instead.
func (ws *WindowingServer) EnterFreeform() {
	if ws.mwm != nil && ws.mwm.mode == multiModeFreeform {
		ws.ExitMultiWindow()
		return
	}
	var shown []*Window
	for _, w := range ws.windows {
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			shown = append(shown, w)
		}
	}
	if len(shown) == 0 {
		ws.log("freeform: no windows")
		return
	}
	if ws.mwm == nil {
		ws.mwm = newMultiWindowManager(ws.ScreenW, ws.ScreenH)
	}
	ws.mwm.enterFreeform(shown)
	ws.focusedWindow = shown[len(shown)-1] // topmost
	ws.log("freeform enter")
}

// ExitMultiWindow returns all windows to fullscreen and clears multi-window state.
func (ws *WindowingServer) ExitMultiWindow() {
	if ws.mwm == nil {
		return
	}
	switch ws.mwm.mode {
	case multiModeSplit:
		ws.mwm.exitSplit()
	case multiModePip:
		ws.mwm.exitPip()
	case multiModeFreeform:
		ws.mwm.exitFreeform(ws.windows)
	}
	ws.mwm.mode = multiModeNone
	ws.focusedWindow = nil
	ws.log("multi-window exit")
}

// CycleFocus moves keyboard focus to the next visible window in split or
// freeform mode.
func (ws *WindowingServer) CycleFocus() {
	if ws.mwm == nil || ws.mwm.mode == multiModeNone {
		return
	}
	var visible []*Window
	for _, w := range ws.windows {
		if w.lifecycle == LifecycleShown {
			visible = append(visible, w)
		}
	}
	if len(visible) == 0 {
		return
	}
	idx := 0
	for i, w := range visible {
		if w == ws.focusedWindow {
			idx = (i + 1) % len(visible)
			break
		}
	}
	ws.focusedWindow = visible[idx]
	ws.log("focus -> " + ws.focusedWindow.AppID())
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
		// In fullscreen mode the newly shown window is the only active window;
		// auto-grant keyboard focus so apps can receive IME input without an
		// explicit RequestFocus call. In multi-window modes focus is managed
		// explicitly by the user or by apps calling RequestFocus().
		if ws.mwm == nil || ws.mwm.mode == multiModeNone {
			ws.grantFocus(w)
		}
	case LifecycleHiding:
		ws.publish(event.Lifecycle{AppID: w.AppID(), Phase: event.PhasePaused})
		// Release keyboard focus when the window starts its closing animation
		// so the next visible window can acquire it immediately.
		ws.releaseFocus(w)
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
	ws.mwm = nil
	ws.focusedWindow = nil
	ws.keyboardFocus = nil
}

// WasActive reports whether the most recent Update call observed any real
// work: input events, in-flight animations, explicit invalidations, or a
// clock-minute boundary crossing.
//
// Callers (typically the simulator) use this to throttle the Ebiten TPS:
// keep full rate while WasActive is true and drop to a low polling rate
// once consecutive idle frames exceed a threshold.
func (ws *WindowingServer) WasActive() bool { return ws.lastFrameActive }

// InvalidateAll marks every shown window as dirty so that each app's Draw is
// called on the next frame regardless of whether it received input. Use this
// after a global state change such as a theme switch.
func (ws *WindowingServer) InvalidateAll() {
	for _, w := range ws.windows {
		w.ctx.invalidated.Store(true)
	}
}

// Update is the per-frame entry point. Order of operations:
//
//  1. Produce input events from this frame's Ebiten state.
//  2. Update each window's published SafeArea.
//  3. Run main-thread system UI (home, recents, keyboard, status bar, curtain, lock).
//  4. Multi-window manager consumes its events (divider drag, pip drag, title drag).
//  5. Update focus from EventDown in any window's display rect.
//  6. For each window: animate; tick its goroutine if Shown; drain commands.
//  7. Purge fully hidden windows; check if multi-window anchors are still alive.
func (ws *WindowingServer) Update() {
	events := ws.inputProducer.Poll()
	cx, cy := input.MouseCursorPosition()
	cursor := draws.XY{X: cx, Y: cy}
	locked := ws.IsLocked()

	// ── Activity probe (for TPS throttling) ──────────────────────────────────
	// Evaluate before processing so that CompareAndSwap in the per-window loop
	// does not consume the invalidated flag before we read it here.
	ws.lastFrameActive = len(events) > 0
	if m := time.Now().Minute(); m != ws.lastMinute {
		// Minute boundary crossed — status bar clock text must change.
		ws.lastMinute = m
		ws.lastFrameActive = true
	}
	if ws.curtain != nil && ws.curtain.IsVisible() {
		ws.lastFrameActive = true
	}
	for _, w := range ws.windows {
		if !w.anim.Done() || w.ctx.invalidated.Load() {
			ws.lastFrameActive = true
			break
		}
	}
	// ─────────────────────────────────────────────────────────────────────────

	sa := ws.SafeArea()
	for _, w := range ws.windows {
		w.ctx.setSafeArea(sa)
	}

	// Layered input gating: lock owns input when locked; curtain owns input
	// when visible (and not locked); otherwise the layers underneath get their
	// normal turn.
	curtainOpen := !locked && ws.curtain != nil && ws.curtain.IsVisible()
	gateBelow := locked || curtainOpen
	if !gateBelow {
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
	}
	if ws.curtain != nil {
		var curtainFrame mosapp.Frame
		if curtainOpen {
			curtainFrame = mosapp.Frame{Cursor: cursor, Events: events}
		} else {
			curtainFrame = mosapp.Frame{Cursor: cursor}
		}
		ws.curtain.Update(curtainFrame)
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

	// Multi-window manager: consume divider / pip / title-bar drag events
	// before the per-window routing step. The manager also maintains its own
	// event buffers (pipFrameEvents / mainFrameEvents) for PiP mode.
	if !gateBelow && ws.mwm != nil && ws.mwm.mode != multiModeNone {
		switch ws.mwm.mode {
		case multiModeSplit:
			events = ws.mwm.updateSplit(events)
		case multiModePip:
			ws.mwm.updatePip(events)
		case multiModeFreeform:
			var newFocus *Window
			events, newFocus = ws.mwm.updateFreeform(events, ws.windows, ws.focusedWindow)
			if newFocus != nil {
				ws.focusedWindow = newFocus
			}
		}
	}

	// Update focus: any EventDown inside a window's display rect focuses it.
	// In fullscreen mode this is a no-op (activeWindow() handles routing).
	if !gateBelow && ws.mwm != nil && ws.mwm.mode != multiModeNone {
		for _, ev := range events {
			if ev.Kind != input.EventDown {
				continue
			}
			for i := len(ws.windows) - 1; i >= 0; i-- {
				w := ws.windows[i]
				if w.lifecycle == LifecycleShown && w.ContainsScreenPos(ev.Pos) {
					ws.focusedWindow = w
					break
				}
			}
		}
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
			frame := ws.frameForWindow(w, cursor, events, active, gateBelow)

			// Implicit dirty tracking: only wake the app goroutine when there
			// is something to react to.
			//
			//  • hasEvents   – pointer / touch input arrived at this window
			//  • animActive  – window layout transition still in-flight (mode
			//                  switch, freeform drag, etc.)
			//  • needsWakeup – app called ctx.Invalidate() from any goroutine
			//                  (explicit escape hatch for background state changes
			//                  or widget animations that outlive their input event)
			//
			// Apps with animated widgets (e.g. toggle knob slide) should call
			// ctx.Invalidate() from Update while the animation is in-flight so
			// ticks continue until it completes.
			hasEvents   := len(frame.Events) > 0
			animActive  := !w.anim.Done()
			needsWakeup := w.ctx.invalidated.CompareAndSwap(true, false)

			if hasEvents || animActive || needsWakeup {
				w.UpdateApp(frame) // marks canvasDirty = true internally
			}

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
			w.Destroy()
			ws.logLifecycleChange(w, before)
			continue
		}
		if w.lifecycle != LifecycleDestroyed {
			live = append(live, w)
		}
	}
	ws.windows = live

	// If a split/pip anchor window was destroyed, exit that mode cleanly.
	if ws.mwm != nil && ws.mwm.mode != multiModeNone {
		ws.mwm.cleanup(ws.windows)
		if ws.mwm.mode == multiModeNone {
			ws.focusedWindow = nil
			ws.log("multi-window auto-exit (anchor destroyed)")
		}
		// Purge focusedWindow if it is no longer alive.
		if ws.focusedWindow != nil && !isWindowInList(ws.focusedWindow, ws.windows) {
			ws.focusedWindow = nil
		}
	}
}

// frameForWindow builds the mosapp.Frame to send to window w for this tick.
// In fullscreen mode the active window receives all events; in multi-window
// modes events are filtered to each window's display rect and transformed
// to canvas-local coordinates.
func (ws *WindowingServer) frameForWindow(w *Window, cursor draws.XY, events []input.Event, active *Window, gateBelow bool) mosapp.Frame {
	var windowEvents []input.Event

	if !gateBelow {
		if ws.mwm == nil || ws.mwm.mode == multiModeNone {
			// Fullscreen: only the topmost active window gets events.
			if w == active {
				windowEvents = events
			}
		} else {
			switch ws.mwm.mode {
			case multiModeSplit:
				// Both split windows receive events filtered to their rect.
				if w == ws.mwm.splitPrimary || w == ws.mwm.splitSecondary {
					windowEvents = filterEventsByRect(events, w.anim.Pos(), w.anim.Size())
				}
			case multiModePip:
				if w == ws.mwm.pipWindow {
					windowEvents = ws.mwm.pipFrameEvents
				} else {
					windowEvents = ws.mwm.mainFrameEvents
				}
			case multiModeFreeform:
				// Only the focused window receives events (in its rect).
				if w == ws.focusedWindow {
					windowEvents = filterEventsByRect(events, w.anim.Pos(), w.anim.Size())
				}
			}
		}
	}

	// Fullscreen windows need no coordinate transform (canvas == screen).
	if w.mode == WindowModeFullscreen {
		return mosapp.Frame{Cursor: cursor, Events: windowEvents}
	}
	// Non-fullscreen: translate from display-rect space to canvas space.
	return w.ToCanvasFrame(
		mosapp.Frame{Cursor: cursor, Events: windowEvents},
		ws.ScreenW, ws.ScreenH,
	)
}

// Draw composites back-to-front:
//
//	wallpaper → home | recents → windows →
//	[pip border] → [split divider] → [freeform title bars] →
//	keyboard → status bar → curtain → lock
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

	// In PiP mode, draw the PiP border BEFORE the pip window so the
	// window sprite renders on top of the border.
	if ws.mwm != nil && ws.mwm.mode == multiModePip {
		ws.mwm.drawPip(dst)
	}

	for _, w := range ws.windows {
		w.Draw(dst)
	}

	// Multi-window overlays drawn on top of window content.
	if ws.mwm != nil {
		switch ws.mwm.mode {
		case multiModeSplit:
			ws.mwm.drawSplit(dst)
		case multiModeFreeform:
			ws.mwm.drawFreeform(dst, ws.windows, ws.focusedWindow)
		}
	}

	if ws.kb != nil {
		ws.kb.Draw(dst)
	}
	if ws.statusBar != nil {
		ws.statusBar.Draw(dst)
	}

	// Frosted-glass blur: snapshot the fully composited scene (wallpaper +
	// windows + status bar), blur it, and hand it to the curtain so it can
	// render a glass backdrop. This happens before curtain.Draw so the
	// snapshot does not include the curtain itself.
	if ws.curtain != nil && ws.curtain.IsVisible() {
		ws.curtain.SetBackground(ws.blurredScene(dst))
		ws.curtain.Draw(dst)
	}

	if ws.lock != nil {
		ws.lock.Draw(dst)
	}
}

// blurredScene returns a blurred copy of dst using lazily allocated buffers.
// The result image is reused across frames; callers must not retain it.
func (ws *WindowingServer) blurredScene(dst draws.Image) draws.Image {
	size := dst.Size()

	if ws.blurSnap.IsEmpty() || ws.blurSnap.Size() != size {
		ws.blurSnap = draws.CreateImage(size.X, size.Y)
		ws.blurOut = draws.CreateImage(size.X, size.Y)
	}
	if ws.sceneBlur == nil {
		ws.sceneBlur = draws.NewBlur(8) // factor-8 ≈ moderate frosted-glass
	}

	// Copy current dst pixels into the snapshot buffer.
	ws.blurSnap.Clear()
	ws.blurSnap.DrawImage(dst.Image, &ebiten.DrawImageOptions{})

	// Blur snapshot → blurOut.
	ws.blurOut.Clear()
	ws.sceneBlur.Apply(ws.blurSnap, ws.blurOut)

	return ws.blurOut
}
