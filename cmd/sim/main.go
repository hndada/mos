package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/windowing"
	"github.com/hndada/mos/ui"
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

	const sc = 0.24
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

func placeDisplayGroup(w, h float64) (float64, float64) {
	centerX := func(width float64) float64 { return (simW - width) / 2 }
	centerY := func(height float64) float64 { return (simH - height) / 2 }
	overlapsLog := func(x, y float64) bool {
		return x < logX+logW+logGap &&
			x+w > logX &&
			y < logY+logH+logGap &&
			y+h > logY
	}

	x, y := centerX(w), centerY(h)
	if !overlapsLog(x, y) {
		return x, y
	}

	rightX := logX + logW + logGap
	if rightX+w <= simW-borderPx {
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
	groups            [displayModeCount]displayGroup
	simGroup          simGroup
	canvas            draws.Image // render target for the active display
	ws                windowing.WindowingServer
	display           windowing.Display
	help              draws.Text
	log               simLog
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
}

const helpString = "P: Power   1/2/3: Bar-Flip-Fold   S: Active screen   B/Esc: Back   N: Curtain   K: Keys   V: Ring   L/F: Log"

const (
	controlPanelH = 38.0
	logMaxLines   = 31
	logX          = 12.0
	logY          = 12.0
	logW          = 560.0
	logH          = simH - logY - 52
	logGap        = 28.0
)

type simLog struct {
	bg      draws.Sprite
	lines   []draws.Text
	entries []string
	visible bool
}

func newSimLog() simLog {
	img := draws.CreateImage(logW, logH)
	img.Fill(color.RGBA{0, 0, 0, 150})
	bg := draws.NewSprite(img)
	bg.Locate(logX, logY, draws.LeftTop)

	opts := draws.NewFaceOptions()
	opts.Size = 16
	lines := make([]draws.Text, logMaxLines)
	for i := range lines {
		lines[i] = draws.NewText("")
		lines[i].SetFace(opts)
		lines[i].Locate(28, 58+float64(i)*22, draws.LeftTop)
	}

	return simLog{
		bg:      bg,
		lines:   lines,
		visible: true,
	}
}

func (l *simLog) Add(msg string) {
	stamp := time.Now().Format("15:04:05.000")
	l.entries = append(l.entries, stamp+"  "+msg)
	if len(l.entries) > logMaxLines {
		copy(l.entries, l.entries[len(l.entries)-logMaxLines:])
		l.entries = l.entries[:logMaxLines]
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
	start := logMaxLines - len(l.entries)
	for i := range l.lines {
		l.lines[i].Text = ""
	}
	for i, entry := range l.entries {
		l.lines[start+i].Text = entry
	}
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
	s := &simulator{}
	s.groups = groups()
	s.activeDisplay = primaryIndex(s.groups[s.mode])

	opts := draws.NewFaceOptions()
	opts.Size = 13
	t := draws.NewText(helpString)
	t.SetFace(opts)
	t.Locate(simW/2, simH-8, draws.CenterBottom)
	s.help = t
	s.log = newSimLog()
	s.logf("simulator boot")

	s.applyMode()
	return s
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

	group := s.groups[s.mode]
	if s.activeDisplay >= len(group) {
		s.activeDisplay = primaryIndex(group)
	}
	active := group[s.activeDisplay]

	s.canvas = draws.CreateImage(active.w, active.h)
	s.ws = windowing.WindowingServer{ScreenW: active.w, ScreenH: active.h, Bus: event.NewBus()}
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
	s.ws.SetCurtain(apps.NewDefaultCurtain(active.w, active.h))
	s.ws.SetKeyboard(apps.NewDefaultKeyboard(active.w, active.h))
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
	s.applyMode()
}

func (s *simulator) cycleActiveDisplay() {
	group := s.groups[s.mode]
	if len(group) <= 1 {
		s.logf("active display unchanged")
		return
	}
	s.activeDisplay = (s.activeDisplay + 1) % len(group)
	s.logf("active display -> %d", s.activeDisplay)
	s.applyMode()
}

func (s *simulator) toggleLog() {
	s.log.Toggle()
}

func (s *simulator) Update() error {
	rawX, rawY := ebiten.CursorPosition()
	rawCursor := draws.XY{X: float64(rawX), Y: float64(rawY)}

	if input.IsKeyJustPressed(input.KeyP) {
		s.display.SetPowered(!s.display.Powered())
		s.logf("power=%v", s.display.Powered())
	}
	if input.IsKeyJustPressed(input.KeyDigit1) {
		s.setMode(displayModeBar)
	}
	if input.IsKeyJustPressed(input.KeyDigit2) {
		s.setMode(displayModeFlip)
	}
	if input.IsKeyJustPressed(input.KeyDigit3) {
		s.setMode(displayModeFold)
	}
	if input.IsKeyJustPressed(input.KeyS) {
		s.cycleActiveDisplay()
	}
	if input.IsKeyJustPressed(input.KeyL) {
		s.toggleLog()
	}
	if input.IsKeyJustPressed(input.KeyF) {
		s.log.Clear()
	}
	if input.IsKeyJustPressed(input.KeyC) || input.IsKeyJustPressed(input.KeyPrintScreen) {
		s.captureNext = true
		s.logf("screenshot requested")
	}
	if input.IsKeyJustPressed(input.KeyB) || input.IsKeyJustPressed(input.KeyEscape) || input.IsKeyJustPressed(input.KeyBackspace) {
		s.ws.GoBack()
	}
	if input.IsKeyJustPressed(input.KeyH) {
		s.ws.GoHome()
	}
	if input.IsKeyJustPressed(input.KeyR) {
		s.ws.GoRecents()
	}
	if input.IsKeyJustPressed(input.KeyK) {
		s.ws.ToggleKeyboard()
	}
	if input.IsKeyJustPressed(input.KeyV) {
		s.ws.ReceiveCall()
	}
	if input.IsKeyJustPressed(input.KeyN) {
		s.ws.ToggleCurtain()
	}
	if s.display.Powered() {
		active := s.groups[s.mode][s.activeDisplay]
		if s.backButton.Update(rawCursor) {
			s.ws.GoBack()
		}
		if s.homeButton.Update(rawCursor) {
			s.ws.GoHome()
		}
		if s.recentsButton.Update(rawCursor) {
			s.ws.GoRecents()
		}

		input.SetCursorOffset(active.x, active.y)
		x, y := input.MouseCursorPosition()
		cursor := draws.XY{X: x, Y: y}
		if s.navGesture.Update(cursor).Kind == ui.GestureSwipeUp {
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
	}

	for i, sl := range s.simGroup.slots {
		brdOp := &ebiten.DrawImageOptions{}
		brdOp.GeoM.Translate(sl.x-borderPx, sl.y-borderPx)
		screen.DrawImage(sl.border.Image, brdOp)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(sl.x, sl.y)
		if i == s.activeDisplay && s.display.Powered() {
			screen.DrawImage(s.canvas.Image, op)
		} else {
			screen.DrawImage(sl.bg.Image, op)
		}
	}

	s.drawTriggers(draws.Image{Image: screen})
	s.log.Draw(draws.Image{Image: screen})
	s.help.Draw(draws.Image{Image: screen})
}

func (s *simulator) drawTriggers(dst draws.Image) {
	s.controlPanelBg.Draw(dst)
	s.backButtonBg.Draw(dst)
	s.homeButtonBg.Draw(dst)
	s.recentsButtonBg.Draw(dst)
	s.backButtonText.Draw(dst)
	s.homeButtonText.Draw(dst)
	s.recentsButtonText.Draw(dst)
}

func (s *simulator) Layout(_, _ int) (int, int) { return simW, simH }

func main() {
	ebiten.SetWindowSize(simW, simH)
	ebiten.RunGame(newSimulator())
}
