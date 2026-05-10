package draws

// Viewport converts relative app coordinates into concrete canvas pixels.
// App code should express placement and sizing as fractions of the current
// window instead of hard-coding one device's pixel dimensions.
type Viewport struct {
	Size XY
}

func NewViewport(w, h float64) Viewport {
	return Viewport{Size: XY{X: w, Y: h}}
}

func (v Viewport) X(frac float64) float64 { return v.Size.X * frac }
func (v Viewport) Y(frac float64) float64 { return v.Size.Y * frac }

// U returns a fraction of the shorter viewport edge. Use it for square-ish
// controls and radii that should keep the same perceived size in either
// orientation.
func (v Viewport) U(frac float64) float64 {
	if v.Size.X < v.Size.Y {
		return v.Size.X * frac
	}
	return v.Size.Y * frac
}

func (v Viewport) Pt(xFrac, yFrac float64) XY {
	return XY{X: v.X(xFrac), Y: v.Y(yFrac)}
}

func (v Viewport) Sz(wFrac, hFrac float64) XY {
	return XY{X: v.X(wFrac), Y: v.Y(hFrac)}
}

func (v Viewport) Rect(xFrac, yFrac, wFrac, hFrac float64) Rect {
	return Rect{
		X: v.X(xFrac),
		Y: v.Y(yFrac),
		W: v.X(wFrac),
		H: v.Y(hFrac),
	}
}

type Rect struct {
	X, Y, W, H float64
}

func (r Rect) Pos() XY  { return XY{X: r.X, Y: r.Y} }
func (r Rect) Size() XY { return XY{X: r.W, Y: r.H} }

func (r Rect) Inset(dx, dy float64) Rect {
	return Rect{
		X: r.X + dx,
		Y: r.Y + dy,
		W: r.W - 2*dx,
		H: r.H - 2*dy,
	}
}
