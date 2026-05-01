package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/fw"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/sysapps"
)

const (
	simW = 1200
	simH = 900
)

// ── display mode ─────────────────────────────────────────────────────────────

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

	cx := func(w float64) float64 { return (simW - w) / 2 }
	cy := func(h float64) float64 { return (simH - h) / 2 }

	// Flip: cover and main side by side horizontally with a gap.
	flipTotalW := fcW + gap + fmW
	flipGroupH := max(fcH, fmH)
	flipX := cx(flipTotalW)
	flipY := cy(flipGroupH)

	// Fold: cover and main side by side horizontally with a gap.
	foldTotalW := fdcW + gap + fdmW
	foldGroupH := max(fdcH, fdmH)
	foldX := cx(foldTotalW)
	foldY := cy(foldGroupH)

	return [displayModeCount]displayGroup{
		displayModeBar: {
			{w: barW, h: barH, x: cx(barW), y: cy(barH), primary: true},
		},
		displayModeFlip: {
			{w: fcW, h: fcH, x: flipX, y: flipY},
			{w: fmW, h: fmH, x: flipX + fcW + gap, y: flipY, primary: true},
		},
		displayModeFold: {
			{w: fdcW, h: fdcH, x: foldX, y: foldY},
			{w: fdmW, h: fdmH, x: foldX + fdcW + gap, y: foldY, primary: true},
		},
	}
}

// ── pre-built per-group visuals ───────────────────────────────────────────────

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

// ── simulator ────────────────────────────────────────────────────────────────

// Simulator drives the OS loop and emulates physical hardware buttons:
//
//	F1 — power (toggle display on/off)
//	F2 — cycle device mode: bar, flip, fold
//	F3 — cycle active display within the current device mode
type simulator struct {
	mode          displayMode
	activeDisplay int
	groups        [displayModeCount]displayGroup
	simGroup      simGroup
	canvas        draws.Image // render target for the active display
	ws            fw.WindowingServer
	display       fw.Display
}

func newSimulator() *simulator {
	s := &simulator{}
	s.groups = groups()
	s.activeDisplay = primaryIndex(s.groups[s.mode])
	s.applyMode()
	return s
}

func (s *simulator) applyMode() {
	group := s.groups[s.mode]
	if s.activeDisplay >= len(group) {
		s.activeDisplay = primaryIndex(group)
	}
	active := group[s.activeDisplay]

	s.canvas = draws.CreateImage(active.w, active.h)
	s.ws = fw.WindowingServer{ScreenW: active.w, ScreenH: active.h}
	s.ws.SetWallpaper(sysapps.NewDefaultWallpaper(active.w, active.h))
	s.ws.SetHome(sysapps.NewDefaultHome(active.w, active.h))
	s.ws.SetStatusBar(sysapps.NewDefaultStatusBar(active.w, active.h))
	s.display.W = active.w
	s.display.H = active.h
	s.simGroup = buildGroup(group)
	ebiten.SetWindowTitle("MOS Simulator - " + s.mode.String())
}

func (s *simulator) changeDeviceMode() {
	s.mode = (s.mode + 1) % displayModeCount
	s.activeDisplay = primaryIndex(s.groups[s.mode])
	s.applyMode()
}

func (s *simulator) changeActiveDisplay() {
	group := s.groups[s.mode]
	if len(group) <= 1 {
		return
	}
	s.activeDisplay = (s.activeDisplay + 1) % len(group)
	s.applyMode()
}

func (s *simulator) Update() error {
	if input.IsKeyJustPressed(input.KeyF1) {
		s.display.SetPowered(!s.display.Powered())
	}
	if input.IsKeyJustPressed(input.KeyF2) {
		s.changeDeviceMode()
	}
	if input.IsKeyJustPressed(input.KeyF3) {
		s.changeActiveDisplay()
	}
	active := s.groups[s.mode][s.activeDisplay]
	input.SetCursorOffset(active.x, active.y)
	s.ws.Update()
	return nil
}

func (s *simulator) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{38, 38, 40, 255}) // device body

	s.canvas.Clear()
	s.ws.Draw(s.canvas)

	for i, sl := range s.simGroup.slots {
		brdOp := &ebiten.DrawImageOptions{}
		brdOp.GeoM.Translate(sl.x-borderPx, sl.y-borderPx)
		screen.DrawImage(sl.border.Image, brdOp)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(sl.x, sl.y)
		if i == s.activeDisplay {
			screen.DrawImage(s.canvas.Image, op)
		} else {
			screen.DrawImage(sl.bg.Image, op)
		}
	}
}

func (s *simulator) Layout(_, _ int) (int, int) { return simW, simH }

func main() {
	ebiten.SetWindowSize(simW, simH)
	ebiten.RunGame(newSimulator())
}
