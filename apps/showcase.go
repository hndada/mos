package apps

// ShowcaseApp demonstrates the declarative component model (ui/comp).
//
// Compare with settings.go (imperative) — this achieves the same kind of
// settings list in ~60 lines of application code with zero manual layout.
//
// Register: mosapp.Register("showcase", NewShowcaseApp)
// Launch:   tap the Showcase icon on the home screen (see register.go).

import (
	"fmt"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/ui/comp"
)

// ShowcaseApp is entirely declarative: state lives in plain fields;
// Build() is a pure function from state → Node tree.
type ShowcaseApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64
	ui      *comp.Renderer

	// ── App state ──────────────────────────────────────────────────────────
	darkMode   bool
	bluetooth  bool
	wifi       bool
	brightness float64 // 0–1
	volume     float64 // 0–1
	tapCount   int
	noticeText string
}

func NewShowcaseApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	app := &ShowcaseApp{
		ctx:        ctx,
		screenW:    sz.X,
		screenH:    sz.Y,
		brightness: 0.65,
		volume:     0.4,
	}
	app.ui = comp.NewRenderer(sz.X, sz.Y, app.Build)
	return app
}

// Build returns the complete UI description for the current state.
// This is the only place that "knows" what the screen looks like.
func (a *ShowcaseApp) Build() comp.Node {
	return comp.Column(0,
		// ── App bar ─────────────────────────────────────────────────────
		comp.SizedBox(0, 56,
			comp.PaddingAll(16,
				comp.Row(0,
					comp.Label("Showcase", comp.FontSize(20)).Flex(1),
					comp.Label(fmt.Sprintf("taps: %d", a.tapCount),
						comp.FontSize(13), comp.Muted()),
				),
			),
		),
		comp.Divider(),

		// ── Quick toggles ────────────────────────────────────────────────
		comp.PaddingH(16, comp.Label("QUICK SETTINGS",
			comp.FontSize(11), comp.Muted())),
		comp.Spacer(4),
		comp.ListTile("Wi-Fi",
			comp.Subtitle("Connected"),
			comp.Trailing(comp.Toggle(&a.wifi, nil)),
		),
		comp.Divider(),
		comp.ListTile("Bluetooth",
			comp.Trailing(comp.Toggle(&a.bluetooth, nil)),
		),
		comp.Divider(),
		comp.ListTile("Dark Mode",
			comp.Trailing(comp.Toggle(&a.darkMode, func(on bool) {
				a.ctx.Bus().Publish(event.System{
					Topic: event.TopicDarkMode, Value: on,
				})
			})),
		),
		comp.Divider(),

		// ── Sliders (display-only progress bars with labels) ─────────────
		comp.Spacer(12),
		comp.PaddingH(16, comp.Label("DISPLAY",
			comp.FontSize(11), comp.Muted())),
		comp.Spacer(4),
		comp.PaddingAll(16,
			comp.Column(6,
				comp.Row(8,
					comp.Label("Brightness", comp.FontSize(14)).Flex(1),
					comp.Label(fmt.Sprintf("%.0f%%", a.brightness*100),
						comp.FontSize(13), comp.Muted()),
				),
				comp.ProgressBar(a.brightness, comp.BarH(8)),
			),
		),
		comp.PaddingAll(16,
			comp.Column(6,
				comp.Row(8,
					comp.Label("Volume", comp.FontSize(14)).Flex(1),
					comp.Label(fmt.Sprintf("%.0f%%", a.volume*100),
						comp.FontSize(13), comp.Muted()),
				),
				comp.ProgressBar(a.volume, comp.BarH(8)),
			),
		),
		comp.Divider(),

		// ── Buttons ──────────────────────────────────────────────────────
		comp.Spacer(12),
		comp.PaddingAll(16,
			comp.Row(12,
				comp.Btn("Tap me", func() {
					a.tapCount++
					a.noticeText = fmt.Sprintf("Tapped %d times", a.tapCount)
					a.ctx.PostNotice(mosapp.Notice{Title: "Showcase", Body: a.noticeText})
				}, comp.BtnFull()).Flex(1),
				comp.Btn("Reset", func() {
					a.tapCount = 0
					a.noticeText = ""
				}, comp.BtnFull()).Flex(1),
			),
		),

		// ── Conditional ──────────────────────────────────────────────────
		comp.If(a.noticeText != "",
			comp.PaddingAll(16,
				comp.Label(a.noticeText, comp.FontSize(13), comp.Muted()),
			),
		),

		// ── Status line ──────────────────────────────────────────────────
		comp.Expand(),
		comp.Divider(),
		comp.SizedBox(0, 36,
			comp.Center(
				comp.Label(
					fmt.Sprintf("dark=%v  bt=%v  wifi=%v",
						a.darkMode, a.bluetooth, a.wifi),
					comp.FontSize(11), comp.Muted()),
			),
		),
	)
}

// ── mosapp.Content interface ──────────────────────────────────────────────────

func (a *ShowcaseApp) Update(frame mosapp.Frame) { a.ui.Update(frame) }
func (a *ShowcaseApp) Draw(dst draws.Image)      { a.ui.Draw(dst) }

func (a *ShowcaseApp) OnCreate()  {}
func (a *ShowcaseApp) OnResume()  {}
func (a *ShowcaseApp) OnPause()   {}
func (a *ShowcaseApp) OnDestroy() {}
