package windowing

import (
	"image/color"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
)

// App IDs for apps the windowing server itself references by name.
const (
	AppIDColor     = "color"
	AppIDGallery   = "gallery"
	AppIDSettings  = "settings"
	AppIDCall      = "call"
	AppIDSceneTest = "scene-test"
	AppIDHello     = "hello"
	AppIDShowcase  = "showcase"
	AppIDMessage   = "message"
)

// App wraps an app.Content instance with OS-level metadata (ID, accent colour,
// context). It is the unit the Window owns. Window updates and draws content
// on the main goroutine; Context methods still go through windowProc so app
// code requests OS actions indirectly.
type App struct {
	ID    string
	Color color.RGBA
	ctx   *windowContext

	content mosapp.Content

	// Shown when no registered content matches the ID.
	title    draws.Text
	subtitle draws.Text
}

// AppState is the minimal snapshot of an App that survives a display mode change.
type AppState struct {
	ID    string
	Color color.RGBA
}

// NewApp instantiates an App for the given ID using ctx.
// It first queries the app registry; if the ID is unregistered a colour-fill
// placeholder is used instead. OnCreate is NOT invoked here; the Window fires
// it immediately on startup.
func NewApp(id string, clr color.RGBA, ctx *windowContext) *App {
	if id == "" {
		id = AppIDColor
	}
	a := &App{ID: id, Color: clr, ctx: ctx}

	if content := mosapp.New(id, ctx); content != nil {
		a.content = content
		return a
	}

	// Fallback: solid colour with a centred label.
	a.initPlaceholder(ctx.screenW, ctx.screenH)
	return a
}

func (a *App) initPlaceholder(screenW, screenH float64) {
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 28
	a.title = draws.NewText(appLabel(a.ID))
	a.title.SetFace(titleOpts)
	a.title.Locate(screenW/2, screenH/2-12, draws.CenterMiddle)

	subtitleOpts := draws.NewFaceOptions()
	subtitleOpts.Size = 14
	a.subtitle = draws.NewText("Running")
	a.subtitle.SetFace(subtitleOpts)
	a.subtitle.Locate(screenW/2, screenH/2+24, draws.CenterMiddle)
}

func appLabel(id string) string {
	switch id {
	case AppIDGallery:
		return "Gallery"
	case AppIDSettings:
		return "Settings"
	case AppIDCall:
		return "Call"
	case AppIDSceneTest:
		return "Scene Test"
	case AppIDHello:
		return "Hello"
	case AppIDShowcase:
		return "Showcase"
	case AppIDMessage:
		return "Messages"
	default:
		return "App"
	}
}

// Draw renders the app's content onto dst. Called on the main goroutine after
// Update, so app state mutated during Update is fully visible without a lock.
func (a *App) Draw(dst draws.Image) {
	if a.content != nil {
		a.content.Draw(dst)
		return
	}
	dst.Fill(a.Color)
	a.title.Draw(dst)
	a.subtitle.Draw(dst)
}
