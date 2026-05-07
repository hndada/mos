// Package theme provides a finite set of semantic colour tokens and built-in
// dark / light themes for MOS widgets.
//
// Every widget that renders colour should read from the active theme rather
// than hard-coding RGBA literals:
//
//	clr := theme.Active().Color(theme.Accent)
//
// To switch themes at runtime (e.g. dark-mode toggle):
//
//	theme.Set(theme.Light())
//	ws.InvalidateAll() // trigger a redraw of all windows
package theme

import "image/color"

// Token is a semantic colour slot. Using named tokens instead of raw RGBA
// literals keeps widgets theme-agnostic and makes palette updates trivial.
type Token int

const (
	TextPrimary   Token = iota // main body text
	TextSecondary              // muted / hint / subtitle text
	SurfaceWidget              // unselected track, checkbox box, and plain button background
	SurfaceInput               // text-field / search-field background
	SurfaceTint                // pressed / active surface overlay
	Accent                     // primary interactive colour (blue)
	AccentSuccess              // on-state indicator (green)
	OnAccent                   // text or icon placed on an Accent surface
	PressOverlay               // translucent tap-highlight on list tiles
	Divider                    // hairline separator lines
	Knob                       // toggle thumb / slider thumb
	Background                 // deep background; hollow radio-button centre

	numTokens // sentinel — do not use directly
)

// Theme maps every Token to a concrete RGBA colour.
type Theme struct {
	colors [numTokens]color.RGBA
}

// Color returns the RGBA value for the given semantic token.
func (t *Theme) Color(tok Token) color.RGBA { return t.colors[tok] }

// active holds the current global theme. It starts as Dark() and is replaced
// by Set(). All reads go through Active() so the pointer indirection is hidden.
var active = Dark()

// Active returns a pointer to the current global theme. The pointer address is
// stable for the lifetime of the process; it is safe to cache.
func Active() *Theme { return &active }

// Set replaces the active global theme. Widgets built in immediate-mode (comp
// package) pick up the new colours on the next frame automatically. Retained
// widgets (ui package) built with theme colours at construction time need to be
// rebuilt, or their owning window must call Invalidate() so Update re-runs.
func Set(t Theme) { active = t }

// ── Built-in themes ───────────────────────────────────────────────────────────

// Dark returns the built-in dark-mode theme (the default at startup).
func Dark() Theme {
	var t Theme
	t.colors[TextPrimary]   = color.RGBA{235, 237, 240, 255}
	t.colors[TextSecondary] = color.RGBA{140, 145, 158, 255}
	t.colors[SurfaceWidget] = color.RGBA{72, 72, 74, 255}
	t.colors[SurfaceInput]  = color.RGBA{44, 46, 56, 255}
	t.colors[SurfaceTint]   = color.RGBA{72, 80, 98, 255}
	t.colors[Accent]        = color.RGBA{10, 132, 255, 255}
	t.colors[AccentSuccess] = color.RGBA{52, 199, 89, 255}
	t.colors[OnAccent]      = color.RGBA{255, 255, 255, 255}
	t.colors[PressOverlay]  = color.RGBA{255, 255, 255, 18}
	t.colors[Divider]       = color.RGBA{55, 60, 70, 255}
	t.colors[Knob]          = color.RGBA{255, 255, 255, 235}
	t.colors[Background]    = color.RGBA{28, 28, 32, 255}
	return t
}

// Light returns the built-in light-mode theme.
func Light() Theme {
	var t Theme
	t.colors[TextPrimary]   = color.RGBA{20, 22, 28, 255}
	t.colors[TextSecondary] = color.RGBA{100, 105, 118, 255}
	t.colors[SurfaceWidget] = color.RGBA{200, 202, 208, 255}
	t.colors[SurfaceInput]  = color.RGBA{240, 241, 245, 255}
	t.colors[SurfaceTint]   = color.RGBA{180, 188, 210, 255}
	t.colors[Accent]        = color.RGBA{0, 122, 255, 255}
	t.colors[AccentSuccess] = color.RGBA{52, 199, 89, 255}
	t.colors[OnAccent]      = color.RGBA{255, 255, 255, 255}
	t.colors[PressOverlay]  = color.RGBA{0, 0, 0, 18}
	t.colors[Divider]       = color.RGBA{198, 200, 208, 255}
	t.colors[Knob]          = color.RGBA{255, 255, 255, 255}
	t.colors[Background]    = color.RGBA{245, 245, 250, 255}
	return t
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ScaleOf converts c into (r, g, b, a) float32 components in [0, 1].
// Pass the results directly to ebiten.ColorScale.Scale():
//
//	sp.ColorScale.Scale(theme.ScaleOf(theme.Active().Color(theme.Accent)))
func ScaleOf(c color.RGBA) (r, g, b, a float32) {
	return float32(c.R) / 255,
		float32(c.G) / 255,
		float32(c.B) / 255,
		float32(c.A) / 255
}
