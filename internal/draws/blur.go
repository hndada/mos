package draws

import "github.com/hajimehoshi/ebiten/v2"

// Blur applies a fast frosted-glass blur via bilinear downsample → upsample.
//
// The technique: render the source image to a 1/factor-size intermediate,
// then scale it back up with linear filtering. Because bilinear filtering
// blends neighbouring pixels during both steps, the result approximates a
// Gaussian blur at a fraction of the cost of a true convolution.
//
//	factor = 4   → light blur (subtle glass)
//	factor = 8   → moderate blur (curtain / drawer panel)
//	factor = 16  → heavy blur (full-screen modal backdrop)
//
// src and dst must not be the same image. The internal intermediate buffer
// is lazily allocated and reused across frames; it is reallocated only when
// the source size changes.
type Blur struct {
	factor int
	small  Image // intermediate: 1/factor of source size
}

func NewBlur(factor int) *Blur {
	if factor < 2 {
		factor = 2
	}
	return &Blur{factor: factor}
}

// Apply draws a blurred version of src onto dst.
// dst is not cleared first — call dst.Clear() before Apply if you want a
// fresh surface.
func (b *Blur) Apply(src, dst Image) {
	if src.IsEmpty() || dst.IsEmpty() {
		return
	}
	size := src.Size()
	smW := max(1.0, size.X/float64(b.factor))
	smH := max(1.0, size.Y/float64(b.factor))

	// Re-allocate the intermediate buffer only when the source size changes.
	if b.small.IsEmpty() || b.small.Size() != (XY{X: smW, Y: smH}) {
		b.small = CreateImage(smW, smH)
	}
	b.small.Clear()

	// ── Pass 1: downsample src → small ───────────────────────────────────
	down := NewSprite(src)
	down.Locate(smW/2, smH/2, CenterMiddle)
	down.Size = XY{X: smW, Y: smH}
	down.Filter = ebiten.FilterLinear
	down.Draw(b.small)

	// ── Pass 2: upsample small → dst ─────────────────────────────────────
	up := NewSprite(b.small)
	up.Locate(size.X/2, size.Y/2, CenterMiddle)
	up.Size = size
	up.Filter = ebiten.FilterLinear
	up.Draw(dst)
}
