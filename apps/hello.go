package apps

// HelloApp demonstrates the MOS app API:
//   - Context: screen size, Launch, ShowKeyboard, PostNotice
//   - Lifecycle: OnCreate / OnResume / OnPause / OnDestroy
//   - Event bus: subscribing to system events (dark-mode, navigation)
//
// Register: mosapp.Register("hello", NewHelloApp)
// Launch:   add "hello" to home.go or call ctx.Launch("hello") from another app.

import (
	"image/color"
	"time"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/ui"
)

type HelloApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64

	// UI
	header  draws.Text
	body    draws.Text
	hint    draws.Text
	btnKB   ui.Button
	btnNoti ui.Button

	// State
	resumeCount int
	dark        bool
	unsubs      []func()
}

func NewHelloApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	w, h := sz.X, sz.Y

	large := draws.NewFaceOptions()
	large.Size = 26
	small := draws.NewFaceOptions()
	small.Size = 14

	header := draws.NewText("Hello, MOS!")
	header.SetFace(large)
	header.Locate(w/2, h*0.28, draws.CenterMiddle)

	body := draws.NewText("")
	body.SetFace(small)
	body.Locate(w/2, h*0.42, draws.CenterMiddle)

	hint := draws.NewText("tap a button below")
	hint.SetFace(small)
	hint.Locate(w/2, h*0.52, draws.CenterMiddle)

	btnKB := ui.NewButton("Keyboard", 13, w/2-110, h*0.64, 100, 36, color.RGBA{50, 120, 220, 255})
	btnNoti := ui.NewButton("Notify", 13, w/2+10, h*0.64, 100, 36, color.RGBA{80, 180, 100, 255})

	return &HelloApp{
		ctx:     ctx,
		screenW: w,
		screenH: h,
		header:  header,
		body:    body,
		hint:    hint,
		btnKB:   btnKB,
		btnNoti: btnNoti,
	}
}

// --- Lifecycle ---

func (h *HelloApp) OnCreate(ctx mosapp.Context) {
	// Subscribe to system events for the window's lifetime.
	h.unsubs = append(h.unsubs,
		ctx.Bus().Subscribe(event.KindSystem, func(e event.Event) {
			se := e.(event.System)
			if se.Topic == event.TopicDarkMode {
				if v, ok := se.Value.(bool); ok {
					h.dark = v
				}
			}
		}),
		ctx.Bus().Subscribe(event.KindNavigation, func(e event.Event) {
			ne := e.(event.Navigation)
			switch ne.Action {
			case event.NavHome:
				h.hint.Text = "went home"
			case event.NavBack:
				h.hint.Text = "went back"
			}
		}),
	)
}

func (h *HelloApp) OnResume() {
	h.resumeCount++
	h.body.Text = "resumed " + itoa(h.resumeCount) + "×  |  " + time.Now().Format("15:04:05")
}

func (h *HelloApp) OnPause() {
	h.hint.Text = "paused"
}

func (h *HelloApp) OnDestroy() {
	for _, unsub := range h.unsubs {
		unsub()
	}
	h.unsubs = nil
}

// --- Content ---

func (h *HelloApp) Update(cursor draws.XY) {
	if h.btnKB.Update(cursor) {
		h.ctx.ShowKeyboard()
		h.hint.Text = "keyboard toggled"
	}
	if h.btnNoti.Update(cursor) {
		h.ctx.PostNotice(mosapp.Notice{
			Title: "HelloApp",
			Body:  "Button tapped at " + time.Now().Format("15:04:05"),
		})
		h.hint.Text = "notice posted"
	}
}

func (h *HelloApp) Draw(dst draws.Image) {
	bg := color.RGBA{18, 22, 38, 255}
	if h.dark {
		bg = color.RGBA{8, 8, 10, 255}
	}
	dst.Fill(bg)
	h.header.Draw(dst)
	h.body.Draw(dst)
	h.hint.Draw(dst)
	h.btnKB.Draw(dst, draws.XY{})
	h.btnNoti.Draw(dst, draws.XY{})
}

// itoa is a tiny helper to avoid importing strconv just for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
