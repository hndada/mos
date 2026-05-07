package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

const vlWheelSpeed = 40.0

// VirtualList is a fixed-size vertical scroll viewport that renders only the
// content that intersects the visible area. It allocates a canvas sized to
// the viewport (not the full content height), so a 1 000-row list costs the
// same GPU memory as a 10-row list.
//
// Usage — each frame:
//
//	// 1. process scroll input
//	vl.Update(frame)
//
//	// 2. paint only visible items
//	vl.Begin(bgColor)
//	for _, item := range items {
//	    top, bottom := item.Y, item.Y+item.H
//	    if !vl.InViewport(top, bottom) {
//	        continue
//	    }
//	    yc := vl.ContentToCanvas(item.Y) // canvas-local Y
//	    item.DrawAt(vl.Canvas(), yc)
//	}
//
//	// 3. blit to screen
//	vl.Draw(dst)
type VirtualList struct {
	// X, Y are the viewport's top-left in the parent (screen) coordinate space.
	X, Y float64
	// W, H are the viewport dimensions. Set once; resize by changing these and
	// calling Begin — the canvas is reallocated automatically.
	W, H float64

	// TotalH is the summed height of all content items.
	// Keep it up to date when items are added or removed.
	TotalH float64

	// ScrollY is the current vertical scroll offset in content-space pixels.
	// It is clamped to [0, TotalH-H] by Update and ScrollBy.
	ScrollY float64

	dragging bool
	prevY    float64

	// canvas is viewport-sized (W×H), not content-sized. It is (re)allocated
	// lazily in ensureCanvas whenever W or H changes.
	canvas draws.Image
	cW, cH float64
}

// maxScroll returns the maximum allowed scroll offset.
func (vl *VirtualList) maxScroll() float64 { return max(vl.TotalH-vl.H, 0) }

// ScrollBy shifts ScrollY by delta, clamped to the content bounds.
func (vl *VirtualList) ScrollBy(delta float64) {
	vl.ScrollY = min(max(vl.ScrollY+delta, 0), vl.maxScroll())
}

// VisibleTop is the content-space Y of the viewport's top edge (== ScrollY).
func (vl *VirtualList) VisibleTop() float64 { return vl.ScrollY }

// VisibleBottom is the content-space Y of the viewport's bottom edge.
func (vl *VirtualList) VisibleBottom() float64 { return vl.ScrollY + vl.H }

// InViewport reports whether a content item spanning [top, bottom) overlaps
// the visible area. Items outside return false and should be skipped.
func (vl *VirtualList) InViewport(top, bottom float64) bool {
	return bottom > vl.VisibleTop() && top < vl.VisibleBottom()
}

// ContentToCanvas converts a content-space Y into a canvas-local Y by
// subtracting the current scroll offset. Pass the result as the yOffset
// argument to widget DrawAt methods.
func (vl *VirtualList) ContentToCanvas(contentY float64) float64 {
	return contentY - vl.ScrollY
}

// Update processes wheel and drag events to adjust ScrollY. Drag tracking
// begins only when the initial touch lands inside the viewport rect.
func (vl *VirtualList) Update(frame mosapp.Frame) {
	for _, ev := range frame.Events {
		switch ev.Kind {
		case input.EventWheel:
			vl.ScrollBy(-ev.Wheel.Y * vlWheelSpeed)
		case input.EventDown:
			lx := ev.Pos.X - vl.X
			ly := ev.Pos.Y - vl.Y
			if lx >= 0 && lx < vl.W && ly >= 0 && ly < vl.H {
				vl.dragging = true
				vl.prevY = ev.Pos.Y
			}
		case input.EventMove:
			if vl.dragging {
				vl.ScrollBy(vl.prevY - ev.Pos.Y)
				vl.prevY = ev.Pos.Y
			}
		case input.EventUp:
			vl.dragging = false
		}
	}
}

// ensureCanvas allocates or reallocates the internal canvas when W or H change.
func (vl *VirtualList) ensureCanvas() {
	if vl.canvas.IsEmpty() || vl.cW != vl.W || vl.cH != vl.H {
		vl.canvas = draws.CreateImage(vl.W, vl.H)
		vl.cW, vl.cH = vl.W, vl.H
	}
}

// Canvas returns the viewport-sized render target. Draw visible items into it
// using canvas-local Y coordinates (see ContentToCanvas). Call Begin first.
func (vl *VirtualList) Canvas() draws.Image {
	vl.ensureCanvas()
	return vl.canvas
}

// Begin clears the canvas with bg. Call once at the start of each Draw cycle
// before rendering any items.
func (vl *VirtualList) Begin(bg color.RGBA) {
	vl.ensureCanvas()
	vl.canvas.Fill(bg)
}

// Draw blits the rendered canvas to dst at the viewport's screen-space (X, Y).
func (vl *VirtualList) Draw(dst draws.Image) {
	if vl.canvas.IsEmpty() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(vl.X, vl.Y)
	dst.DrawImage(vl.canvas.Image, op)
}
