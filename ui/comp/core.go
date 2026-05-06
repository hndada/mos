// Package comp provides a declarative component model for building UIs.
//
// Design: identical to React / Jetpack Compose — you describe what the UI
// should look like (Build returns a Node tree), and the Renderer handles
// layout, input routing, and drawing each frame.
//
//	type CounterApp struct{ n int }
//
//	func (a *CounterApp) Build() comp.Node {
//	    return comp.Column(8,
//	        comp.Label(fmt.Sprintf("Count: %d", a.n), comp.FontSize(18)),
//	        comp.Btn("Increment", func() { a.n++ }),
//	    )
//	}
//
// Custom components are ordinary Go functions that return a Node:
//
//	func Header(title string) comp.Node {
//	    return comp.PaddingAll(16, comp.Label(title, comp.FontSize(20)))
//	}
package comp

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/internal/draws"
)

// ── Geometry ──────────────────────────────────────────────────────────────────

// Rect is a screen-space axis-aligned rectangle.
type Rect struct{ X, Y, W, H float64 }

func (r Rect) Contains(p draws.XY) bool {
	return p.X >= r.X && p.X < r.X+r.W &&
		p.Y >= r.Y && p.Y < r.Y+r.H
}

// ── Interaction ───────────────────────────────────────────────────────────────

// IA (Interaction) carries per-frame UI state for one widget.
// The Renderer populates it from retained state before each Draw.
type IA struct {
	Pressed bool // finger/mouse is currently held down on this widget
	Focused bool // this widget holds keyboard focus
}

// ── Widget interface ──────────────────────────────────────────────────────────

// Widget is implemented by every concrete widget type.
// Implementations are stateless value types created fresh each frame.
type Widget interface {
	// measure returns the preferred (width, height) given available space.
	// Pass a very large number for unconstrained axes.
	measure(maxW, maxH float64) (float64, float64)

	// place receives the final allocated Rect and returns a fully laid-out
	// subtree ready for drawing and hit-testing.
	place(r Rect, path string) *placed
}

// ── Node ──────────────────────────────────────────────────────────────────────

// Node is an immutable UI element description, built fresh each frame.
// It wraps a Widget together with layout modifiers (key, flex weight).
type Node struct {
	w    Widget
	key  string  // explicit stable identity; uses tree path when empty
	flex float64 // flex weight inside Row/Column; 0 = intrinsic size
}

// Key attaches an explicit identity key for stable retained state.
func (n Node) Key(k string) Node { n.key = k; return n }

// Flex marks this node as stretchy with the given weight in its parent.
func (n Node) Flex(weight float64) Node { n.flex = weight; return n }

// ── Placed tree ───────────────────────────────────────────────────────────────

// placed is a laid-out widget: its screen Rect is known, its children are
// placed recursively, and it carries the callbacks the Renderer needs.
type placed struct {
	rect     Rect
	path     string
	drawFn   func(dst draws.Image, ia IA) // nil for pure containers
	onClick  func()                        // non-nil → widget is tappable
	isFocus  bool                          // non-false → widget accepts keyboard focus
	children []*placed
}

// hitTest returns the deepest interactive placed node at screen point pt,
// or nil if pt is outside this subtree or hits no interactive node.
func (p *placed) hitTest(pt draws.XY) *placed {
	if !p.rect.Contains(pt) {
		return nil
	}
	// Children with higher index are drawn on top — check in reverse.
	for i := len(p.children) - 1; i >= 0; i-- {
		if h := p.children[i].hitTest(pt); h != nil {
			return h
		}
	}
	if p.onClick != nil || p.isFocus {
		return p
	}
	return nil
}

// drawAll recursively draws the subtree. ia maps a node path to its IA.
func (p *placed) drawAll(dst draws.Image, ia func(string) IA) {
	if p.drawFn != nil {
		p.drawFn(dst, ia(p.path))
	}
	for _, c := range p.children {
		c.drawAll(dst, ia)
	}
}

// ── Low-level drawing helpers ─────────────────────────────────────────────────

// solidWhite is a 1×1 white pixel reused for all fillRect calls.
// Scaling via GeoM and colouring via ColorScale avoids per-frame allocations.
var solidWhite *ebiten.Image

func init() {
	solidWhite = ebiten.NewImage(1, 1)
	solidWhite.Fill(color.White)
}

// fillRect draws a solid-colour rectangle directly onto dst.
func fillRect(dst draws.Image, r Rect, clr color.RGBA) {
	if dst.IsEmpty() || r.W <= 0 || r.H <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(r.W, r.H)
	op.GeoM.Translate(r.X, r.Y)
	op.ColorScale.Scale(
		float32(clr.R)/255,
		float32(clr.G)/255,
		float32(clr.B)/255,
		float32(clr.A)/255,
	)
	dst.DrawImage(solidWhite, op)
}

// drawText positions a draws.Text at (x, y) with the given anchor and draws it.
func drawText(dst draws.Image, t draws.Text, x, y float64, align draws.Aligns) {
	t.Locate(x, y, align)
	t.Draw(dst)
}
