package apps

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui"
)

const (
	galleryPad     = 18.0
	galleryGap     = 12.0
	galleryTop     = 64.0
	galleryColumns = 2
)

// GalleryApp shows in-memory screenshots as a 2-column thumbnail grid.
// It reads screenshots from the OS context each frame so the view is always
// up-to-date without needing a separate Prepare call.
type GalleryApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64
	scroll  ui.ScrollBox

	title draws.Text
	empty draws.Text

	thumbs       []draws.Sprite
	thumbCount   int
	thumbScreenW float64
	thumbScreenH float64
}

func newGalleryApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	screenW, screenH := sz.X, sz.Y

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

	g := &GalleryApp{
		ctx:     ctx,
		screenW: screenW,
		screenH: screenH,
		title:   title,
		empty:   empty,
	}
	g.scroll.Size = draws.XY{X: screenW, Y: screenH - galleryTop}
	g.scroll.Locate(0, galleryTop, draws.LeftTop)
	return g
}

func (g *GalleryApp) Update(frame mosapp.Frame) {
	g.layoutScroll(len(g.ctx.Screenshots()))
	g.scroll.Update(frame)
}

func (g *GalleryApp) Draw(dst draws.Image) {
	shots := g.ctx.Screenshots()
	dst.Fill(color.RGBA{18, 19, 24, 255})

	g.title.Text = fmt.Sprintf("Gallery  %d", len(shots))
	g.title.Draw(dst)

	if len(shots) == 0 {
		g.empty.Draw(dst)
		return
	}
	if g.needsRebuild(shots) {
		g.rebuild(shots)
	}
	g.layoutScroll(len(shots))
	off := g.scroll.Offset()
	thumbH := g.thumbH()
	rows := (len(g.thumbs) + galleryColumns - 1) / galleryColumns
	startRow, endRow := ui.VisibleRange(off.Y, g.screenH-galleryTop, thumbH+galleryGap, rows, 1)
	for row := startRow; row < endRow; row++ {
		for col := 0; col < galleryColumns; col++ {
			idx := row*galleryColumns + col
			if idx >= len(g.thumbs) {
				break
			}
			thumb := g.thumbs[idx]
			thumb.Position.Y -= off.Y
			thumb.Draw(dst)
		}
	}
}

func (g *GalleryApp) needsRebuild(shots []draws.Image) bool {
	return g.thumbCount != len(shots) ||
		g.thumbScreenW != g.screenW ||
		g.thumbScreenH != g.screenH
}

func (g *GalleryApp) thumbW() float64 {
	return (g.screenW - galleryPad*2 - galleryGap) / galleryColumns
}

func (g *GalleryApp) thumbH() float64 {
	return g.thumbW() * 0.72
}

func (g *GalleryApp) layoutScroll(count int) {
	thumbH := g.thumbH()
	rows := (count + galleryColumns - 1) / galleryColumns
	g.scroll.ContentSize = draws.XY{
		X: g.screenW,
		Y: max(0, float64(rows)*(thumbH+galleryGap)-galleryGap),
	}
}

func (g *GalleryApp) rebuild(shots []draws.Image) {
	thumbW := g.thumbW()
	thumbH := g.thumbH()

	g.thumbs = g.thumbs[:0]
	for i := len(shots) - 1; i >= 0; i-- {
		visIdx := len(shots) - 1 - i
		col := visIdx % galleryColumns
		row := visIdx / galleryColumns
		x := galleryPad + float64(col)*(thumbW+galleryGap)
		y := galleryTop + float64(row)*(thumbH+galleryGap)

		frame := draws.CreateImage(thumbW, thumbH)
		frame.Fill(color.RGBA{235, 235, 240, 255})
		bg := draws.CreateImage(thumbW-4, thumbH-4)
		bg.Fill(color.RGBA{7, 7, 10, 255})
		bgOp := &ebiten.DrawImageOptions{}
		bgOp.GeoM.Translate(2, 2)
		frame.DrawImage(bg.Image, bgOp)

		shot := draws.NewSprite(shots[i])
		shotSize := shots[i].Size()
		scale := min((thumbW-8)/shotSize.X, (thumbH-8)/shotSize.Y)
		shot.Size = shotSize.Scale(scale)
		shot.Locate(thumbW/2, thumbH/2, draws.CenterMiddle)
		shot.Draw(frame)

		sp := draws.NewSprite(frame)
		sp.Locate(x, y, draws.LeftTop)
		g.thumbs = append(g.thumbs, sp)
	}
	g.thumbCount = len(shots)
	g.thumbScreenW = g.screenW
	g.thumbScreenH = g.screenH
}
