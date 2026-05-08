package windowing

import (
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// Server is the OS compositor. It owns the layer stack and dispatches
// navigation, lifecycle, and system events via the Bus.
type Server struct {
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

	// inputProducer turns Ebiten polling state into discrete events that are
	// routed to the active window each frame.
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
	blurSnap  draws.Image
	blurOut   draws.Image
	sceneBlur *draws.Blur

	sceneVersion uint64
	blurVersion  uint64
	statusMinute string

	serviceMu        sync.RWMutex
	darkMode         bool
	clipboard        string
	preferences      map[string]string
	permissions      map[mosapp.Permission]mosapp.PermissionStatus
	titles           map[string]string
	badges           map[string]int
	barStyles        map[string]mosapp.BarStyle
	keepAwake        map[string]bool
	orientations     map[string]mosapp.Orientation
	systemBarsHidden map[string]bool
	scheduled        map[string]scheduledNotice
	latestExport     string
	audioFocus       string
	bgSeq            int
	bgTasks          map[string]string
}

type scheduledNotice struct {
	AppID  string
	ID     string
	Notice mosapp.Notice
	At     time.Time
}

var pathPartReplacer = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")

func (ws *Server) SetLogger(logger func(string)) { ws.Logger = logger }

func (ws *Server) log(msg string) {
	if ws.Logger != nil {
		ws.Logger(msg)
	}
}

func (ws *Server) publish(e event.Event) {
	if ws.Bus != nil {
		ws.Bus.Publish(e)
	}
}

func (ws *Server) invalidateScene() { ws.sceneVersion++ }

func (ws *Server) SetHome(h Home)           { ws.home = h }
func (ws *Server) SetWallpaper(w Wallpaper) { ws.wallpaper = w }
func (ws *Server) SetHistory(h History)     { ws.hist = h }
func (ws *Server) SetKeyboard(k Keyboard)   { ws.kb = k }
func (ws *Server) SetStatusBar(s StatusBar) { ws.statusBar = s }
func (ws *Server) SetCurtain(s Curtain)     { ws.curtain = s }
func (ws *Server) SetLock(l Lock)           { ws.lock = l }

func (ws *Server) ShowKeyboard() {
	if ws.kb != nil && !ws.kb.IsVisible() {
		ws.kb.Show()
		ws.invalidateScene()
		ws.log("keyboard show")
	}
}

func (ws *Server) HideKeyboard() {
	if ws.kb != nil && ws.kb.IsVisible() {
		ws.kb.Hide()
		ws.invalidateScene()
		ws.log("keyboard hide")
	}
}

func (ws *Server) ToggleKeyboard() {
	if ws.kb == nil {
		return
	}
	if ws.kb.IsVisible() {
		ws.kb.Hide()
		ws.invalidateScene()
		ws.log("keyboard hide")
	} else {
		ws.kb.Show()
		ws.invalidateScene()
		ws.log("keyboard show")
	}
}

func (ws *Server) KeyboardVisible() bool {
	return ws.kb != nil && ws.kb.IsVisible()
}

func (ws *Server) Locale() string           { return "en-US" }
func (ws *Server) TimeZone() *time.Location { return time.Local }
func (ws *Server) Now() time.Time           { return time.Now() }
func (ws *Server) FontScale() float64       { return 1 }
func (ws *Server) ReduceMotion() bool       { return false }
func (ws *Server) BatteryLevel() int        { return 100 }
func (ws *Server) NetworkStatus() mosapp.NetworkStatus {
	return mosapp.NetworkOnline
}

func (ws *Server) SetTitle(appID, title string) {
	ws.serviceMu.Lock()
	if ws.titles == nil {
		ws.titles = make(map[string]string)
	}
	ws.titles[appID] = title
	ws.serviceMu.Unlock()
	ws.log("title " + appID + "=" + title)
}

func (ws *Server) SetBadge(appID string, count int) {
	ws.serviceMu.Lock()
	if ws.badges == nil {
		ws.badges = make(map[string]int)
	}
	if count <= 0 {
		delete(ws.badges, appID)
	} else {
		ws.badges[appID] = count
	}
	ws.serviceMu.Unlock()
	ws.log("badge " + appID + "=" + strconv.Itoa(count))
}

func (ws *Server) SetKeepScreenOn(appID string, enabled bool) {
	ws.serviceMu.Lock()
	if ws.keepAwake == nil {
		ws.keepAwake = make(map[string]bool)
	}
	ws.keepAwake[appID] = enabled
	ws.serviceMu.Unlock()
	ws.log("keep screen on " + appID + "=" + strconv.FormatBool(enabled))
}

func (ws *Server) SetPreferredOrientation(appID string, o mosapp.Orientation) {
	ws.serviceMu.Lock()
	if ws.orientations == nil {
		ws.orientations = make(map[string]mosapp.Orientation)
	}
	ws.orientations[appID] = o
	ws.serviceMu.Unlock()
	ws.log("preferred orientation " + appID)
}

func (ws *Server) SetBarStyle(appID, slot string, style mosapp.BarStyle) {
	key := appID + "/" + slot
	ws.serviceMu.Lock()
	if ws.barStyles == nil {
		ws.barStyles = make(map[string]mosapp.BarStyle)
	}
	ws.barStyles[key] = style
	ws.serviceMu.Unlock()
	ws.log("bar style " + key)
}

func (ws *Server) SetSystemBarsHidden(appID string, hidden bool) {
	ws.serviceMu.Lock()
	if ws.systemBarsHidden == nil {
		ws.systemBarsHidden = make(map[string]bool)
	}
	ws.systemBarsHidden[appID] = hidden
	ws.serviceMu.Unlock()
	ws.log("system bars hidden " + appID + "=" + strconv.FormatBool(hidden))
}

func (ws *Server) ShowToast(text string) {
	ws.log("toast: " + text)
	ws.PostNotice(mosapp.Notice{Title: "Toast", Body: text})
}

func (ws *Server) ScheduleNotice(appID, id string, n mosapp.Notice, at time.Time) {
	if id == "" {
		return
	}
	key := appID + "/" + id
	ws.serviceMu.Lock()
	if ws.scheduled == nil {
		ws.scheduled = make(map[string]scheduledNotice)
	}
	ws.scheduled[key] = scheduledNotice{AppID: appID, ID: id, Notice: n, At: at}
	ws.serviceMu.Unlock()
	ws.log("schedule notice " + key)
}

func (ws *Server) CancelNotice(appID, id string) {
	ws.serviceMu.Lock()
	delete(ws.scheduled, appID+"/"+id)
	ws.serviceMu.Unlock()
	ws.log("cancel notice " + appID + "/" + id)
}

func (ws *Server) IsDarkMode() bool {
	ws.serviceMu.RLock()
	defer ws.serviceMu.RUnlock()
	return ws.darkMode
}

func (ws *Server) SetDarkMode(enabled bool) {
	ws.serviceMu.Lock()
	if ws.darkMode == enabled {
		ws.serviceMu.Unlock()
		return
	}
	ws.darkMode = enabled
	ws.serviceMu.Unlock()
	ws.publish(event.System{Topic: event.TopicDarkMode, Value: enabled})
	ws.InvalidateAll()
	ws.log("dark mode=" + strconv.FormatBool(enabled))
}

func (ws *Server) OpenURL(rawURL string) {
	ws.log("open url: " + rawURL)
	ws.PostNotice(mosapp.Notice{Title: "Open URL", Body: rawURL})
}

func (ws *Server) ShareText(text string) {
	ws.log("share text")
	ws.PostNotice(mosapp.Notice{Title: "Share", Body: truncateForNotice(text)})
}

func truncateForNotice(s string) string {
	if len(s) <= 48 {
		return s
	}
	return s[:45] + "..."
}

func (ws *Server) CopyText(text string) {
	ws.serviceMu.Lock()
	ws.clipboard = text
	ws.serviceMu.Unlock()
	ws.log("clipboard copied")
}

func (ws *Server) ClipboardText() string {
	ws.serviceMu.RLock()
	defer ws.serviceMu.RUnlock()
	return ws.clipboard
}

func (ws *Server) DocumentsDir(appID string) string {
	return filepath.Join(os.TempDir(), "mos", "apps", sanitizePathPart(appID), "documents")
}

func (ws *Server) CacheDir(appID string) string {
	return filepath.Join(os.TempDir(), "mos", "apps", sanitizePathPart(appID), "cache")
}

func sanitizePathPart(s string) string {
	if s == "" {
		return "app"
	}
	return pathPartReplacer.Replace(s)
}

func (ws *Server) appFilePath(appID, name string) (string, error) {
	root := ws.DocumentsDir(appID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	clean := filepath.Clean(name)
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, "..") {
		clean = filepath.Base(clean)
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join(root, filepath.Base(name)), nil
	}
	return full, nil
}

func (ws *Server) ReadFile(appID, name string) ([]byte, error) {
	path, err := ws.appFilePath(appID, name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (ws *Server) WriteFile(appID, name string, data []byte) error {
	path, err := ws.appFilePath(appID, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (ws *Server) DeleteFile(appID, name string) error {
	path, err := ws.appFilePath(appID, name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (ws *Server) SaveFile(appID, name string, data []byte) (string, error) {
	root := filepath.Join(os.TempDir(), "mos", "exports", sanitizePathPart(appID))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.Base(name))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	ws.serviceMu.Lock()
	ws.latestExport = path
	ws.serviceMu.Unlock()
	ws.log("save file " + path)
	return path, nil
}

func (ws *Server) PickFile() (string, []byte, bool) {
	ws.serviceMu.RLock()
	path := ws.latestExport
	ws.serviceMu.RUnlock()
	if path == "" {
		return "", nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, false
	}
	return path, data, true
}

func (ws *Server) PickPhoto() (draws.Image, bool) {
	if len(ws.screenshots) == 0 {
		return draws.Image{}, false
	}
	return ws.screenshots[len(ws.screenshots)-1], true
}

func (ws *Server) CanLaunch(appID string) bool {
	return mosapp.Has(appID) || appID == AppIDColor
}

func (ws *Server) Preference(appID, key string) (string, bool) {
	ws.serviceMu.RLock()
	defer ws.serviceMu.RUnlock()
	v, ok := ws.preferences[appID+"/"+key]
	return v, ok
}

func (ws *Server) SetPreference(appID, key, value string) {
	ws.serviceMu.Lock()
	if ws.preferences == nil {
		ws.preferences = make(map[string]string)
	}
	ws.preferences[appID+"/"+key] = value
	ws.serviceMu.Unlock()
}

func (ws *Server) RemovePreference(appID, key string) {
	ws.serviceMu.Lock()
	delete(ws.preferences, appID+"/"+key)
	ws.serviceMu.Unlock()
}

func (ws *Server) PermissionStatus(p mosapp.Permission) mosapp.PermissionStatus {
	ws.serviceMu.RLock()
	defer ws.serviceMu.RUnlock()
	if ws.permissions == nil {
		return mosapp.PermissionNotDetermined
	}
	return ws.permissions[p]
}

func (ws *Server) RequestPermission(p mosapp.Permission) mosapp.PermissionStatus {
	ws.serviceMu.Lock()
	if ws.permissions == nil {
		ws.permissions = make(map[mosapp.Permission]mosapp.PermissionStatus)
	}
	status := ws.permissions[p]
	if status == mosapp.PermissionNotDetermined {
		status = mosapp.PermissionGranted
		ws.permissions[p] = status
	}
	ws.serviceMu.Unlock()
	ws.log("permission " + string(p) + "=" + permissionString(status))
	return status
}

func permissionString(s mosapp.PermissionStatus) string {
	switch s {
	case mosapp.PermissionGranted:
		return "granted"
	case mosapp.PermissionDenied:
		return "denied"
	default:
		return "not-determined"
	}
}

func (ws *Server) Vibrate(duration time.Duration) {
	ws.log("vibrate " + duration.String())
}

func (ws *Server) RequestAudioFocus(appID string) bool {
	ws.serviceMu.Lock()
	defer ws.serviceMu.Unlock()
	if ws.audioFocus != "" && ws.audioFocus != appID {
		return false
	}
	ws.audioFocus = appID
	ws.log("audio focus -> " + appID)
	return true
}

func (ws *Server) ReleaseAudioFocus(appID string) {
	ws.serviceMu.Lock()
	if ws.audioFocus == appID {
		ws.audioFocus = ""
	}
	ws.serviceMu.Unlock()
	ws.log("audio focus release " + appID)
}

func (ws *Server) HasAudioFocus(appID string) bool {
	ws.serviceMu.RLock()
	defer ws.serviceMu.RUnlock()
	return ws.audioFocus == appID
}

func (ws *Server) BeginBackgroundTask(appID, name string) string {
	ws.serviceMu.Lock()
	defer ws.serviceMu.Unlock()
	ws.bgSeq++
	token := appID + "/bg/" + strconv.Itoa(ws.bgSeq)
	if ws.bgTasks == nil {
		ws.bgTasks = make(map[string]string)
	}
	ws.bgTasks[token] = name
	ws.log("background begin " + token + " " + name)
	return token
}

func (ws *Server) EndBackgroundTask(token string) {
	ws.serviceMu.Lock()
	delete(ws.bgTasks, token)
	ws.serviceMu.Unlock()
	ws.log("background end " + token)
}

func (ws *Server) Announce(text string) {
	ws.log("announce: " + text)
	ws.PostNotice(mosapp.Notice{Title: "Accessibility", Body: text})
}

func (ws *Server) deliverScheduledNotices(now time.Time) {
	ws.serviceMu.Lock()
	var due []scheduledNotice
	for key, n := range ws.scheduled {
		if !n.At.IsZero() && !now.Before(n.At) {
			due = append(due, n)
			delete(ws.scheduled, key)
		}
	}
	ws.serviceMu.Unlock()
	for _, n := range due {
		ws.PostNotice(n.Notice)
		ws.log("scheduled notice " + n.AppID + "/" + n.ID)
	}
}

// Lock / Unlock delegate to the bound Lock implementation, if any.
// IsLocked reports the current lock state (false when no Lock is bound).
func (ws *Server) Lock() {
	if ws.lock != nil && !ws.lock.IsLocked() {
		ws.lock.Lock()
		ws.log("lock")
	}
}

func (ws *Server) Unlock() {
	if ws.lock != nil && ws.lock.IsLocked() {
		ws.lock.Unlock()
		ws.log("unlock")
	}
}

func (ws *Server) IsLocked() bool {
	return ws.lock != nil && ws.lock.IsLocked()
}

// PostNotice delivers a notice posted by an app to the curtain's notification
// list. Called on the main goroutine after the producing window's Update, so
// it is safe to mutate curtain state here.
func (ws *Server) PostNotice(n mosapp.Notice) {
	if ws.curtain != nil {
		ws.curtain.AddNotice(n)
	}
	ws.log("notice: " + n.Title + " - " + n.Body)
}

// grantFocus moves keyboard/IME focus to w, notifying the previous holder.
// Passing nil clears the focus slot entirely. Called from drain (on the main
// goroutine) in response to CmdRequestFocus, and automatically by the server
// when a window becomes the sole active window in fullscreen mode.
func (ws *Server) grantFocus(w *Window) {
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
func (ws *Server) releaseFocus(w *Window) {
	if ws.keyboardFocus == w {
		ws.grantFocus(nil)
	}
}

func (ws *Server) ToggleCurtain() {
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
func (ws *Server) EnterSplit() {
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
func (ws *Server) EnterPip() {
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
func (ws *Server) EnterFreeform() {
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
func (ws *Server) ExitMultiWindow() {
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
func (ws *Server) CycleFocus() {
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

func (ws *Server) SetScreenshots(shots []draws.Image) { ws.screenshots = shots }
func (ws *Server) Screenshots() []draws.Image         { return ws.screenshots }

func (ws *Server) AddScreenshot(src draws.Image) {
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
func (ws *Server) SafeArea() mosapp.SafeArea {
	var sa mosapp.SafeArea
	if ws.statusBar != nil {
		sa.Top = ws.statusBar.Height()
	}
	if ws.kb != nil {
		sa.Bottom = ws.kb.Height()
	}
	return sa
}

func (ws *Server) launchApp(iconPos, iconSize draws.XY, clr color.RGBA, appID string) {
	w := NewWindow(iconPos, iconSize, clr, appID, ws.ScreenW, ws.ScreenH, ws)
	ws.windows = append(ws.windows, w)
	ws.log("launch " + appID)
	ws.log("window " + w.AppID() + ": " + LifecycleInitializing.String() + " -> " + w.lifecycle.String())
	ws.publish(event.Lifecycle{AppID: appID, Phase: event.PhaseCreated})
}

// Launch opens an app from a neutral launcher position in the lower half of
// the screen. It is used by simulator scenarios and app Context.Launch.
func (ws *Server) Launch(appID string) {
	iconSize := draws.XY{X: ws.ScreenW * 0.18, Y: ws.ScreenW * 0.18}
	iconPos := draws.XY{X: ws.ScreenW / 2, Y: ws.ScreenH - iconSize.Y}
	ws.launchApp(iconPos, iconSize, appColor(appID), appID)
	ws.showingRecents = false
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Hide()
	}
}

func appColor(appID string) color.RGBA {
	switch appID {
	case AppIDGallery:
		return color.RGBA{0, 122, 255, 255}
	case AppIDSettings:
		return color.RGBA{95, 99, 110, 255}
	case AppIDCall:
		return color.RGBA{52, 199, 89, 255}
	case AppIDSceneTest:
		return color.RGBA{255, 149, 0, 255}
	case AppIDHello:
		return color.RGBA{88, 86, 214, 255}
	case AppIDShowcase:
		return color.RGBA{175, 82, 222, 255}
	case AppIDMessage:
		return color.RGBA{255, 45, 85, 255}
	default:
		return color.RGBA{50, 70, 110, 255}
	}
}

func (ws *Server) ReceiveCall() {
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

func (ws *Server) StartCall() { ws.ReceiveCall() }

func (ws *Server) RestoreActiveApp(state AppState) {
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

func (ws *Server) activeWindow() (*Window, bool) {
	for i := len(ws.windows) - 1; i >= 0; i-- {
		w := ws.windows[i]
		if w.lifecycle == LifecycleShown || w.lifecycle == LifecycleShowing {
			return w, true
		}
	}
	return nil, false
}

func (ws *Server) ActiveAppState() (AppState, bool) {
	w, ok := ws.activeWindow()
	if !ok {
		return AppState{}, false
	}
	return w.AppState(), true
}

func (ws *Server) hasVisibleWindow() bool {
	for _, w := range ws.windows {
		if w.lifecycle.Visible() {
			return true
		}
	}
	return false
}

func (ws *Server) dismissTopWindow() {
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

func (ws *Server) dismissTopWindowToCard() {
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

func (ws *Server) logLifecycleChange(w *Window, before Lifecycle) {
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

func (ws *Server) GoHome() {
	ws.showingRecents = false
	if ws.hist != nil && ws.hist.IsVisible() {
		ws.hist.Hide()
	}
	if w, ok := ws.activeWindow(); ok {
		if w.ctx.handleBack() {
			ws.log("back: handled by " + w.AppID())
			return
		}
		if ws.hist != nil {
			ws.hist.AddCard(w.HistoryEntry())
		}
		ws.dismissTopWindowToCard()
	}
	ws.publish(event.Navigation{Action: event.NavHome})
	ws.log("home")
}

func (ws *Server) GoBack() {
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

func (ws *Server) GoRecents() {
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
func (ws *Server) HistoryEntries() []apps.HistoryEntry {
	if ws.hist == nil {
		return nil
	}
	return ws.hist.Entries()
}

// Shutdown destroys every live window before discarding a Server instance
// (e.g. on display-mode change).
func (ws *Server) Shutdown() {
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

// InvalidateAll marks every shown window as dirty so that each app's Draw is
// called on the next frame regardless of whether it received input. Use this
// after a global state change such as a theme switch.
func (ws *Server) InvalidateAll() {
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
//  6. For each window: animate, update app content if needed, drain commands.
//  7. Purge fully hidden windows; check if multi-window anchors are still alive.
func (ws *Server) Update() {
	now := time.Now()
	ws.deliverScheduledNotices(now)
	events := ws.inputProducer.Poll()
	cx, cy := input.MouseCursorPosition()
	cursor := draws.XY{X: cx, Y: cy}
	locked := ws.IsLocked()

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
			if len(events) > 0 {
				ws.invalidateScene()
			}
			if pos, size, clr, appID, ok := ws.home.TappedIcon(); ok {
				if !ws.hasVisibleWindow() {
					ws.launchApp(pos, size, clr, appID)
				}
			}
		}
		if ws.hist != nil && ws.hist.IsVisible() {
			ws.hist.Update()
			if len(events) > 0 {
				ws.invalidateScene()
			}
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
		minute := time.Now().Format("15:04")
		if minute != ws.statusMinute {
			ws.statusMinute = minute
			ws.invalidateScene()
		}
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
		if len(events) > 0 {
			ws.invalidateScene()
		}
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

			// Implicit dirty tracking: only update app content when there is
			// something to react to.
			//
			//   hasEvents: pointer / touch input arrived at this window
			//   animActive: window layout transition is still in flight
			//   needsWakeup: app called ctx.Invalidate() from any goroutine
			//
			// Apps with animated widgets (e.g. toggle knob slide) should call
			// ctx.Invalidate() from Update while the animation is in-flight so
			// ticks continue until it completes.
			hasEvents := len(frame.Events) > 0
			animActive := !w.anim.Done()
			needsWakeup := w.ctx.invalidated.CompareAndSwap(true, false)
			timerDue := w.ctx.consumeWakeDue(now)

			if hasEvents || animActive || needsWakeup || timerDue {
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
			ws.Launch(id)
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

	// Purge fully hidden windows: fire OnDestroy, then remove.
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
// In fullscreen mode the active window receives only canvas-local events inside
// the screen bounds; in multi-window modes events are filtered to each window's
// display rect and transformed to canvas-local coordinates.
func (ws *Server) frameForWindow(w *Window, cursor draws.XY, events []input.Event, active *Window, gateBelow bool) mosapp.Frame {
	var windowEvents []input.Event

	if !gateBelow {
		if ws.mwm == nil || ws.mwm.mode == multiModeNone {
			// Fullscreen: only the topmost active window gets events inside
			// the phone canvas. Simulator chrome clicks live outside this rect
			// and must not leak into app content.
			if w == active {
				center := draws.XY{X: ws.ScreenW / 2, Y: ws.ScreenH / 2}
				size := draws.XY{X: ws.ScreenW, Y: ws.ScreenH}
				windowEvents = filterEventsByRect(events, center, size, w.captured)
			}
		} else {
			switch ws.mwm.mode {
			case multiModeSplit:
				// Both split windows receive events filtered to their rect.
				if w == ws.mwm.splitPrimary || w == ws.mwm.splitSecondary {
					windowEvents = filterEventsByRect(events, w.anim.Pos(), w.anim.Size(), w.captured)
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
					windowEvents = filterEventsByRect(events, w.anim.Pos(), w.anim.Size(), w.captured)
				}
			}
		}
	}

	// Fullscreen windows need no coordinate transform (canvas == screen).
	if w.mode == ModeFullscreen {
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
//	wallpaper -> home | recents -> windows ->
//	[pip border] -> [split divider] -> [freeform title bars] ->
//	keyboard -> status bar -> curtain -> lock
//
// Each Window's content.Draw is invoked here on the main goroutine after
// Update has run, so reading app state requires no mutex.
func (ws *Server) Draw(dst draws.Image) {
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
		if w.canvasDirty || !w.anim.Done() {
			ws.invalidateScene()
		}
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
func (ws *Server) blurredScene(dst draws.Image) draws.Image {
	size := dst.Size()

	if ws.blurSnap.IsEmpty() || ws.blurSnap.Size() != size {
		ws.blurSnap = draws.CreateImage(size.X, size.Y)
		ws.blurOut = draws.CreateImage(size.X, size.Y)
		ws.blurVersion = ws.sceneVersion - 1
	}
	if ws.sceneBlur == nil {
		ws.sceneBlur = draws.NewBlur(8) // factor-8: moderate frosted glass
	}
	if ws.blurVersion == ws.sceneVersion {
		return ws.blurOut
	}

	// Copy current dst pixels into the snapshot buffer.
	ws.blurSnap.Clear()
	ws.blurSnap.DrawImage(dst.Image, &ebiten.DrawImageOptions{})

	// Blur snapshot into blurOut.
	ws.blurOut.Clear()
	ws.sceneBlur.Apply(ws.blurSnap, ws.blurOut)
	ws.blurVersion = ws.sceneVersion

	return ws.blurOut
}
