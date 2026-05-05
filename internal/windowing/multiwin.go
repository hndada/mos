package windowing

// multiwin.go — split-screen, picture-in-picture, and freeform floating
// window management.
//
// Real-OS analogue:
//
//	Split       ↔  Android split-screen (ActivityOptions.setLaunchBounds) /
//	               iPadOS split view (UISplitViewController)
//	PiP         ↔  Android PictureInPictureParams / AVPictureInPictureController
//	Freeform    ↔  Android freeform window mode (developer option) /
//	               macOS NSWindow free positioning
//
// All layout state lives in multiWindowManager, which is owned by the
// WindowingServer. It is updated once per frame (Update*) and drawn once
// per frame (Draw*) on the main goroutine.

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	// DurationMultiWin is the animation duration for split/pip/freeform entry
	// and exit transitions.
	DurationMultiWin = 260 * time.Millisecond

	// Split divider
	splitGap              = 6.0  // screen pixels between the two windows
	splitMinFrac          = 0.25 // minimum primary fraction
	splitMaxFrac          = 0.75 // maximum primary fraction
	splitDragHitThickness = 32.0 // wider grab area for the divider

	// PiP
	pipFracW     = 0.38  // pip width as fraction of screen width
	pipMargin    = 14.0  // minimum distance from screen edge
	pipSnapSpeed = 120 * time.Millisecond

	// Freeform
	freeformTitleH       = 28.0  // title-bar height in screen pixels
	freeformDefaultFracW = 0.60  // default window width as fraction of screen
	freeformDefaultFracH = 0.65  // default window height as fraction of screen
	freeformStagger      = 28.0  // per-window stagger offset
)

// ── Mode enum ────────────────────────────────────────────────────────────────

type multiWindowMode int

const (
	multiModeNone     multiWindowMode = iota
	multiModeSplit                    // two windows divided by a draggable bar
	multiModePip                      // small overlay + fullscreen background
	multiModeFreeform                 // any number of draggable floating windows
)

// ── Axis / Corner helpers ─────────────────────────────────────────────────────

// SplitAxis controls whether split windows are arranged side-by-side or stacked.
type SplitAxis int

const (
	SplitAxisVertical   SplitAxis = iota // primary | secondary (left/right)
	SplitAxisHorizontal                  // primary / secondary (top/bottom)
)

type pipCorner int

const (
	pipCornerBottomRight pipCorner = iota
	pipCornerBottomLeft
	pipCornerTopRight
	pipCornerTopLeft
)

// ── multiWindowManager ───────────────────────────────────────────────────────

// multiWindowManager handles all multi-window state for one WindowingServer.
// It is nil when all windows are fullscreen.
type multiWindowManager struct {
	mode    multiWindowMode
	screenW float64
	screenH float64

	// ── Split ──────────────────────────────────────────────────────────────
	splitPrimary   *Window
	splitSecondary *Window
	splitAxis      SplitAxis
	splitFrac      float64 // 0..1, fraction of screen for primary
	splitDragging  bool
	splitDragStart float64 // screen-axis coordinate at drag start
	splitDragFrac  float64 // splitFrac at drag start

	splitDivImg draws.Image // thin solid-colour bar; scaled at draw time

	// ── PiP ────────────────────────────────────────────────────────────────
	pipWindow   *Window
	pipCorner   pipCorner
	pipDragging bool
	pipDragOff  draws.XY // offset from pip center to pointer at drag start

	pipBorderImg draws.Image // 1-pixel accent; scaled at draw time

	// per-frame event buffers populated by updatePip
	pipFrameEvents  []input.Event
	mainFrameEvents []input.Event

	// ── Freeform ───────────────────────────────────────────────────────────
	freeformDragWindow *Window
	freeformDragOffset draws.XY // offset from window center to pointer

	freeformTitleNormalImg  draws.Image // unfocused title bar colour
	freeformTitleFocusedImg draws.Image // focused title bar colour
	freeformFocusRingImg    draws.Image // 1-px accent for the focused border
}

func newMultiWindowManager(screenW, screenH float64) *multiWindowManager {
	m := &multiWindowManager{
		screenW:   screenW,
		screenH:   screenH,
		splitFrac: 0.5,
	}

	// Pre-allocate tiny solid-colour images; scaled by sprites at draw time.
	m.splitDivImg = draws.CreateImage(4, 4)
	m.splitDivImg.Fill(color.RGBA{80, 84, 96, 255})

	m.pipBorderImg = draws.CreateImage(4, 4)
	m.pipBorderImg.Fill(color.RGBA{80, 150, 255, 255})

	m.freeformTitleNormalImg = draws.CreateImage(4, 4)
	m.freeformTitleNormalImg.Fill(color.RGBA{32, 34, 46, 220})

	m.freeformTitleFocusedImg = draws.CreateImage(4, 4)
	m.freeformTitleFocusedImg.Fill(color.RGBA{30, 90, 190, 230})

	m.freeformFocusRingImg = draws.CreateImage(4, 4)
	m.freeformFocusRingImg.Fill(color.RGBA{60, 140, 255, 255})

	return m
}

// ── Split ─────────────────────────────────────────────────────────────────────

func (m *multiWindowManager) enterSplit(primary, secondary *Window) {
	m.mode = multiModeSplit
	m.splitPrimary = primary
	m.splitSecondary = secondary
	m.splitAxis = SplitAxisVertical
	m.splitFrac = 0.5
	m.applySplitPlacements(DurationMultiWin)
}

func (m *multiWindowManager) splitPlacements() (primary, secondary WindowPlacement) {
	if m.splitAxis == SplitAxisVertical {
		pW := m.screenW*m.splitFrac - splitGap/2
		sW := m.screenW*(1-m.splitFrac) - splitGap/2
		primary = WindowPlacement{
			Center: draws.XY{X: pW / 2, Y: m.screenH / 2},
			Size:   draws.XY{X: pW, Y: m.screenH},
		}
		secondary = WindowPlacement{
			Center: draws.XY{X: m.screenW - sW/2, Y: m.screenH / 2},
			Size:   draws.XY{X: sW, Y: m.screenH},
		}
	} else {
		pH := m.screenH*m.splitFrac - splitGap/2
		sH := m.screenH*(1-m.splitFrac) - splitGap/2
		primary = WindowPlacement{
			Center: draws.XY{X: m.screenW / 2, Y: pH / 2},
			Size:   draws.XY{X: m.screenW, Y: pH},
		}
		secondary = WindowPlacement{
			Center: draws.XY{X: m.screenW / 2, Y: m.screenH - sH/2},
			Size:   draws.XY{X: m.screenW, Y: sH},
		}
	}
	return
}

func (m *multiWindowManager) applySplitPlacements(dur time.Duration) {
	p, s := m.splitPlacements()
	m.splitPrimary.mode = WindowModeSplit
	m.splitPrimary.placement = p
	m.splitSecondary.mode = WindowModeSplit
	m.splitSecondary.placement = s
	if dur > 0 {
		m.splitPrimary.anim.Retarget(p.Center, p.Size, dur)
		m.splitSecondary.anim.Retarget(s.Center, s.Size, dur)
	} else {
		m.splitPrimary.anim.SnapTo(p.Center, p.Size)
		m.splitSecondary.anim.SnapTo(s.Center, s.Size)
	}
}

func (m *multiWindowManager) splitDividerPos() float64 {
	if m.splitAxis == SplitAxisVertical {
		return m.screenW * m.splitFrac
	}
	return m.screenH * m.splitFrac
}

func (m *multiWindowManager) inSplitDivider(pos draws.XY) bool {
	dp := m.splitDividerPos()
	ht := splitDragHitThickness
	if m.splitAxis == SplitAxisVertical {
		return pos.X >= dp-ht/2 && pos.X < dp+ht/2
	}
	return pos.Y >= dp-ht/2 && pos.Y < dp+ht/2
}

// updateSplit consumes divider-drag events and returns the remainder for
// normal window routing.
func (m *multiWindowManager) updateSplit(events []input.Event) []input.Event {
	remaining := events[:0]
	for _, ev := range events {
		switch ev.Kind {
		case input.EventDown:
			if m.inSplitDivider(ev.Pos) {
				m.splitDragging = true
				if m.splitAxis == SplitAxisVertical {
					m.splitDragStart = ev.Pos.X
				} else {
					m.splitDragStart = ev.Pos.Y
				}
				m.splitDragFrac = m.splitFrac
			} else {
				remaining = append(remaining, ev)
			}
		case input.EventMove:
			if m.splitDragging {
				var coord, total float64
				if m.splitAxis == SplitAxisVertical {
					coord, total = ev.Pos.X, m.screenW
				} else {
					coord, total = ev.Pos.Y, m.screenH
				}
				m.splitFrac = clampF(
					m.splitDragFrac+(coord-m.splitDragStart)/total,
					splitMinFrac, splitMaxFrac)
				m.applySplitPlacements(0)
			} else {
				remaining = append(remaining, ev)
			}
		case input.EventUp:
			if m.splitDragging {
				m.splitDragging = false
			} else {
				remaining = append(remaining, ev)
			}
		default:
			remaining = append(remaining, ev)
		}
	}
	return remaining
}

func (m *multiWindowManager) drawSplit(dst draws.Image) {
	dp := m.splitDividerPos()
	s := draws.NewSprite(m.splitDivImg)
	if m.splitAxis == SplitAxisVertical {
		s.Locate(dp, m.screenH/2, draws.CenterMiddle)
		s.Size = draws.XY{X: splitGap, Y: m.screenH}
	} else {
		s.Locate(m.screenW/2, dp, draws.CenterMiddle)
		s.Size = draws.XY{X: m.screenW, Y: splitGap}
	}
	s.Draw(dst)
}

func (m *multiWindowManager) exitSplit() {
	fp := fullscreenPlacement(m.screenW, m.screenH)
	for _, w := range []*Window{m.splitPrimary, m.splitSecondary} {
		if w == nil {
			continue
		}
		w.mode = WindowModeFullscreen
		w.placement = fp
		w.anim.Retarget(fp.Center, fp.Size, DurationMultiWin)
	}
	m.splitPrimary = nil
	m.splitSecondary = nil
}

// ── PiP ──────────────────────────────────────────────────────────────────────

func pipPlacement(corner pipCorner, screenW, screenH float64) WindowPlacement {
	w := screenW * pipFracW
	h := w * (screenH / screenW)
	var cx, cy float64
	switch corner {
	case pipCornerBottomRight:
		cx, cy = screenW-w/2-pipMargin, screenH-h/2-pipMargin
	case pipCornerBottomLeft:
		cx, cy = w/2+pipMargin, screenH-h/2-pipMargin
	case pipCornerTopRight:
		cx, cy = screenW-w/2-pipMargin, h/2+pipMargin
	case pipCornerTopLeft:
		cx, cy = w/2+pipMargin, h/2+pipMargin
	}
	return WindowPlacement{
		Center: draws.XY{X: cx, Y: cy},
		Size:   draws.XY{X: w, Y: h},
	}
}

func (m *multiWindowManager) enterPip(pip *Window, corner pipCorner) {
	m.mode = multiModePip
	m.pipWindow = pip
	m.pipCorner = corner
	p := pipPlacement(corner, m.screenW, m.screenH)
	pip.mode = WindowModePip
	pip.placement = p
	pip.anim.Retarget(p.Center, p.Size, DurationMultiWin)
}

// updatePip separates events into pip-destined and main-destined, and
// handles corner-to-corner drag. Populated into m.pipFrameEvents /
// m.mainFrameEvents for retrieval in frameForWindow.
func (m *multiWindowManager) updatePip(events []input.Event) {
	m.pipFrameEvents = m.pipFrameEvents[:0]
	m.mainFrameEvents = m.mainFrameEvents[:0]
	pip := m.pipWindow
	if pip == nil {
		m.mainFrameEvents = append(m.mainFrameEvents, events...)
		return
	}

	pc := pip.anim.Pos()
	ps := pip.anim.Size()

	for _, ev := range events {
		inPip := ev.Pos.X >= pc.X-ps.X/2 && ev.Pos.X < pc.X+ps.X/2 &&
			ev.Pos.Y >= pc.Y-ps.Y/2 && ev.Pos.Y < pc.Y+ps.Y/2

		switch ev.Kind {
		case input.EventDown:
			if inPip {
				m.pipDragging = true
				m.pipDragOff = draws.XY{X: ev.Pos.X - pc.X, Y: ev.Pos.Y - pc.Y}
				m.pipFrameEvents = append(m.pipFrameEvents, ev)
			} else {
				m.mainFrameEvents = append(m.mainFrameEvents, ev)
			}

		case input.EventMove:
			if m.pipDragging {
				newCX := ev.Pos.X - m.pipDragOff.X
				newCY := ev.Pos.Y - m.pipDragOff.Y
				hw, hh := ps.X/2, ps.Y/2
				newCX = clampF(newCX, hw+pipMargin, m.screenW-hw-pipMargin)
				newCY = clampF(newCY, hh+pipMargin, m.screenH-hh-pipMargin)
				pip.placement = WindowPlacement{Center: draws.XY{X: newCX, Y: newCY}, Size: ps}
				pip.anim.SnapTo(draws.XY{X: newCX, Y: newCY}, ps)
			} else {
				m.mainFrameEvents = append(m.mainFrameEvents, ev)
			}

		case input.EventUp:
			if m.pipDragging {
				m.pipDragging = false
				// Snap to the nearest screen corner.
				m.pipCorner = m.nearestPipCorner(pip.anim.Pos())
				np := pipPlacement(m.pipCorner, m.screenW, m.screenH)
				pip.placement = np
				pip.anim.Retarget(np.Center, np.Size, pipSnapSpeed)
				m.pipFrameEvents = append(m.pipFrameEvents, ev)
			} else {
				m.mainFrameEvents = append(m.mainFrameEvents, ev)
			}

		default:
			m.mainFrameEvents = append(m.mainFrameEvents, ev)
		}
	}
}

func (m *multiWindowManager) nearestPipCorner(center draws.XY) pipCorner {
	mx, my := m.screenW/2, m.screenH/2
	if center.X < mx {
		if center.Y < my {
			return pipCornerTopLeft
		}
		return pipCornerBottomLeft
	}
	if center.Y < my {
		return pipCornerTopRight
	}
	return pipCornerBottomRight
}

// drawPip renders a 2-pixel accent border around the PiP window.
func (m *multiWindowManager) drawPip(dst draws.Image) {
	pip := m.pipWindow
	if pip == nil {
		return
	}
	pc := pip.anim.Pos()
	ps := pip.anim.Size()
	const bw = 2.0
	// Draw as a slightly larger solid rect behind the window sprite.
	bs := draws.NewSprite(m.pipBorderImg)
	bs.Locate(pc.X, pc.Y, draws.CenterMiddle)
	bs.Size = draws.XY{X: ps.X + bw*2, Y: ps.Y + bw*2}
	bs.Draw(dst)
}

func (m *multiWindowManager) exitPip() {
	if m.pipWindow != nil {
		fp := fullscreenPlacement(m.screenW, m.screenH)
		m.pipWindow.mode = WindowModeFullscreen
		m.pipWindow.placement = fp
		m.pipWindow.anim.Retarget(fp.Center, fp.Size, DurationMultiWin)
	}
	m.pipWindow = nil
	m.pipDragging = false
}

// ── Freeform ──────────────────────────────────────────────────────────────────

func (m *multiWindowManager) enterFreeform(windows []*Window) {
	m.mode = multiModeFreeform
	baseW := m.screenW * freeformDefaultFracW
	baseH := m.screenH * freeformDefaultFracH
	for i, w := range windows {
		off := float64(i) * freeformStagger
		cx := m.screenW/2 + off
		cy := m.screenH/2 + off
		// Clamp so the window stays on screen.
		if cx+baseW/2 > m.screenW {
			cx = m.screenW - baseW/2
		}
		if cy+baseH/2 > m.screenH {
			cy = m.screenH - baseH/2
		}
		p := WindowPlacement{
			Center: draws.XY{X: cx, Y: cy},
			Size:   draws.XY{X: baseW, Y: baseH},
		}
		w.mode = WindowModeFloat
		w.placement = p
		w.anim.Retarget(p.Center, p.Size, DurationMultiWin)
	}
}

// inTitleBar reports whether pos is inside the title bar of window w.
func (m *multiWindowManager) inTitleBar(w *Window, pos draws.XY) bool {
	c := w.anim.Pos()
	sz := w.anim.Size()
	minX := c.X - sz.X/2
	minY := c.Y - sz.Y/2
	return pos.X >= minX && pos.X < minX+sz.X &&
		pos.Y >= minY && pos.Y < minY+freeformTitleH
}

// updateFreeform handles title-bar drag events. Returns remaining events
// (those not consumed as drags) and the newly focused window (may be nil if
// no focus change occurred).
func (m *multiWindowManager) updateFreeform(events []input.Event, windows []*Window, currentFocus *Window) (remaining []input.Event, newFocus *Window) {
	remaining = events[:0]
	for _, ev := range events {
		switch ev.Kind {
		case input.EventDown:
			// Check title bars back-to-front (topmost window first).
			grabbed := false
			for i := len(windows) - 1; i >= 0; i-- {
				w := windows[i]
				if w.lifecycle != LifecycleShown {
					continue
				}
				if m.inTitleBar(w, ev.Pos) {
					m.freeformDragWindow = w
					c := w.anim.Pos()
					m.freeformDragOffset = draws.XY{X: ev.Pos.X - c.X, Y: ev.Pos.Y - c.Y}
					newFocus = w
					grabbed = true
					break
				}
			}
			if !grabbed {
				remaining = append(remaining, ev)
			}
		case input.EventMove:
			if m.freeformDragWindow != nil {
				w := m.freeformDragWindow
				sz := w.anim.Size()
				newCX := ev.Pos.X - m.freeformDragOffset.X
				newCY := ev.Pos.Y - m.freeformDragOffset.Y
				newCX = clampF(newCX, sz.X/2, m.screenW-sz.X/2)
				newCY = clampF(newCY, sz.Y/2, m.screenH-sz.Y/2)
				p := WindowPlacement{Center: draws.XY{X: newCX, Y: newCY}, Size: sz}
				w.placement = p
				w.anim.SnapTo(p.Center, p.Size)
			} else {
				remaining = append(remaining, ev)
			}
		case input.EventUp:
			if m.freeformDragWindow != nil {
				m.freeformDragWindow = nil
			} else {
				remaining = append(remaining, ev)
			}
		default:
			remaining = append(remaining, ev)
		}
	}
	return
}

// drawFreeform renders a title bar and optional focus ring for each shown
// freeform window. Must be called after the window sprites are drawn.
func (m *multiWindowManager) drawFreeform(dst draws.Image, windows []*Window, focused *Window) {
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 11

	for _, w := range windows {
		if w.lifecycle != LifecycleShown && w.lifecycle != LifecycleShowing {
			continue
		}
		if w.mode != WindowModeFloat {
			continue
		}
		c := w.anim.Pos()
		sz := w.anim.Size()
		minX := c.X - sz.X/2
		minY := c.Y - sz.Y/2

		// Title bar background.
		bgImg := m.freeformTitleNormalImg
		if w == focused {
			bgImg = m.freeformTitleFocusedImg
		}
		tbg := draws.NewSprite(bgImg)
		tbg.Locate(minX+sz.X/2, minY+freeformTitleH/2, draws.CenterMiddle)
		tbg.Size = draws.XY{X: sz.X, Y: freeformTitleH}
		tbg.Draw(dst)

		// App-ID label.
		label := draws.NewText(w.AppID())
		label.SetFace(titleOpts)
		label.Locate(minX+sz.X/2, minY+freeformTitleH/2, draws.CenterMiddle)
		label.Draw(dst)

		// Focus ring: a 2-px top edge in accent colour.
		if w == focused {
			ring := draws.NewSprite(m.freeformFocusRingImg)
			ring.Locate(minX+sz.X/2, minY+1, draws.CenterMiddle)
			ring.Size = draws.XY{X: sz.X, Y: 2}
			ring.Draw(dst)
		}
	}
}

func (m *multiWindowManager) exitFreeform(windows []*Window) {
	fp := fullscreenPlacement(m.screenW, m.screenH)
	for _, w := range windows {
		if w.mode != WindowModeFloat {
			continue
		}
		w.mode = WindowModeFullscreen
		w.placement = fp
		w.anim.Retarget(fp.Center, fp.Size, DurationMultiWin)
	}
	m.freeformDragWindow = nil
}

// ── Cleanup ───────────────────────────────────────────────────────────────────

// cleanup verifies that the windows that anchor the current multi-window mode
// are still alive. If a key window has been destroyed, the mode is cleared and
// surviving windows are returned to fullscreen. Call after the purge step each
// frame.
func (m *multiWindowManager) cleanup(windows []*Window) {
	switch m.mode {
	case multiModeSplit:
		pAlive := isWindowInList(m.splitPrimary, windows)
		sAlive := isWindowInList(m.splitSecondary, windows)
		if !pAlive || !sAlive {
			m.exitSplit()
			m.mode = multiModeNone
		}
	case multiModePip:
		if !isWindowInList(m.pipWindow, windows) {
			m.exitPip()
			m.mode = multiModeNone
		}
	}
}

func isWindowInList(w *Window, list []*Window) bool {
	if w == nil {
		return false
	}
	for _, lw := range list {
		if lw == w && lw.lifecycle != LifecycleDestroyed {
			return true
		}
	}
	return false
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// filterEventsByRect returns events whose Pos is inside the given rect.
// EventMove and EventUp are also filtered so a window never receives stray
// events from pointer activity in another window.
func filterEventsByRect(events []input.Event, center, size draws.XY) []input.Event {
	if len(events) == 0 {
		return nil
	}
	minX := center.X - size.X/2
	minY := center.Y - size.Y/2
	maxX := minX + size.X
	maxY := minY + size.Y
	var out []input.Event
	for _, ev := range events {
		if ev.Pos.X >= minX && ev.Pos.X < maxX &&
			ev.Pos.Y >= minY && ev.Pos.Y < maxY {
			out = append(out, ev)
		}
	}
	return out
}
