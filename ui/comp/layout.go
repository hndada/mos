package comp

import (
	"fmt"
	"math"
)

const big = math.MaxFloat32 // "unconstrained" sentinel

// ── Column ────────────────────────────────────────────────────────────────────

// Column stacks children top-to-bottom with gap pixels between them.
// Children with Flex() weight share the remaining height proportionally.
//
//	comp.Column(8,
//	    comp.Label("Title", comp.FontSize(18)),
//	    comp.Divider(),
//	    comp.ListTile("Item", nil).Flex(1),
//	)
func Column(gap float64, children ...Node) Node {
	return Node{w: &columnW{gap: gap, children: children}}
}

type columnW struct {
	gap      float64
	children []Node
}

func (c *columnW) measure(maxW, maxH float64) (float64, float64) {
	totalH := c.gap * float64(max(0, len(c.children)-1))
	maxCW := 0.0
	for _, n := range c.children {
		if n.flex > 0 {
			continue // flex children don't contribute to the intrinsic height
		}
		cw, ch := n.w.measure(maxW, big)
		if cw > maxCW {
			maxCW = cw
		}
		totalH += ch
	}
	if maxW < big && maxCW < maxW {
		maxCW = maxW // columns expand to fill available width
	}
	return maxCW, totalH
}

func (c *columnW) place(r Rect, path string) *placed {
	p := &placed{rect: r, path: path}

	// First pass: measure fixed children, accumulate total flex weight.
	heights := make([]float64, len(c.children))
	fixedH := c.gap * float64(max(0, len(c.children)-1))
	totalFlex := 0.0
	for i, n := range c.children {
		if n.flex > 0 {
			totalFlex += n.flex
		} else {
			_, h := n.w.measure(r.W, big)
			heights[i] = h
			fixedH += h
		}
	}

	// Second pass: distribute remaining height to flex children.
	flexH := math.Max(0, r.H-fixedH)
	for i, n := range c.children {
		if n.flex > 0 {
			heights[i] = flexH * (n.flex / totalFlex)
		}
	}

	// Third pass: place each child.
	y := r.Y
	for i, n := range c.children {
		cr := Rect{X: r.X, Y: y, W: r.W, H: heights[i]}
		cp := childPath(path, i, n.key)
		p.children = append(p.children, n.w.place(cr, cp))
		y += heights[i] + c.gap
	}
	return p
}

// ── Row ───────────────────────────────────────────────────────────────────────

// Row arranges children left-to-right with gap pixels between them.
// Children with Flex() weight share the remaining width proportionally.
//
//	comp.Row(12,
//	    comp.Label("Left"),
//	    comp.Expand(),   // pushes "Right" to the far edge
//	    comp.Label("Right"),
//	)
func Row(gap float64, children ...Node) Node {
	return Node{w: &rowW{gap: gap, children: children}}
}

type rowW struct {
	gap      float64
	children []Node
}

func (c *rowW) measure(maxW, maxH float64) (float64, float64) {
	totalW := c.gap * float64(max(0, len(c.children)-1))
	maxCH := 0.0
	for _, n := range c.children {
		if n.flex > 0 {
			continue
		}
		cw, ch := n.w.measure(big, maxH)
		if ch > maxCH {
			maxCH = ch
		}
		totalW += cw
	}
	return totalW, maxCH
}

func (c *rowW) place(r Rect, path string) *placed {
	p := &placed{rect: r, path: path}

	widths := make([]float64, len(c.children))
	fixedW := c.gap * float64(max(0, len(c.children)-1))
	totalFlex := 0.0
	for i, n := range c.children {
		if n.flex > 0 {
			totalFlex += n.flex
		} else {
			w, _ := n.w.measure(big, r.H)
			widths[i] = w
			fixedW += w
		}
	}

	flexW := math.Max(0, r.W-fixedW)
	for i, n := range c.children {
		if n.flex > 0 {
			widths[i] = flexW * (n.flex / totalFlex)
		}
	}

	x := r.X
	for i, n := range c.children {
		cr := Rect{X: x, Y: r.Y, W: widths[i], H: r.H}
		cp := childPath(path, i, n.key)
		p.children = append(p.children, n.w.place(cr, cp))
		x += widths[i] + c.gap
	}
	return p
}

// ── Padding ───────────────────────────────────────────────────────────────────

// Padding wraps child with the given insets on each edge.
func Padding(top, right, bottom, left float64, child Node) Node {
	return Node{w: &paddingW{top, right, bottom, left, child}}
}

// PaddingAll wraps child with uniform padding on all four edges.
func PaddingAll(v float64, child Node) Node {
	return Padding(v, v, v, v, child)
}

// PaddingH wraps child with horizontal (left+right) padding only.
func PaddingH(h float64, child Node) Node {
	return Padding(0, h, 0, h, child)
}

// PaddingV wraps child with vertical (top+bottom) padding only.
func PaddingV(v float64, child Node) Node {
	return Padding(v, 0, v, 0, child)
}

type paddingW struct {
	top, right, bottom, left float64
	child                    Node
}

func (pw *paddingW) measure(maxW, maxH float64) (float64, float64) {
	cw, ch := pw.child.w.measure(
		maxW-pw.left-pw.right,
		maxH-pw.top-pw.bottom,
	)
	return cw + pw.left + pw.right, ch + pw.top + pw.bottom
}

func (pw *paddingW) place(r Rect, path string) *placed {
	p := &placed{rect: r, path: path}
	inner := Rect{
		X: r.X + pw.left,
		Y: r.Y + pw.top,
		W: r.W - pw.left - pw.right,
		H: r.H - pw.top - pw.bottom,
	}
	p.children = append(p.children, pw.child.w.place(inner, path+".c"))
	return p
}

// ── SizedBox ──────────────────────────────────────────────────────────────────

// SizedBox forces child into exactly (w, h). Pass 0 to keep the child's
// intrinsic size on that axis.
func SizedBox(w, h float64, child Node) Node {
	return Node{w: &sizedBoxW{w, h, child}}
}

type sizedBoxW struct {
	fw, fh float64
	child  Node
}

func (s *sizedBoxW) measure(maxW, maxH float64) (float64, float64) {
	cw, ch := s.child.w.measure(maxW, maxH)
	if s.fw > 0 {
		cw = s.fw
	}
	if s.fh > 0 {
		ch = s.fh
	}
	return cw, ch
}

func (s *sizedBoxW) place(r Rect, path string) *placed {
	p := &placed{rect: r, path: path}
	p.children = append(p.children, s.child.w.place(r, path+".c"))
	return p
}

// ── Spacer / Expand ───────────────────────────────────────────────────────────

// Spacer inserts a fixed-size empty gap.
func Spacer(size float64) Node { return Node{w: &spacerW{size}} }

// Expand is a flex spacer that grows to fill remaining space in Row/Column.
func Expand() Node { return Node{w: &spacerW{0}, flex: 1} }

type spacerW struct{ size float64 }

func (s *spacerW) measure(_, _ float64) (float64, float64) { return s.size, s.size }
func (s *spacerW) place(r Rect, path string) *placed        { return &placed{rect: r, path: path} }

// ── Stack ─────────────────────────────────────────────────────────────────────

// Stack draws children on top of each other in the same Rect.
// Size is determined by the first (bottom-most) child.
func Stack(children ...Node) Node {
	return Node{w: &stackW{children: children}}
}

type stackW struct{ children []Node }

func (s *stackW) measure(maxW, maxH float64) (float64, float64) {
	if len(s.children) == 0 {
		return 0, 0
	}
	return s.children[0].w.measure(maxW, maxH)
}

func (s *stackW) place(r Rect, path string) *placed {
	p := &placed{rect: r, path: path}
	for i, n := range s.children {
		cp := childPath(path, i, n.key)
		p.children = append(p.children, n.w.place(r, cp))
	}
	return p
}

// ── Align ─────────────────────────────────────────────────────────────────────

// Center centres child both horizontally and vertically within the available rect.
func Center(child Node) Node { return Node{w: &alignW{0.5, 0.5, child}} }

// AlignRight aligns child to the right edge, vertically centred.
func AlignRight(child Node) Node { return Node{w: &alignW{1, 0.5, child}} }

type alignW struct {
	ax, ay float64 // 0=start, 0.5=centre, 1=end
	child  Node
}

func (a *alignW) measure(maxW, maxH float64) (float64, float64) {
	return maxW, maxH
}

func (a *alignW) place(r Rect, path string) *placed {
	p := &placed{rect: r, path: path}
	cw, ch := a.child.w.measure(r.W, r.H)
	cr := Rect{
		X: r.X + (r.W-cw)*a.ax,
		Y: r.Y + (r.H-ch)*a.ay,
		W: cw,
		H: ch,
	}
	p.children = append(p.children, a.child.w.place(cr, path+".c"))
	return p
}

// ── helpers ───────────────────────────────────────────────────────────────────

func childPath(parent string, idx int, key string) string {
	if key != "" {
		return key
	}
	return fmt.Sprintf("%s.%d", parent, idx)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
