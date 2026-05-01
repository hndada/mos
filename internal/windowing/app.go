package windowing

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hndada/mos/apps"
	"github.com/hndada/mos/internal/draws"
)

const (
	AppIDColor     = "color"
	AppIDGallery   = "gallery"
	AppIDSettings  = "settings"
	AppIDCall      = "call"
	AppIDSceneTest = "scene-test"
)

// AppContent is implemented by each app to provide per-frame update and draw.
type AppContent interface {
	Update(cursor draws.XY)
	Draw(dst draws.Image)
}

type closeRequester interface {
	ShouldClose() bool
}

type App struct {
	ID       string
	Color    color.RGBA
	content  AppContent
	gallery  *GalleryApp
	title    draws.Text
	subtitle draws.Text
}

type AppState struct {
	ID    string
	Color color.RGBA
}

// NewApp creates an App and instantiates the appropriate AppContent.
func NewApp(id string, clr color.RGBA, screenW, screenH float64) *App {
	a := &App{ID: id, Color: clr}
	switch id {
	case AppIDGallery:
		a.gallery = NewGalleryApp(screenW, screenH)
	case AppIDSettings:
		a.content = apps.NewSettings(screenW, screenH)
	case AppIDCall:
		a.content = apps.NewCall(screenW, screenH)
	case AppIDSceneTest:
		a.content = apps.NewSceneTest(screenW, screenH)
	}
	if a.content == nil && a.gallery == nil {
		a.initPlaceholderText(screenW, screenH)
	}
	return a
}

func (a *App) initPlaceholderText(screenW, screenH float64) {
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 28
	a.title = draws.NewText(appTitle(a.ID))
	a.title.SetFace(titleOpts)
	a.title.Locate(screenW/2, screenH/2-12, draws.CenterMiddle)

	subtitleOpts := draws.NewFaceOptions()
	subtitleOpts.Size = 14
	a.subtitle = draws.NewText("Running")
	a.subtitle.SetFace(subtitleOpts)
	a.subtitle.Locate(screenW/2, screenH/2+24, draws.CenterMiddle)
}

func appTitle(id string) string {
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

func (a *App) Update(cursor draws.XY) {
	if a.content != nil {
		a.content.Update(cursor)
	}
}

func (a *App) ShouldClose() bool {
	if closer, ok := a.content.(closeRequester); ok {
		return closer.ShouldClose()
	}
	return false
}

func (a *App) Prepare(screenshots []draws.Image) {
	if a.gallery != nil {
		a.gallery.Prepare(screenshots)
	}
}

func (a *App) Draw(dst draws.Image, screenshots []draws.Image) {
	if a.content != nil {
		a.content.Draw(dst)
		return
	}
	if a.gallery != nil {
		a.gallery.Draw(dst, screenshots)
		return
	}
	dst.Fill(a.Color)
	a.title.Draw(dst)
	a.subtitle.Draw(dst)
}

type GalleryApp struct {
	screenW float64
	screenH float64

	title draws.Text
	empty draws.Text

	thumbs       []draws.Sprite
	thumbCount   int
	thumbScreenW float64
	thumbScreenH float64
}

func NewGalleryApp(screenW, screenH float64) *GalleryApp {
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 22
	title := draws.NewText("")
	title.SetFace(titleOpts)
	title.Locate(18, 18, draws.LeftTop)

	emptyOpts := draws.NewFaceOptions()
	emptyOpts.Size = 18
	empty := draws.NewText("No screenshots yet")
	empty.SetFace(emptyOpts)
	empty.Locate(screenW/2, screenH/2, draws.CenterMiddle)

	return &GalleryApp{
		screenW: screenW,
		screenH: screenH,
		title:   title,
		empty:   empty,
	}
}

func (g *GalleryApp) Draw(dst draws.Image, screenshots []draws.Image) {
	dst.Fill(color.RGBA{18, 19, 24, 255})

	g.title.Text = fmt.Sprintf("Gallery  %d", len(screenshots))
	g.title.Draw(dst)

	if len(screenshots) == 0 {
		g.empty.Draw(dst)
		return
	}

	for _, thumb := range g.thumbs {
		thumb.Draw(dst)
	}
}

func (g *GalleryApp) Prepare(screenshots []draws.Image) {
	if g.needsThumbRebuild(screenshots) {
		g.rebuildThumbs(screenshots)
	}
}

func (g *GalleryApp) needsThumbRebuild(screenshots []draws.Image) bool {
	return g.thumbCount != len(screenshots) ||
		g.thumbScreenW != g.screenW ||
		g.thumbScreenH != g.screenH
}

func (g *GalleryApp) rebuildThumbs(screenshots []draws.Image) {
	const (
		pad     = 18.0
		gap     = 12.0
		top     = 64.0
		columns = 2
	)
	thumbW := (g.screenW - pad*2 - gap) / columns
	thumbH := thumbW * 0.72

	g.thumbs = g.thumbs[:0]
	for i := len(screenshots) - 1; i >= 0; i-- {
		visibleIndex := len(screenshots) - 1 - i
		col := visibleIndex % columns
		row := visibleIndex / columns
		x := pad + float64(col)*(thumbW+gap)
		y := top + float64(row)*(thumbH+gap)
		if y >= g.screenH {
			break
		}

		frame := draws.CreateImage(thumbW, thumbH)
		frame.Fill(color.RGBA{235, 235, 240, 255})
		bg := draws.CreateImage(thumbW-4, thumbH-4)
		bg.Fill(color.RGBA{7, 7, 10, 255})
		bgOp := &ebiten.DrawImageOptions{}
		bgOp.GeoM.Translate(2, 2)
		frame.DrawImage(bg.Image, bgOp)

		shot := draws.NewSprite(screenshots[i])
		shotSize := screenshots[i].Size()
		scale := min((thumbW-8)/shotSize.X, (thumbH-8)/shotSize.Y)
		shot.Size = shotSize.Scale(scale)
		shot.Locate(thumbW/2, thumbH/2, draws.CenterMiddle)
		shot.Draw(frame)

		sp := draws.NewSprite(frame)
		sp.Locate(x, y, draws.LeftTop)
		g.thumbs = append(g.thumbs, sp)
	}
	g.thumbCount = len(screenshots)
	g.thumbScreenW = g.screenW
	g.thumbScreenH = g.screenH
}
