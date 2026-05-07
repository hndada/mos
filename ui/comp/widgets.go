package comp

import (
	"image/color"
	"math"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui/theme"
)

// ── Colour palette ────────────────────────────────────────────────────────────
//
// Each function reads from the active theme so that a theme switch (e.g.
// dark→light) is reflected on the very next frame without any explicit refresh.

func colText() color.RGBA     { return theme.Active().Color(theme.TextPrimary) }
func colMuted() color.RGBA    { return theme.Active().Color(theme.TextSecondary) }
func colBtnBg() color.RGBA    { return theme.Active().Color(theme.SurfaceWidget) }
func colBtnPress() color.RGBA { return theme.Active().Color(theme.SurfaceTint) }
func colAccent() color.RGBA   { return theme.Active().Color(theme.Accent) }
func colDivider() color.RGBA  { return theme.Active().Color(theme.Divider) }
func colTrackOff() color.RGBA { return theme.Active().Color(theme.SurfaceWidget) }
func colTileHold() color.RGBA { return theme.Active().Color(theme.PressOverlay) }
func colBarTrack() color.RGBA { return theme.Active().Color(theme.SurfaceWidget) }

// ── Label ─────────────────────────────────────────────────────────────────────

// Label displays a single line of text.
//
//	comp.Label("Hello", comp.FontSize(18), comp.Bold())
//	comp.Label("hint", comp.Muted(), comp.FontSize(12))
func Label(text string, opts ...LabelOpt) Node {
	lw := &labelW{text: text, size: 14, clr: colText()}
	for _, o := range opts {
		o(lw)
	}
	return Node{w: lw}
}

// labelW is the Widget behind Label(). It memoizes the draws.Text it builds
// so that layout's two-pass measure→place cycle only calls draws.NewText once
// per label per frame rather than twice.
type labelW struct {
	text  string
	size  float64
	clr   color.RGBA
	built bool       // true after first textEl() call within this frame build
	el    draws.Text // memoized result; valid when built == true
}

// LabelOpt is a functional option for Label.
type LabelOpt func(*labelW)

func FontSize(s float64) LabelOpt     { return func(lw *labelW) { lw.size = s } }
func FontColor(c color.RGBA) LabelOpt { return func(lw *labelW) { lw.clr = c } }
func Muted() LabelOpt                 { return func(lw *labelW) { lw.clr = colMuted() } }

// textEl returns the memoized draws.Text, building it on the first call.
// Subsequent calls within the same frame build return the cached object,
// saving one draws.NewText / LoadFace / text.Measure round-trip.
func (lw *labelW) textEl() draws.Text {
	if !lw.built {
		opts := draws.NewFaceOptions()
		opts.Size = lw.size
		lw.el = draws.NewText(lw.text)
		lw.el.SetFace(opts)
		lw.built = true
	}
	return lw.el
}

func (lw *labelW) measure(_, _ float64) (float64, float64) {
	// Box.Size was pre-computed by SetFace; no extra text.Measure call needed.
	sz := lw.textEl().Box.Size
	return sz.X, sz.Y
}

func (lw *labelW) place(r Rect, path string) *placed {
	t := lw.textEl()
	clr := lw.clr // snapshot theme colour at layout time (comp rebuilds each frame)
	return &placed{
		rect: r,
		path: path,
		drawFn: func(dst draws.Image, _ IA) {
			tt := t
			tt.Locate(r.X, r.Y+r.H/2, draws.LeftMiddle)
			cr, cg, cb, ca := theme.ScaleOf(clr)
			tt.ColorScale.Scale(cr, cg, cb, ca)
			tt.Draw(dst)
		},
	}
}

// ── ColorBox ──────────────────────────────────────────────────────────────────

// ColorBox fills its entire rect with a solid colour. Useful for backgrounds,
// dividers, and decorative blocks.
func ColorBox(clr color.RGBA) Node { return Node{w: &colorBoxW{clr}} }

type colorBoxW struct{ clr color.RGBA }

func (c *colorBoxW) measure(maxW, maxH float64) (float64, float64) {
	return maxW, maxH
}
func (c *colorBoxW) place(r Rect, path string) *placed {
	clr := c.clr
	return &placed{
		rect: r,
		path: path,
		drawFn: func(dst draws.Image, _ IA) {
			fillRect(dst, r, clr)
		},
	}
}

// ── Divider ───────────────────────────────────────────────────────────────────

// Divider renders a 1 px hairline separator. Place it in a Column.
func Divider() Node { return Node{w: &dividerW{}} }

type dividerW struct{}

func (d *dividerW) measure(maxW, _ float64) (float64, float64) { return maxW, 1 }
func (d *dividerW) place(r Rect, path string) *placed {
	return &placed{
		rect: r,
		path: path,
		drawFn: func(dst draws.Image, _ IA) {
			fillRect(dst, Rect{r.X, r.Y, r.W, 1}, colDivider())
		},
	}
}

// ── Btn ───────────────────────────────────────────────────────────────────────

// Btn is a tappable button with a text label. onTap is called synchronously
// when the user releases inside the button's rect. Pass nil for a no-op button.
//
//	comp.Btn("Save", func() { app.save() })
//	comp.Btn("Cancel", nil)
func Btn(label string, onTap func(), opts ...BtnOpt) Node {
	bw := &btnW{label: label, onTap: onTap, h: 44}
	for _, o := range opts {
		o(bw)
	}
	return Node{w: bw}
}

// btnW is the Widget behind Btn(). Like labelW it memoizes its draws.Text
// so the measure→place two-pass layout only allocates once per button per frame.
type btnW struct {
	label  string
	onTap  func()
	h      float64
	full   bool // expand to full width
	accent bool // use Accent colour as background
	built  bool
	el     draws.Text
}

// BtnOpt is a functional option for Btn.
type BtnOpt func(*btnW)

// BtnH overrides the button height (default 44).
func BtnH(h float64) BtnOpt { return func(b *btnW) { b.h = h } }

// BtnFull makes the button expand to the full available width.
func BtnFull() BtnOpt { return func(b *btnW) { b.full = true } }

// BtnAccent styles the button with the theme Accent colour as its background
// and OnAccent as its text colour (e.g. a primary call-to-action button).
func BtnAccent() BtnOpt { return func(b *btnW) { b.accent = true } }

func (b *btnW) textEl() draws.Text {
	if !b.built {
		opts := draws.NewFaceOptions()
		opts.Size = 15
		b.el = draws.NewText(b.label)
		b.el.SetFace(opts)
		b.built = true
	}
	return b.el
}

func (b *btnW) measure(maxW, _ float64) (float64, float64) {
	tw := b.textEl().Box.Size.X + 32
	if b.full || tw > maxW {
		tw = maxW
	}
	return tw, b.h
}

func (b *btnW) place(r Rect, path string) *placed {
	t := b.textEl()
	onTap := b.onTap
	accent := b.accent
	return &placed{
		rect:    r,
		path:    path,
		onClick: onTap,
		drawFn: func(dst draws.Image, ia IA) {
			var bg color.RGBA
			if accent {
				bg = colAccent()
				if ia.Pressed {
					// Darken the accent slightly on press.
					bg.R = uint8(float32(bg.R) * 0.82)
					bg.G = uint8(float32(bg.G) * 0.82)
					bg.B = uint8(float32(bg.B) * 0.82)
				}
			} else {
				bg = colBtnBg()
				if ia.Pressed {
					bg = colBtnPress()
				}
			}
			fillRect(dst, r, bg)
			tt := t
			tt.Locate(r.X+r.W/2, r.Y+r.H/2, draws.CenterMiddle)
			if accent {
				cr, cg, cb, ca := theme.ScaleOf(theme.Active().Color(theme.OnAccent))
				tt.ColorScale.Scale(cr, cg, cb, ca)
			}
			tt.Draw(dst)
		},
	}
}

// ── Toggle ────────────────────────────────────────────────────────────────────

// Toggle is a boolean on/off switch bound to *value.
// onChange is called after the value flips (may be nil).
//
//	comp.Toggle(&app.darkMode, func(v bool) { bus.Publish(...) })
func Toggle(value *bool, onChange func(bool)) Node {
	return Node{w: &toggleW{value: value, onChange: onChange}}
}

const toggleW_ = 50.0
const toggleH_ = 28.0

type toggleW struct {
	value    *bool
	onChange func(bool)
}

func (t *toggleW) measure(_, _ float64) (float64, float64) { return toggleW_, toggleH_ }

func (t *toggleW) place(r Rect, path string) *placed {
	val := t.value
	cb := t.onChange
	return &placed{
		rect: r,
		path: path,
		onClick: func() {
			*val = !*val
			if cb != nil {
				cb(*val)
			}
		},
		drawFn: func(dst draws.Image, ia IA) {
			on := *val
			// Track
			track := colTrackOff()
			if on {
				track = colAccent()
			}
			fillRect(dst, r, track)
			// Knob
			kSize := r.H - 4
			kX := r.X + 2
			if on {
				kX = r.X + r.W - kSize - 2
			}
			if ia.Pressed {
				// Slight visual feedback
				kSize -= 2
				kX += 1
			}
			fillRect(dst, Rect{kX, r.Y + 2, kSize, kSize}, theme.Active().Color(theme.Knob))
		},
	}
}

// ── ListTile ──────────────────────────────────────────────────────────────────

// ListTileOpt is a functional option for ListTile.
type ListTileOpt func(*listTileOpts)

type listTileOpts struct {
	subtitle string
	trailing Node  // widget placed at the right edge (e.g. Toggle, Label)
	onTap    func()
	h        float64
}

// Subtitle adds a secondary line of text below the title.
func Subtitle(s string) ListTileOpt { return func(o *listTileOpts) { o.subtitle = s } }

// Trailing places a widget at the right edge of the tile (e.g. a Toggle).
func Trailing(n Node) ListTileOpt { return func(o *listTileOpts) { o.trailing = n } }

// OnTap registers a tap callback on the tile itself.
func OnTap(fn func()) ListTileOpt { return func(o *listTileOpts) { o.onTap = fn } }

// TileH overrides the tile height (default 56).
func TileH(h float64) ListTileOpt { return func(o *listTileOpts) { o.h = h } }

// ListTile is a standard list row with a title, optional subtitle, and
// optional trailing widget. It is itself a component — a function returning
// a Node built from primitives.
//
//	comp.ListTile("Dark Mode",
//	    comp.Trailing(comp.Toggle(&app.darkMode, nil)),
//	    comp.Subtitle("Changes the colour theme"),
//	)
func ListTile(title string, opts ...ListTileOpt) Node {
	o := &listTileOpts{h: 56}
	for _, opt := range opts {
		opt(o)
	}
	return Node{w: &listTileW{title: title, opts: o}}
}

type listTileW struct {
	title string
	opts  *listTileOpts
}

func (lt *listTileW) measure(maxW, _ float64) (float64, float64) { return maxW, lt.opts.h }

func (lt *listTileW) place(r Rect, path string) *placed {
	o := lt.opts

	// Body node tree — built inline as components calling components.
	const padX = 16.0

	// Title (and optional subtitle) in a Column.
	var textNode Node
	if o.subtitle != "" {
		textNode = Column(2,
			Label(lt.title, FontSize(15)),
			Label(o.subtitle, FontSize(12), Muted()),
		)
	} else {
		textNode = Label(lt.title, FontSize(15))
	}

	// Main row: text (flex) + optional trailing.
	var rowChildren []Node
	rowChildren = append(rowChildren, textNode.Flex(1))
	if o.trailing.w != nil {
		rowChildren = append(rowChildren, Center(o.trailing))
	}
	body := Padding(0, padX, 0, padX,
		Row(12, rowChildren...),
	)

	inner := body.w.place(r, path+".body")

	onTap := o.onTap
	return &placed{
		rect:     r,
		path:     path,
		onClick:  onTap,
		children: []*placed{inner},
		drawFn: func(dst draws.Image, ia IA) {
			if ia.Pressed {
				fillRect(dst, r, colTileHold())
			}
		},
	}
}

// ── ProgressBar ───────────────────────────────────────────────────────────────

// ProgressBar displays a horizontal fill bar. value must be in [0, 1].
//
//	comp.ProgressBar(app.loadProgress)
func ProgressBar(value float64, opts ...ProgressOpt) Node {
	po := &progressOpts{h: 6, clr: colAccent()}
	for _, o := range opts {
		o(po)
	}
	return Node{w: &progressW{value: math.Max(0, math.Min(1, value)), opts: po}}
}

type progressOpts struct {
	h   float64
	clr color.RGBA
}

// ProgressOpt is a functional option for ProgressBar.
type ProgressOpt func(*progressOpts)

// BarH sets the bar height (default 6).
func BarH(h float64) ProgressOpt { return func(o *progressOpts) { o.h = h } }

// BarColor sets the fill colour.
func BarColor(c color.RGBA) ProgressOpt { return func(o *progressOpts) { o.clr = c } }

type progressW struct {
	value float64
	opts  *progressOpts
}

func (pw *progressW) measure(maxW, _ float64) (float64, float64) { return maxW, pw.opts.h }
func (pw *progressW) place(r Rect, path string) *placed {
	v := pw.value
	clr := pw.opts.clr
	return &placed{
		rect: r,
		path: path,
		drawFn: func(dst draws.Image, _ IA) {
			fillRect(dst, r, colBarTrack())
			fillRect(dst, Rect{r.X, r.Y, r.W * v, r.H}, clr)
		},
	}
}

// ── If / IfElse ───────────────────────────────────────────────────────────────

// If conditionally includes a node. When cond is false the node occupies no
// space and draws nothing. Use .Key() on the returned Node for stable identity.
//
//	comp.If(app.loggedIn, comp.Label("Welcome back!"))
func If(cond bool, then Node) Node {
	if cond {
		return then
	}
	return Node{w: &spacerW{0}}
}

// IfElse returns then when cond is true, otherwise els.
func IfElse(cond bool, then, els Node) Node {
	if cond {
		return then
	}
	return els
}
