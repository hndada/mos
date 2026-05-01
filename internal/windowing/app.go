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
)

// App wraps an app.Content instance with OS-level metadata (ID, accent colour,
// context). It is the unit the Window owns.
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
// placeholder is used instead.
func NewApp(id string, clr color.RGBA, ctx *windowContext) *App {
	if id == "" {
		id = AppIDColor
	}
	a := &App{ID: id, Color: clr, ctx: ctx}

	if content := mosapp.New(id, ctx); content != nil {
		a.content = content
		if lc, ok := content.(mosapp.Lifecycle); ok {
			lc.OnCreate(ctx)
		}
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
	default:
		return "App"
	}
}

// ShouldClose returns true when the app (or its context) has requested closure.
// It checks ctx.Finish() first, then falls back to an optional ShouldClose()
// method on the content for backward compatibility.
func (a *App) ShouldClose() bool {
	if a.ctx.shouldClose {
		return true
	}
	type shouldCloser interface{ ShouldClose() bool }
	if sc, ok := a.content.(shouldCloser); ok {
		return sc.ShouldClose()
	}
	return false
}

func (a *App) Update(cursor draws.XY) {
	if a.content != nil {
		a.content.Update(cursor)
	}
}

func (a *App) Draw(dst draws.Image) {
	if a.content != nil {
		a.content.Draw(dst)
		return
	}
	dst.Fill(a.Color)
	a.title.Draw(dst)
	a.subtitle.Draw(dst)
}
