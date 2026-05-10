package main

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/apps"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/tween"
	"github.com/hndada/mos/internal/windowing"
	"github.com/hndada/mos/ui"
	uithem "github.com/hndada/mos/ui/theme"
)

const (
	simW = 1200
	simH = 900
)

// Display mode.

type displayMode int

const (
	displayModeBar  displayMode = iota // standard slab phone
	displayModeFlip                    // clamshell: outer cover + inner main
	displayModeFold                    // foldable: two halves side by side
	displayModeCount
)

func (m displayMode) String() string {
	switch m {
	case displayModeBar:
		return "bar"
	case displayModeFlip:
		return "flip"
	case displayModeFold:
		return "fold"
	default:
		return "unknown"
	}
}

// screenSpec describes one physical screen's size and position in the sim viewport.
type screenSpec struct {
	w, h    float64
	x, y    float64
	primary bool // the windowing server renders into the primary screen
}

// displayGroup is the set of screen specs for one device mode.
type displayGroup []screenSpec

// groups returns all three display groups, centred inside the sim viewport.
// S26, Flip 7, Fold 7 scaled to fit the simulator viewport with chrome.
func groups() [displayModeCount]displayGroup {
	type WH struct{ W, H float64 }
	s26 := WH{1440, 3200}
	flip7Cover := WH{1048, 948}
	flip7Main := WH{1080, 2640}
	fold7Cover := WH{904, 2316}
	fold7Main := WH{1812, 2176}

	const sc = 0.26
	const gap = 32.0

	barW, barH := s26.W*sc, s26.H*sc
	fcW, fcH := flip7Cover.W*sc, flip7Cover.H*sc
	fmW, fmH := flip7Main.W*sc, flip7Main.H*sc
	fdcW, fdcH := fold7Cover.W*sc, fold7Cover.H*sc
	fdmW, fdmH := fold7Main.W*sc, fold7Main.H*sc

	place := placeDisplayGroup

	// Flip: cover and main side by side horizontally with a gap.
	flipTotalW := fcW + gap + fmW
	flipGroupH := max(fcH, fmH)
	flipX, flipY := place(flipTotalW, flipGroupH)
	flipCoverY := flipY + flipGroupH - fcH

	// Fold: cover and main side by side horizontally with a gap.
	foldTotalW := fdcW + gap + fdmW
	foldGroupH := max(fdcH, fdmH)
	foldX, foldY := place(foldTotalW, foldGroupH)

	barX, barY := place(barW, barH)

	return [displayModeCount]displayGroup{
		displayModeBar: {
			{w: barW, h: barH, x: barX, y: barY, primary: true},
		},
		displayModeFlip: {
			{w: fcW, h: fcH, x: flipX, y: flipCoverY},
			{w: fmW, h: fmH, x: flipX + fcW + gap, y: flipY, primary: true},
		},
		displayModeFold: {
			{w: fdcW, h: fdcH, x: foldX, y: foldY},
			{w: fdmW, h: fdmH, x: foldX + fdcW + gap, y: foldY, primary: true},
		},
	}
}

// rotateSpec swaps the screen's w and h while keeping its visual center fixed.
// Used to simulate device rotation: a portrait slot becomes a landscape slot
// pivoting on its own midpoint.
func rotateSpec(s screenSpec) screenSpec {
	cx := s.x + s.w/2
	cy := s.y + s.h/2
	return screenSpec{
		w:       s.h,
		h:       s.w,
		x:       cx - s.h/2,
		y:       cy - s.w/2,
		primary: s.primary,
	}
}

func placeDisplayGroup(w, h float64) (float64, float64) {
	left := logX + logW + logGap
	right := actionPanelLeft() - logGap
	if right-left >= w {
		return left + (right-left-w)/2, (simH - h) / 2
	}

	centerX := func(width float64) float64 { return (simW - width) / 2 }
	centerY := func(height float64) float64 { return (simH - height) / 2 }
	overlapsReserved := func(x, y float64) bool {
		overlapsLog := x < logX+logW+logGap &&
			x+w > logX &&
			y < logY+logH+logGap &&
			y+h > logY
		overlapsControls := x+w > actionPanelLeft()-logGap &&
			x < simW &&
			y < actionPanelBottom()+logGap &&
			y+h > actionPanelMargin
		return overlapsLog || overlapsControls
	}

	x, y := centerX(w), centerY(h)
	if !overlapsReserved(x, y) {
		return x, y
	}

	rightX := logX + logW + logGap
	if rightX+w <= actionPanelLeft()-logGap {
		return rightX, centerY(h)
	}

	belowY := logY + logH + logGap
	if belowY+h <= simH-12 {
		return centerX(w), belowY
	}

	return x, y
}

// Pre-built per-group visuals.

const borderPx = 8.0

type screenSlot struct {
	screenSpec
	bg     draws.Image // shown on secondary (non-primary) screens
	border draws.Image // per-screen bezel
}

type simGroup struct {
	slots []screenSlot
}

func buildGroup(group displayGroup) simGroup {
	slots := make([]screenSlot, len(group))
	for i, sp := range group {
		bg := draws.CreateImage(sp.w, sp.h)
		bg.Fill(color.RGBA{10, 10, 10, 255})
		brd := draws.CreateImage(sp.w+2*borderPx, sp.h+2*borderPx)
		brd.Fill(color.RGBA{72, 72, 74, 255})
		slots[i] = screenSlot{screenSpec: sp, bg: bg, border: brd}
	}
	return simGroup{slots: slots}
}

func primaryIndex(group displayGroup) int {
	for i, sp := range group {
		if sp.primary {
			return i
		}
	}
	return 0
}

func historyAspectSpec(mode displayMode, group displayGroup) screenSpec {
	switch mode {
	case displayModeFlip:
		return group[primaryIndex(group)]
	case displayModeFold:
		return group[0]
	default:
		return group[primaryIndex(group)]
	}
}

// Simulator.

// Simulator drives the OS loop and emulates physical hardware buttons:
//
//	F1: power (toggle display on/off)
//	F2: cycle device mode: bar, flip, fold
//	F3: cycle active display within the current device mode
type simulator struct {
	mode              displayMode
	activeDisplay     int
	rotated           bool // landscape: active display's w/h swapped
	groups            [displayModeCount]displayGroup
	simGroup          simGroup
	canvas            draws.Image // render target for the active display
	ws                windowing.Server
	display           windowing.Display
	help              draws.Text
	log               simLog
	shell             simShell
	powerMenu         simPowerMenu
	historyEntries    []apps.HistoryEntry // persisted across screen/mode changes
	screenshots       []draws.Image       // persisted in memory across screen/mode changes
	captureNext       bool
	controlPanelBg    draws.Sprite
	backButton        ui.TriggerButton
	homeButton        ui.TriggerButton
	recentsButton     ui.TriggerButton
	backButtonBg      draws.Sprite
	homeButtonBg      draws.Sprite
	recentsButtonBg   draws.Sprite
	backButtonText    draws.Text
	homeButtonText    draws.Text
	recentsButtonText draws.Text
	navGesture        ui.GestureDetector

	// Sim-level action panel: one tappable button per supported action.
	// Lives in raw viewport coords (not on the phone canvas) and operates
	// regardless of display power, so users can power on with a click.
	actions      []simAction
	actionGroups []simActionGroup

	// AOD renders a low-power clock when the display is "powered off."
	aod        *apps.DefaultAOD
	aodEnabled bool
	volume     int
	volumeShow time.Time

	// bus is the current OS-wide event bus (recreated on applyMode).
	bus *event.Bus
	// isDark tracks the current theme so the "Dark" action can toggle it.
	isDark bool

	// Per-frame input producers. Sim chrome buttons live in raw window-space;
	// the nav-swipe lives in canvas-space. Each producer tracks its own
	// lastPos in its own coord system. The windowing server has its own
	// canvas-space producer in addition to navInput; both can poll the same
	// frame's Ebiten state without conflict (inpututil's JustPressed is a
	// frame-level boolean, not a one-shot consumer).
	rawInput input.Producer
	navInput input.Producer

	rotateAnim rotateAnimation
}

const helpString = "`: Shell   P: Power   X: Lock   1/2/3: Bar-Flip-Fold   S: Screen   O: Rotate   B/Esc: Back   N: Curtain   K: Keys   V: Ring   W: Split   I: PiP   G: Float   Tab: Focus   L/F: Log"

type simAction struct {
	Label   string
	Group   string
	Keys    []input.Key
	Handler func()
}

type simActionGroup struct {
	title draws.Text
	panel ui.ControlPanel
}

type rotateAnimation struct {
	active   bool
	snapshot draws.Image
	clip     draws.Image
	p        tween.Transition
	dir      float64
}

const (
	controlPanelH = 38.0
	logMaxLines   = 10
	logHistoryMax = 200
	logX          = 12.0
	logY          = 12.0
	logW          = 340.0
	logH          = 220.0
	logGap        = 28.0
)

const rotateAnimDuration = 320 * time.Millisecond

type simLog struct {
	bg      draws.Sprite
	lines   []draws.Text
	entries []simLogEntry
	visible bool
}

type simLogEntry struct {
	stamp  string
	msg    string
	repeat int
}

type simShell struct {
	active bool
	input  string
	bg     draws.Sprite
	prompt draws.Text
}

type simPowerMenu struct {
	visible bool
	bg      draws.Sprite
	title   draws.Text
	buttons []simPowerButton
}

type simPowerButton struct {
	label string
	btn   ui.TriggerButton
	text  draws.Text
}

func newSimLog() simLog {
	img := draws.CreateImage(logW, logH)
	img.Fill(color.RGBA{0, 0, 0, 150})
	bg := draws.NewSprite(img)
	bg.Locate(logX, logY, draws.LeftTop)

	opts := draws.NewFaceOptions()
	opts.Size = 12
	lines := make([]draws.Text, logMaxLines)
	for i := range lines {
		lines[i] = draws.NewText("")
		lines[i].SetFace(opts)
		lines[i].ColorScale.Scale(1, 1, 1, 1)
		lines[i].Locate(logX+12, logY+12+float64(i)*20, draws.LeftTop)
	}

	return simLog{
		bg:      bg,
		lines:   lines,
		visible: true,
	}
}

func newSimShell() simShell {
	img := draws.CreateImage(simW-40, 34)
	img.Fill(color.RGBA{6, 8, 12, 230})
	bg := draws.NewSprite(img)
	bg.Locate(20, simH-56, draws.LeftTop)

	opts := draws.NewFaceOptions()
	opts.Size = 14
	prompt := draws.NewText("")
	prompt.SetFace(opts)
	prompt.Locate(32, simH-47, draws.LeftTop)
	return simShell{bg: bg, prompt: prompt}
}

func newSimPowerMenu() simPowerMenu {
	w, h := 280.0, 214.0
	x, y := (simW-w)/2, (simH-h)/2
	bg := draws.CreateImage(w, h)
	bg.Fill(color.RGBA{12, 14, 20, 238})
	bgSp := draws.NewSprite(bg)
	bgSp.Locate(x, y, draws.LeftTop)

	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 18
	title := draws.NewText("Power")
	title.SetFace(titleOpts)
	title.Locate(x+w/2, y+28, draws.CenterMiddle)

	labels := []string{"Lock", "Sleep", "Restart", "Cancel"}
	buttons := make([]simPowerButton, len(labels))
	opts := draws.NewFaceOptions()
	opts.Size = 13
	for i, label := range labels {
		by := y + 58 + float64(i)*36
		txt := draws.NewText(label)
		txt.SetFace(opts)
		txt.Locate(x+w/2, by+15, draws.CenterMiddle)
		buttons[i] = simPowerButton{
			label: label,
			btn:   ui.NewTriggerButton(x+34, by, w-68, 30),
			text:  txt,
		}
	}
	return simPowerMenu{bg: bgSp, title: title, buttons: buttons}
}

func (pm *simPowerMenu) Draw(dst draws.Image) {
	if !pm.visible {
		return
	}
	pm.bg.Draw(dst)
	pm.title.Draw(dst)
	for i := range pm.buttons {
		b := &pm.buttons[i]
		img := draws.CreateImage(b.btn.W(), b.btn.H())
		img.Fill(color.RGBA{255, 255, 255, 32})
		sp := draws.NewSprite(img)
		sp.Locate(b.btn.X(), b.btn.Y(), draws.LeftTop)
		sp.Draw(dst)
		b.text.Draw(dst)
	}
}

func (sh *simShell) Toggle() {
	sh.active = !sh.active
}

func (sh *simShell) Draw(dst draws.Image) {
	if !sh.active {
		return
	}
	sh.bg.Draw(dst)
	sh.prompt.Text = "mos> " + sh.input
	sh.prompt.Draw(dst)
}

func (l *simLog) Add(msg string) {
	stamp := time.Now().Format("15:04:05.000")
	fmt.Println(stamp + "  " + msg)

	last := len(l.entries) - 1
	if last >= 0 && l.entries[last].msg == msg {
		l.entries[last].stamp = stamp
		l.entries[last].repeat++
		l.refreshLines()
		return
	}

	l.entries = append(l.entries, simLogEntry{
		stamp:  stamp,
		msg:    msg,
		repeat: 1,
	})
	if len(l.entries) > logHistoryMax {
		copy(l.entries, l.entries[len(l.entries)-logHistoryMax:])
		l.entries = l.entries[:logHistoryMax]
	}

	l.refreshLines()
}

func (l *simLog) Toggle() {
	l.visible = !l.visible
}

func (l *simLog) Clear() {
	l.entries = l.entries[:0]
	l.refreshLines()
}

func (l *simLog) refreshLines() {
	for i := range l.lines {
		l.lines[i].Text = ""
	}
	if len(l.entries) == 0 {
		return
	}
	first := len(l.entries) - logMaxLines
	if first < 0 {
		first = 0
	}
	lineStart := logMaxLines - (len(l.entries) - first)
	for i, entry := range l.entries[first:] {
		l.lines[lineStart+i].Text = trimLogLine(entry.String(), 50)
	}
}

func (e simLogEntry) String() string {
	line := e.stamp + "  " + e.msg
	if e.repeat > 1 {
		line += "  x" + strconv.Itoa(e.repeat)
	}
	return line
}

func trimLogLine(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	if limit <= 3 {
		return string(r[:limit])
	}
	return string(r[:limit-3]) + "..."
}

func (l *simLog) Draw(dst draws.Image) {
	if !l.visible {
		return
	}
	l.bg.Draw(dst)
	for _, line := range l.lines {
		line.Draw(dst)
	}
}

func newSimulator() *simulator {
	s := &simulator{isDark: true, aodEnabled: true, volume: 7} // theme.Dark() is active at startup
	s.groups = groups()
	s.activeDisplay = primaryIndex(s.groups[s.mode])

	opts := draws.NewFaceOptions()
	opts.Size = 13
	t := draws.NewText(helpString)
	t.SetFace(opts)
	t.Locate(simW/2, simH-8, draws.CenterBottom)
	s.help = t
	s.log = newSimLog()
	s.shell = newSimShell()
	s.powerMenu = newSimPowerMenu()
	s.logf("simulator boot")

	s.actions = s.buildActions()
	s.buildActionPanel()
	s.applyMode()
	return s
}

const actionPanelMargin = 10.0
const actionPanelCols = 2
const actionPanelTitleGap = 3.0
const actionPanelSectionGap = 7.0

func actionPanelWidth() float64 {
	return ui.ControlPanelButtonW*float64(actionPanelCols) +
		ui.ControlPanelGap*float64(actionPanelCols-1) + ui.ControlPanelPad*2
}

func actionPanelLeft() float64 {
	return simW - actionPanelWidth() - actionPanelMargin
}

func actionPanelBottom() float64 {
	groupOrder := []string{"Scenario", "Device", "Form", "Display", "Navigation", "System", "Windowing", "Debug"}
	rowsByGroup := map[string]int{
		"Scenario":   2,
		"Device":     2,
		"Form":       2,
		"Display":    1,
		"Navigation": 2,
		"System":     3,
		"Windowing":  2,
		"Debug":      1,
	}
	titleH := 12.0
	y := actionPanelMargin
	for _, group := range groupOrder {
		rows := rowsByGroup[group]
		panelH := ui.ControlPanelButtonH*float64(rows) +
			ui.ControlPanelGap*float64(rows-1) + ui.ControlPanelPad*2
		y += titleH + actionPanelTitleGap + panelH + actionPanelSectionGap
	}
	return y
}

func (s *simulator) buildActions() []simAction {
	return []simAction{
		{Label: "Split Msg", Group: "Scenario", Handler: s.scenarioSplitMessages},
		{Label: "Chat Keys", Group: "Scenario", Handler: s.scenarioMessageKeyboard},
		{Label: "Curtain", Group: "Scenario", Handler: s.scenarioCurtainNotices},
		{Label: "Recents", Group: "Scenario", Handler: s.scenarioRecentsStack},
		{Label: "Power", Group: "Device", Keys: []input.Key{input.KeyP}, Handler: s.togglePower},
		{Label: "Lock", Group: "Device", Keys: []input.Key{input.KeyX}, Handler: s.toggleLock},
		{Label: "Menu", Group: "Device", Keys: []input.Key{input.KeyF12}, Handler: s.togglePowerMenu},
		{Label: "Shot", Group: "Device", Keys: []input.Key{input.KeyC, input.KeyPrintScreen}, Handler: s.requestScreenshot},
		{Label: "Bar", Group: "Form", Keys: []input.Key{input.KeyDigit1}, Handler: func() { s.setMode(displayModeBar) }},
		{Label: "Flip", Group: "Form", Keys: []input.Key{input.KeyDigit2}, Handler: func() { s.setMode(displayModeFlip) }},
		{Label: "Fold", Group: "Form", Keys: []input.Key{input.KeyDigit3}, Handler: func() { s.setMode(displayModeFold) }},
		{Label: "Screen", Group: "Display", Keys: []input.Key{input.KeyS}, Handler: s.cycleActiveDisplay},
		{Label: "Rotate", Group: "Display", Keys: []input.Key{input.KeyO}, Handler: s.rotateScreen},
		{Label: "Back", Group: "Navigation", Keys: []input.Key{input.KeyB, input.KeyEscape}, Handler: func() { s.ws.GoBack() }},
		{Label: "Home", Group: "Navigation", Keys: []input.Key{input.KeyH}, Handler: func() { s.ws.GoHome() }},
		{Label: "Recents", Group: "Navigation", Keys: []input.Key{input.KeyR}, Handler: func() { s.ws.GoRecents() }},
		{Label: "Curtain", Group: "System", Keys: []input.Key{input.KeyN}, Handler: func() { s.ws.ToggleCurtain() }},
		{Label: "Keys", Group: "System", Keys: []input.Key{input.KeyK}, Handler: func() { s.ws.ToggleKeyboard() }},
		{Label: "Ring", Group: "System", Keys: []input.Key{input.KeyV}, Handler: func() { s.ws.ReceiveCall() }},
		{Label: "Dark", Group: "System", Handler: func() { s.ws.SetDarkMode(!s.isDark) }},
		{Label: "Vol-", Group: "System", Keys: []input.Key{input.KeyMinus}, Handler: func() { s.adjustVolume(-1) }},
		{Label: "Vol+", Group: "System", Keys: []input.Key{input.KeyEqual}, Handler: func() { s.adjustVolume(1) }},
		{Label: "Split", Group: "Windowing", Keys: []input.Key{input.KeyW}, Handler: func() { s.ws.EnterSplit() }},
		{Label: "PiP", Group: "Windowing", Keys: []input.Key{input.KeyI}, Handler: func() { s.ws.EnterPip() }},
		{Label: "Float", Group: "Windowing", Keys: []input.Key{input.KeyG}, Handler: func() { s.ws.EnterFreeform() }},
		{Label: "Focus", Group: "Windowing", Keys: []input.Key{input.KeyTab}, Handler: func() { s.ws.CycleFocus() }},
		{Label: "Log", Group: "Debug", Keys: []input.Key{input.KeyL}, Handler: s.toggleLog},
		{Label: "Clear", Group: "Debug", Keys: []input.Key{input.KeyF}, Handler: s.clearLog},
	}
}

// buildActionPanel constructs right-side ui.ControlPanels from the same
// action table used by keyboard shortcuts, keeping click and key paths in sync.
func (s *simulator) buildActionPanel() {
	// Anchor the panel flush with the right viewport edge. We only need its
	// width to do that; the panel itself derives its full footprint internally.
	x := actionPanelLeft()
	y := actionPanelMargin

	groupOrder := []string{"Scenario", "Device", "Form", "Display", "Navigation", "System", "Windowing", "Debug"}
	s.actionGroups = s.actionGroups[:0]
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 10

	for _, group := range groupOrder {
		actions := s.controlActionsForGroup(group)
		if len(actions) == 0 {
			continue
		}

		title := draws.NewText(group)
		title.SetFace(titleOpts)
		title.ColorScale.Scale(1, 1, 1, 1)
		title.Locate(x, y, draws.LeftTop)
		y += title.Size().Y + actionPanelTitleGap

		panel := ui.NewControlPanel(x, y, actionPanelCols, actions)
		y += panel.Size().Y + actionPanelSectionGap
		s.actionGroups = append(s.actionGroups, simActionGroup{title: title, panel: panel})
	}
}

func (s *simulator) controlActionsForGroup(group string) []ui.ControlAction {
	var actions []ui.ControlAction
	for _, a := range s.actions {
		if a.Group != group {
			continue
		}
		actions = append(actions, ui.ControlAction{Label: a.Label, Handler: a.Handler})
	}
	return actions
}

func (s *simulator) logf(format string, args ...any) {
	s.log.Add(fmt.Sprintf(format, args...))
}

func (s *simulator) logLine(msg string) {
	s.log.Add(msg)
}

func (s *simulator) applyMode() {
	activeApp, hasActiveApp := s.ws.ActiveAppState()
	// Persist history entries before tearing down the old windowing server.
	s.historyEntries = s.ws.HistoryEntries()
	if shots := s.ws.Screenshots(); shots != nil {
		s.screenshots = shots
	}
	// Tear down the old server's per-window goroutines so they don't leak.
	s.ws.Shutdown()

	group := s.effectiveGroup()
	if s.activeDisplay >= len(group) {
		s.activeDisplay = primaryIndex(group)
	}
	active := group[s.activeDisplay]

	s.canvas = draws.CreateImage(active.w, active.h)
	bus := event.NewBus()
	s.bus = bus
	s.ws = windowing.Server{ScreenW: active.w, ScreenH: active.h, Bus: bus}

	// Subscribe to system setting events so Settings, curtain tiles, and the
	// simulator action panel all funnel through the same path.
	bus.Subscribe(event.KindSystem, func(e event.Event) {
		sys, ok := e.(event.System)
		if !ok {
			return
		}
		switch sys.Topic {
		case event.TopicDarkMode:
			dark, ok := sys.Value.(bool)
			if !ok {
				return
			}
			s.isDark = dark
			if dark {
				uithem.Set(uithem.Dark())
			} else {
				uithem.Set(uithem.Light())
			}
			s.ws.InvalidateAll()
		case event.TopicAOD:
			enabled, ok := sys.Value.(bool)
			if !ok {
				return
			}
			s.aodEnabled = enabled
			s.logf("aod=%v", enabled)
		}
	})
	s.ws.SetDarkMode(s.isDark)
	s.ws.SetLogger(s.logLine)
	s.ws.SetScreenshots(s.screenshots)
	s.ws.SetWallpaper(apps.NewDefaultWallpaper(active.w, active.h))
	s.ws.SetHome(apps.NewDefaultHome(active.w, active.h))

	historyAspect := historyAspectSpec(s.mode, group)
	hist := apps.NewDefaultHistoryWithCardAspect(active.w, active.h, historyAspect.w, historyAspect.h)
	// Restore saved app history newest-last so AddCard (which prepends)
	// rebuilds the slice with index 0 = newest.
	for i := len(s.historyEntries) - 1; i >= 0; i-- {
		hist.AddCard(s.historyEntries[i])
	}
	s.ws.SetHistory(hist)
	if hasActiveApp {
		s.ws.RestoreActiveApp(activeApp)
	}

	s.ws.SetStatusBar(apps.NewDefaultStatusBar(active.w, active.h))
	curtain := apps.NewDefaultCurtain(active.w, active.h, bus)
	curtain.SubscribeBus()
	bus.Publish(event.System{Topic: event.TopicDarkMode, Value: s.isDark})
	bus.Publish(event.System{Topic: event.TopicAOD, Value: s.aodEnabled})
	s.ws.SetCurtain(curtain)
	s.ws.SetKeyboard(apps.NewDefaultKeyboard(active.w, active.h))
	s.ws.SetLock(apps.NewDefaultLock(active.w, active.h))
	s.aod = apps.NewDefaultAOD(active.w, active.h)
	s.display.W = active.w
	s.display.H = active.h
	s.display.SetPowered(true)
	s.simGroup = buildGroup(group)
	s.layoutTriggers(active)
	ebiten.SetWindowTitle("MOS Simulator - " + s.mode.String())
	s.logf("display=%s active=%d %.0fx%.0f", s.mode.String(), s.activeDisplay, active.w, active.h)
}

func (s *simulator) layoutTriggers(active screenSpec) {
	const (
		buttonH = 26.0
		gap     = 4.0
		margin  = 5.0
	)
	buttonW := min(62, (active.w-margin*2-gap*2)/3)
	totalW := buttonW*3 + gap*2
	panelW := totalW + margin*2
	panelH := controlPanelH
	x := active.x + (active.w-totalW)/2
	if x < active.x+margin {
		x = active.x + margin
	}
	if x+totalW > active.x+active.w-margin {
		x = active.x + active.w - margin - totalW
	}
	y := active.y + active.h - panelH - margin + (panelH-buttonH)/2
	if x < 0 {
		x = 0
	}

	if s.controlPanelBg.Source.IsEmpty() || s.controlPanelBg.Size.X != panelW || s.controlPanelBg.Size.Y != panelH {
		img := draws.CreateImage(panelW, panelH)
		img.Fill(color.RGBA{0, 0, 0, 150})
		s.controlPanelBg = draws.NewSprite(img)
	}
	s.controlPanelBg.Locate(x-margin, y-(panelH-buttonH)/2, draws.LeftTop)

	step := buttonW + gap
	s.recentsButton.SetRect(x, y, buttonW, buttonH)
	s.homeButton.SetRect(x+step, y, buttonW, buttonH)
	s.backButton.SetRect(x+step*2, y, buttonW, buttonH)
	s.layoutTriggerVisuals()
	s.navGesture = ui.NewGestureDetector(0, active.h-80, active.w, 80)
	s.navGesture.MinSwipePx = ui.HomeSwipeMinPx
}

func (s *simulator) layoutTriggerVisuals() {
	s.configureTriggerVisuals(&s.recentsButtonBg, &s.recentsButtonText, s.recentsButton.Box, "[ ]")
	s.configureTriggerVisuals(&s.homeButtonBg, &s.homeButtonText, s.homeButton.Box, "Home")
	s.configureTriggerVisuals(&s.backButtonBg, &s.backButtonText, s.backButton.Box, "< Back")
}

func (s *simulator) configureTriggerVisuals(bg *draws.Sprite, txt *draws.Text, box ui.Box, label string) {
	if bg.Source.IsEmpty() || bg.Size != box.Size {
		img := draws.CreateImage(box.W(), box.H())
		img.Fill(color.RGBA{255, 255, 255, 48})
		*bg = draws.NewSprite(img)
	}
	bg.Position = box.Position
	bg.Size = box.Size
	bg.Aligns = box.Aligns

	if txt.Text == "" {
		opts := draws.NewFaceOptions()
		opts.Size = 10
		*txt = draws.NewText("")
		txt.SetFace(opts)
		txt.ColorScale.Scale(0.07, 0.07, 0.07, 1)
	}
	txt.Text = label
	txt.Locate(box.Position.X+box.W()/2, box.Position.Y+box.H()/2, draws.CenterMiddle)
}

func (s *simulator) setMode(m displayMode) {
	if s.mode == m {
		return
	}
	s.logf("mode change %s -> %s", s.mode.String(), m.String())
	s.mode = m
	s.activeDisplay = primaryIndex(s.groups[s.mode])
	s.rotated = false
	s.applyMode()
}

func (s *simulator) cycleActiveDisplay() {
	group := s.groups[s.mode]
	if len(group) <= 1 {
		s.logf("active display unchanged")
		return
	}
	s.activeDisplay = (s.activeDisplay + 1) % len(group)
	s.rotated = false
	s.logf("active display -> %d", s.activeDisplay)
	s.applyMode()
}

// effectiveGroup returns the active mode's display group with rotation applied
// to the active display when s.rotated is true. The original groups slice is
// not mutated, so toggling rotation off restores the natural layout.
func (s *simulator) effectiveGroup() displayGroup {
	group := s.groups[s.mode]
	if !s.rotated {
		return group
	}
	out := make(displayGroup, len(group))
	copy(out, group)
	idx := s.activeDisplay
	if idx >= len(out) {
		idx = primaryIndex(out)
	}
	out[idx] = rotateSpec(out[idx])
	return out
}

// rotateScreen toggles the active display between portrait and landscape and
// rebuilds the windowing server (which preserves the active app and history).
func (s *simulator) rotateScreen() {
	snapshot := s.snapshotCanvas()
	dir := 1.0
	s.rotated = !s.rotated
	if s.rotated {
		s.logf("rotate -> landscape")
	} else {
		s.logf("rotate -> portrait")
		dir = -1
	}
	s.applyMode()
	s.startRotateAnimation(snapshot, dir)
}

func (s *simulator) snapshotCanvas() draws.Image {
	if s.canvas.IsEmpty() {
		return draws.Image{}
	}
	size := s.canvas.Size()
	img := draws.CreateImage(size.X, size.Y)
	img.DrawImage(s.canvas.Image, &ebiten.DrawImageOptions{})
	return img
}

func (s *simulator) startRotateAnimation(snapshot draws.Image, dir float64) {
	if snapshot.IsEmpty() {
		return
	}
	s.rotateAnim = rotateAnimation{
		active:   true,
		snapshot: snapshot,
		dir:      dir,
	}
	s.rotateAnim.p.Snap(0)
	s.rotateAnim.p.To(1, rotateAnimDuration, tween.EaseOutExponential)
}

func (s *simulator) toggleLog() {
	s.log.Toggle()
}

func (s *simulator) clearLog() {
	s.log.Clear()
}

func (s *simulator) togglePowerMenu() {
	s.powerMenu.visible = !s.powerMenu.visible
	s.logf("power menu=%v", s.powerMenu.visible)
}

func (s *simulator) updatePowerMenu(frame mosapp.Frame) bool {
	if !s.powerMenu.visible {
		return false
	}
	for i := range s.powerMenu.buttons {
		b := &s.powerMenu.buttons[i]
		if !b.btn.Update(frame) {
			continue
		}
		switch b.label {
		case "Lock":
			s.ws.Lock()
		case "Sleep":
			if s.display.Powered() {
				s.togglePower()
			}
		case "Restart":
			s.logf("restart")
			s.applyMode()
		case "Cancel":
		}
		s.powerMenu.visible = false
		return true
	}
	for _, ev := range frame.Events {
		if ev.Kind == input.EventDown {
			return true
		}
	}
	return true
}

func (s *simulator) adjustVolume(delta int) {
	s.volume = max(0, min(10, s.volume+delta))
	s.volumeShow = time.Now().Add(1400 * time.Millisecond)
	s.logf("volume=%d", s.volume)
}

func (s *simulator) updateShell() bool {
	if input.IsKeyJustPressed(input.KeyBackquote) {
		s.shell.Toggle()
		return true
	}
	if !s.shell.active {
		return false
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r == '`' {
			continue
		}
		s.shell.input += string(r)
	}
	if input.IsKeyJustPressed(input.KeyBackspace) {
		runes := []rune(s.shell.input)
		if len(runes) > 0 {
			s.shell.input = string(runes[:len(runes)-1])
		}
	}
	if input.IsKeyJustPressed(input.KeyEscape) {
		s.shell.active = false
		return true
	}
	if input.IsKeyJustPressed(input.KeyEnter) {
		cmd := strings.TrimSpace(s.shell.input)
		s.shell.input = ""
		if cmd != "" {
			s.runShellCommand(cmd)
		}
	}
	return true
}

func (s *simulator) runShellCommand(cmd string) {
	s.logf("$ %s", cmd)
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return
	}
	switch fields[0] {
	case "help":
		s.logf("shell: help launch <id> back home recents curtain lock unlock power menu|sleep|restart split pip float shot dump windows secure on|off crash volume <0..10> logs clear")
	case "launch":
		if len(fields) < 2 {
			s.logf("shell: launch requires app id")
			return
		}
		if !s.ws.CanLaunch(fields[1]) {
			s.logf("shell: unknown app %s", fields[1])
			return
		}
		s.ws.Launch(fields[1])
	case "back":
		s.ws.GoBack()
	case "home":
		s.ws.GoHome()
	case "recents":
		s.ws.GoRecents()
	case "curtain":
		s.ws.ToggleCurtain()
	case "lock":
		s.ws.Lock()
	case "unlock":
		s.ws.Unlock()
	case "power":
		if len(fields) < 2 || fields[1] == "menu" {
			s.powerMenu.visible = true
			return
		}
		switch fields[1] {
		case "sleep", "off":
			if s.display.Powered() {
				s.togglePower()
			}
		case "wake", "on":
			if !s.display.Powered() {
				s.togglePower()
			}
		case "restart":
			s.logf("restart")
			s.applyMode()
		default:
			s.logf("shell: power menu|sleep|wake|restart")
		}
	case "split":
		s.ws.EnterSplit()
	case "pip":
		s.ws.EnterPip()
	case "float":
		s.ws.EnterFreeform()
	case "shot", "screenshot":
		s.requestScreenshot()
	case "dump":
		if len(fields) > 1 && fields[1] == "windows" {
			for _, line := range s.ws.DumpWindows() {
				s.logLine(line)
			}
			return
		}
		s.logf("shell: try dump windows")
	case "secure":
		if len(fields) < 2 {
			s.logf("shell: secure on|off")
			return
		}
		enabled := fields[1] == "on" || fields[1] == "true" || fields[1] == "1"
		if !s.ws.SetActiveSecureContent(enabled) {
			s.logf("shell: no active window")
		}
	case "crash":
		if !s.ws.CrashActiveApp("shell crash") {
			s.logf("shell: no active window")
		}
	case "logs":
		if len(fields) > 1 && fields[1] == "clear" {
			s.clearLog()
			return
		}
		s.logf("shell: try logs clear")
	case "volume":
		if len(fields) < 2 {
			s.logf("volume=%d", s.volume)
			return
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			s.logf("shell: volume 0..10")
			return
		}
		s.volume = max(0, min(10, n))
		s.volumeShow = time.Now().Add(1400 * time.Millisecond)
		s.logf("volume=%d", s.volume)
	default:
		s.logf("shell: unknown command %s", fields[0])
	}
}

// togglePower flips display power. A power-off→on transition auto-locks the
// screen, mirroring real phone behaviour.
func (s *simulator) togglePower() {
	before := s.display.Powered()
	s.display.SetPowered(!before)
	s.logf("power=%v", s.display.Powered())
	if !before && s.display.Powered() {
		s.ws.Lock()
	}
}

func (s *simulator) toggleLock() {
	if s.ws.IsLocked() {
		s.ws.Unlock()
	} else {
		s.ws.Lock()
	}
}

func (s *simulator) requestScreenshot() {
	s.captureNext = true
	s.logf("screenshot requested")
}

func (s *simulator) scenarioSplitMessages() {
	s.ws.ExitMultiWindow()
	s.ws.Launch(windowing.AppIDSettings)
	s.ws.Launch(windowing.AppIDMessage)
	s.ws.EnterSplit()
	s.logf("scenario: split messages")
}

func (s *simulator) scenarioMessageKeyboard() {
	s.ws.ExitMultiWindow()
	s.ws.Launch(windowing.AppIDMessage)
	s.ws.ShowKeyboard()
	s.logf("scenario: message keyboard")
}

func (s *simulator) scenarioCurtainNotices() {
	s.ws.PostNotice(mosapp.Notice{Title: "Build", Body: "Scenario notice: compositor ready"})
	s.ws.PostNotice(mosapp.Notice{Title: "Message", Body: "Try reply flow with keyboard"})
	s.ws.PostNotice(mosapp.Notice{Title: "System", Body: "Dark mode and split are available"})
	s.ws.ToggleCurtain()
	s.logf("scenario: curtain notices")
}

func (s *simulator) scenarioRecentsStack() {
	s.ws.ExitMultiWindow()
	s.ws.Launch(windowing.AppIDGallery)
	s.ws.Launch(windowing.AppIDSceneTest)
	s.ws.Launch(windowing.AppIDMessage)
	s.ws.GoRecents()
	s.logf("scenario: recents stack")
}

func (s *simulator) Update() error {
	input.SetCursorOffset(0, 0)
	rawX, rawY := ebiten.CursorPosition()
	rawCursor := draws.XY{X: float64(rawX), Y: float64(rawY)}

	shellHandled := s.updateShell()
	ctrlDown := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	if !shellHandled && !ctrlDown {
		for _, a := range s.actions {
			for _, k := range a.Keys {
				if input.IsKeyJustPressed(k) {
					a.Handler()
					break
				}
			}
		}
	}
	if !s.display.Powered() && s.aodEnabled && s.aod != nil {
		// AOD ticks even when the display is "off" so the clock keeps moving.
		s.aod.Update()
	}

	// Sim action panel runs in raw viewport coords and dispatches regardless
	// of display power, so a click on Power can wake the screen.
	rawFrame := mosapp.Frame{Cursor: rawCursor, Events: s.rawInput.Poll()}
	powerMenuHandled := s.updatePowerMenu(rawFrame)
	for i := range s.actionGroups {
		if !powerMenuHandled {
			s.actionGroups[i].panel.Update(rawFrame)
		}
	}

	if s.display.Powered() && !shellHandled && !powerMenuHandled {
		group := s.effectiveGroup()
		active := group[s.activeDisplay]

		// On-screen nav buttons share rawFrame with the action panel.
		if s.backButton.Update(rawFrame) {
			s.ws.GoBack()
		}
		if s.homeButton.Update(rawFrame) {
			s.ws.GoHome()
		}
		if s.recentsButton.Update(rawFrame) {
			s.ws.GoRecents()
		}

		// Switch to canvas coords for the nav-swipe and the windowing server.
		input.SetCursorOffset(active.x, active.y)
		x, y := input.MouseCursorPosition()
		cursor := draws.XY{X: x, Y: y}
		navFrame := mosapp.Frame{Cursor: cursor, Events: s.navInput.Poll()}
		if s.navGesture.Update(navFrame).Kind == ui.GestureSwipeUp {
			s.ws.GoHome()
		}
		s.ws.Update()
	}
	return nil
}

func (s *simulator) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{38, 38, 40, 255}) // device body

	if s.display.Powered() {
		s.canvas.Clear()
		s.ws.Draw(s.canvas)
		if s.captureNext {
			s.ws.AddScreenshot(s.canvas)
			s.screenshots = s.ws.Screenshots()
			s.captureNext = false
		}
	} else if s.aodEnabled && s.aod != nil {
		// Display is off — paint the AOD layer so the active slot below shows
		// a dim clock instead of going to the slot's idle background.
		s.canvas.Clear()
		s.aod.Draw(s.canvas)
	} else {
		s.canvas.Fill(color.RGBA{0, 0, 0, 255})
	}

	s.drawDisplayGroup(screen)

	s.drawTriggers(draws.Image{Image: screen})
	s.drawVolumePanel(draws.Image{Image: screen})
	s.powerMenu.Draw(draws.Image{Image: screen})
	s.log.Draw(draws.Image{Image: screen})
	s.shell.Draw(draws.Image{Image: screen})
	s.help.Draw(draws.Image{Image: screen})
}

func (s *simulator) drawVolumePanel(dst draws.Image) {
	if time.Now().After(s.volumeShow) {
		return
	}
	x := actionPanelLeft() - 72
	y := actionPanelMargin
	bg := draws.CreateImage(52, 190)
	bg.Fill(color.RGBA{8, 10, 14, 220})
	sp := draws.NewSprite(bg)
	sp.Locate(x, y, draws.LeftTop)
	sp.Draw(dst)

	track := draws.CreateImage(8, 120)
	track.Fill(color.RGBA{255, 255, 255, 42})
	trackSp := draws.NewSprite(track)
	trackSp.Locate(x+22, y+48, draws.LeftTop)
	trackSp.Draw(dst)

	fillH := float64(s.volume) / 10 * 120
	fill := draws.CreateImage(8, fillH)
	fill.Fill(color.RGBA{80, 170, 255, 230})
	fillSp := draws.NewSprite(fill)
	fillSp.Locate(x+22, y+48+120-fillH, draws.LeftTop)
	fillSp.Draw(dst)

	opts := draws.NewFaceOptions()
	opts.Size = 11
	label := draws.NewText("VOL")
	label.SetFace(opts)
	label.Locate(x+26, y+18, draws.CenterMiddle)
	label.Draw(dst)
	value := draws.NewText(strconv.Itoa(s.volume))
	value.SetFace(opts)
	value.Locate(x+26, y+176, draws.CenterMiddle)
	value.Draw(dst)
}

func (s *simulator) drawDisplayGroup(screen *ebiten.Image) {
	for i, sl := range s.simGroup.slots {
		s.drawSlot(screen, sl, sl.screenSpec, i == s.activeDisplay)
	}
}

func (s *simulator) drawSlot(screen *ebiten.Image, sl screenSlot, spec screenSpec, active bool) {
	drawImageInSpec(screen, sl.border.Image, screenSpec{
		x: spec.x - borderPx,
		y: spec.y - borderPx,
		w: spec.w + borderPx*2,
		h: spec.h + borderPx*2,
	}, 0, 1)

	src := sl.bg.Image
	if active {
		src = s.canvas.Image
	}
	drawImageInSpec(screen, src, spec, 0, 1)
	if active {
		s.drawRotateOverlay(screen, spec)
	}
}

func (s *simulator) drawRotateOverlay(screen *ebiten.Image, spec screenSpec) {
	if !s.rotateAnim.active {
		return
	}
	p := s.rotateAnim.p.Value()
	theta := s.rotateAnim.dir * (1 - p) * math.Pi / 2
	alpha := float32(1 - p)
	s.drawClippedRotateOverlay(screen, spec, theta, alpha)
	if s.rotateAnim.p.Done() {
		s.rotateAnim.active = false
	}
}

func (s *simulator) drawClippedRotateOverlay(screen *ebiten.Image, spec screenSpec, theta float64, alpha float32) {
	w, h := int(spec.w), int(spec.h)
	if w <= 0 || h <= 0 {
		return
	}
	if s.rotateAnim.clip.IsEmpty() || s.rotateAnim.clip.Image.Bounds().Dx() != w || s.rotateAnim.clip.Image.Bounds().Dy() != h {
		s.rotateAnim.clip = draws.CreateImage(spec.w, spec.h)
	}
	s.rotateAnim.clip.Clear()

	localSpec := screenSpec{w: spec.w, h: spec.h}
	drawImageInSpec(s.rotateAnim.clip.Image, s.rotateAnim.snapshot.Image, localSpec, theta, alpha)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(spec.x, spec.y)
	screen.DrawImage(s.rotateAnim.clip.Image, op)
}

func drawImageInSpec(screen *ebiten.Image, src *ebiten.Image, spec screenSpec, theta float64, alpha float32) {
	b := src.Bounds()
	srcW := float64(b.Dx())
	srcH := float64(b.Dy())
	if srcW <= 0 || srcH <= 0 || spec.w <= 0 || spec.h <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-srcW/2, -srcH/2)
	op.GeoM.Scale(spec.w/srcW, spec.h/srcH)
	op.GeoM.Rotate(theta)
	op.GeoM.Translate(spec.x+spec.w/2, spec.y+spec.h/2)
	op.ColorScale.ScaleAlpha(alpha)
	screen.DrawImage(src, op)
}

func (s *simulator) drawTriggers(dst draws.Image) {
	s.controlPanelBg.Draw(dst)
	s.backButtonBg.Draw(dst)
	s.homeButtonBg.Draw(dst)
	s.recentsButtonBg.Draw(dst)
	s.backButtonText.Draw(dst)
	s.homeButtonText.Draw(dst)
	s.recentsButtonText.Draw(dst)

	for i := range s.actionGroups {
		s.actionGroups[i].title.Draw(dst)
		s.actionGroups[i].panel.Draw(dst)
	}
}

func (s *simulator) Layout(_, _ int) (int, int) { return simW, simH }

func main() {
	ebiten.SetWindowSize(simW, simH)
	ebiten.RunGame(newSimulator())
}
