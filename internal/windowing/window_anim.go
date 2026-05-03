package windowing

import (
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/tween"
)

// WindowAnim bundles the five property transitions that drive a Window:
// position (X, Y), size (W, H), and alpha. Methods are intent-level
// (OpenFrom, CloseTo, SnapOpen) so window code never pokes individual
// properties — and retargeting is uniform: every dimension rebases from
// its current value, so a mid-open Dismiss reverses smoothly.
type WindowAnim struct {
	posX, posY   tween.Transition
	sizeW, sizeH tween.Transition
	alpha        tween.Transition
}

// OpenFrom begins the icon-zoom open: position and size start at iconPos /
// iconSize, then animate to the centre and full extent of screen. Alpha is
// snapped to 1 (the open animation does not fade).
func (a *WindowAnim) OpenFrom(iconPos, iconSize, screen draws.XY, dur time.Duration) {
	a.posX.Snap(iconPos.X)
	a.posY.Snap(iconPos.Y)
	a.sizeW.Snap(iconSize.X)
	a.sizeH.Snap(iconSize.Y)
	a.alpha.Snap(1)

	ease := tween.EaseOutExponential
	a.posX.To(screen.X/2, dur, ease)
	a.posY.To(screen.Y/2, dur, ease)
	a.sizeW.To(screen.X, dur, ease)
	a.sizeH.To(screen.Y, dur, ease)
}

// CloseTo retargets every property to shrink toward targetCenter / targetSize
// and fade out over dur. Safe to call mid-open: each property rebases from
// its current value, producing a continuous reversal rather than a snap.
func (a *WindowAnim) CloseTo(targetCenter, targetSize draws.XY, dur time.Duration) {
	ease := tween.EaseOutExponential
	a.posX.To(targetCenter.X, dur, ease)
	a.posY.To(targetCenter.Y, dur, ease)
	a.sizeW.To(targetSize.X, dur, ease)
	a.sizeH.To(targetSize.Y, dur, ease)
	a.alpha.To(0, dur, ease)
}

// SnapOpen sets the window to the fully-open state instantly (no animation).
// Used when restoring an active window after a display-mode change.
func (a *WindowAnim) SnapOpen(screen draws.XY) {
	a.posX.Snap(screen.X / 2)
	a.posY.Snap(screen.Y / 2)
	a.sizeW.Snap(screen.X)
	a.sizeH.Snap(screen.Y)
	a.alpha.Snap(1)
}

func (a *WindowAnim) Pos() draws.XY  { return draws.XY{X: a.posX.Value(), Y: a.posY.Value()} }
func (a *WindowAnim) Size() draws.XY { return draws.XY{X: a.sizeW.Value(), Y: a.sizeH.Value()} }
func (a *WindowAnim) Alpha() float64 { return a.alpha.Value() }

// Done reports whether all property transitions have reached their target.
func (a *WindowAnim) Done() bool {
	return a.posX.Done() && a.posY.Done() &&
		a.sizeW.Done() && a.sizeH.Done() &&
		a.alpha.Done()
}
