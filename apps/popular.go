package apps

import (
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2/vector"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui"
)

type ClipsApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64

	index     int
	liked     []bool
	saved     []bool
	captions  []string
	palette   [][]color.RGBA
	nextBtn   ui.Button
	likeBtn   ui.Button
	shareBtn  ui.Button
	recordBtn ui.Button
	saveBtn   ui.Button
}

func NewClipsApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	a := &ClipsApp{
		ctx:     ctx,
		screenW: sz.X,
		screenH: sz.Y,
		captions: []string{
			"One-thumb video feed rebuilt on MOS: wake timers, haptics, camera, share, and notices.",
			"Short clips can run without a backend when the OS API gives apps the right primitives.",
			"Swipe culture, simulator edition. Tap Next and the app schedules its own refresh.",
		},
		palette: [][]color.RGBA{
			{{255, 45, 85, 255}, {255, 149, 0, 255}, {35, 39, 50, 255}},
			{{88, 86, 214, 255}, {90, 200, 250, 255}, {20, 24, 32, 255}},
			{{52, 199, 89, 255}, {10, 132, 255, 255}, {28, 31, 42, 255}},
		},
	}
	a.liked = make([]bool, len(a.captions))
	a.saved = make([]bool, len(a.captions))
	a.layout()
	return a
}

func (a *ClipsApp) OnCreate(ctx mosapp.Context) {
	a.ctx = ctx
	ctx.SetTitle("Clips")
	ctx.SetAccentColor(color.RGBA{255, 45, 85, 255})
	ctx.SetSystemBarsHidden(true)
	ctx.SetKeepScreenOn(true)
}

func (a *ClipsApp) OnResume() { a.ctx.WakeAt(a.ctx.Now().Add(3 * time.Second)) }
func (a *ClipsApp) OnPause()  { a.ctx.SetKeepScreenOn(false) }
func (a *ClipsApp) OnDestroy() {
	a.ctx.SetSystemBarsHidden(false)
	a.ctx.SetKeepScreenOn(false)
}

func (a *ClipsApp) layout() {
	sa := a.ctx.SafeArea()
	y := a.screenH - max(200, sa.Bottom+190)
	a.nextBtn = ui.NewButton("Next", 13, a.screenW-92, y, 72, 34, color.RGBA{255, 255, 255, 45})
	a.likeBtn = ui.NewButton("Like", 13, a.screenW-92, y+44, 72, 34, color.RGBA{255, 45, 85, 255})
	a.saveBtn = ui.NewButton("Save", 13, a.screenW-92, y+88, 72, 34, color.RGBA{70, 76, 90, 255})
	a.shareBtn = ui.NewButton("Share", 13, a.screenW-92, y+132, 72, 34, color.RGBA{10, 132, 255, 255})
	a.recordBtn = ui.NewButton("Record", 13, 20, y+132, 92, 34, color.RGBA{255, 45, 85, 255})
}

func (a *ClipsApp) Update(frame mosapp.Frame) {
	a.layout()
	if a.nextBtn.Update(frame) {
		a.next()
	}
	if a.likeBtn.Update(frame) {
		a.liked[a.index] = !a.liked[a.index]
		a.ctx.Vibrate(20 * time.Millisecond)
	}
	if a.saveBtn.Update(frame) {
		a.saved[a.index] = !a.saved[a.index]
		a.ctx.SetPreference("saved", a.bits(a.saved))
		a.ctx.ShowToast("Saved clip")
	}
	if a.shareBtn.Update(frame) {
		a.ctx.ShareText(a.captions[a.index])
	}
	if a.recordBtn.Update(frame) {
		if _, ok := a.ctx.CapturePhoto(); ok {
			a.ctx.PostNotice(mosapp.Notice{Title: "Clips", Body: "Captured a new clip cover"})
		}
	}
	if a.ctx.Now().Second()%3 == 0 {
		a.ctx.WakeAt(a.ctx.Now().Add(1 * time.Second))
	}
}

func (a *ClipsApp) next() {
	a.index = (a.index + 1) % len(a.captions)
	a.ctx.WakeAt(a.ctx.Now().Add(3 * time.Second))
}

func (a *ClipsApp) bits(values []bool) string {
	var b strings.Builder
	for _, v := range values {
		if v {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

func (a *ClipsApp) Draw(dst draws.Image) {
	a.layout()
	a.paintVideo(dst)
	title := flowText("Clips", 22, color.RGBA{255, 255, 255, 245})
	title.Locate(20, a.ctx.SafeArea().Top+24, draws.LeftMiddle)
	title.Draw(dst)

	y := a.screenH - max(120, a.ctx.SafeArea().Bottom+110)
	for i, line := range wrapText(a.captions[a.index], 28) {
		t := flowText(line, 15, color.RGBA{255, 255, 255, 235})
		t.Locate(20, y+float64(i)*20, draws.LeftTop)
		t.Draw(dst)
	}
	if a.liked[a.index] {
		a.drawPill(dst, "Liked", a.screenW-98, y-50, color.RGBA{255, 45, 85, 230})
	}
	a.nextBtn.Draw(dst)
	a.likeBtn.Draw(dst)
	a.saveBtn.Draw(dst)
	a.shareBtn.Draw(dst)
	a.recordBtn.Draw(dst)
}

func (a *ClipsApp) paintVideo(dst draws.Image) {
	dst.Fill(color.RGBA{8, 9, 13, 255})
	cols := a.palette[a.index]
	for i, c := range cols {
		vector.DrawFilledRect(dst.Image, float32(i)*float32(a.screenW)/3, 0, float32(a.screenW)/3+1, float32(a.screenH), c, true)
	}
	pulse := 0.18 + float32(a.ctx.Now().UnixMilli()%1200)/1200*0.12
	vector.DrawFilledCircle(dst.Image, float32(a.screenW)*0.58, float32(a.screenH)*0.38, float32(a.screenW)*pulse, color.RGBA{255, 255, 255, 34}, true)
	vector.DrawFilledCircle(dst.Image, float32(a.screenW)*0.30, float32(a.screenH)*0.70, float32(a.screenW)*0.16, color.RGBA{0, 0, 0, 58}, true)
}

func (a *ClipsApp) drawPill(dst draws.Image, label string, x, y float64, c color.RGBA) {
	img := roundedRectImage(78, 28, 14, c)
	sp := draws.NewSprite(img)
	sp.Locate(x, y, draws.LeftTop)
	sp.Draw(dst)
	t := flowText(label, 12, color.RGBA{255, 255, 255, 255})
	t.Locate(x+39, y+14, draws.CenterMiddle)
	t.Draw(dst)
}

type RideApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64

	destination ui.TextField
	requestBtn  ui.Button
	shareBtn    ui.Button
	cancelBtn   ui.Button
	stage       int
	status      string
}

func NewRideApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	a := &RideApp{ctx: ctx, screenW: sz.X, screenH: sz.Y, status: "Choose destination"}
	a.layout()
	return a
}

func (a *RideApp) OnCreate(ctx mosapp.Context) {
	a.ctx = ctx
	ctx.SetTitle("Ride")
	ctx.SetAccentColor(color.RGBA{0, 160, 110, 255})
	_ = ctx.RequestPermission(mosapp.PermissionLocation)
}
func (a *RideApp) OnResume()  {}
func (a *RideApp) OnPause()   { a.ctx.HideKeyboard() }
func (a *RideApp) OnDestroy() {}

func (a *RideApp) layout() {
	sa := a.ctx.SafeArea()
	panelY := a.screenH - max(188, sa.Bottom+178)
	a.destination = ui.NewTextField(18, panelY+48, a.screenW-36, "Where to?")
	a.requestBtn = ui.NewButton("Request", 14, 18, panelY+104, a.screenW-36, 38, color.RGBA{0, 160, 110, 255})
	a.shareBtn = ui.NewButton("Share ETA", 13, 18, panelY+150, (a.screenW-48)/2, 34, color.RGBA{10, 132, 255, 255})
	a.cancelBtn = ui.NewButton("Cancel", 13, 30+(a.screenW-48)/2, panelY+150, (a.screenW-48)/2, 34, color.RGBA{72, 76, 86, 255})
}

func (a *RideApp) Update(frame mosapp.Frame) {
	a.layout()
	if a.destination.Update(frame) {
		if a.destination.IsFocused() {
			a.ctx.ShowKeyboard()
		}
	}
	a.destination.PollKeyboard()
	if a.requestBtn.Update(frame) {
		a.stage = 1
		a.status = "Driver arriving in 4 min"
		a.ctx.SetBadge(1)
		a.ctx.PostNotice(mosapp.Notice{Title: "Ride", Body: a.status})
	}
	if a.shareBtn.Update(frame) {
		a.ctx.ShareText("My MOS ride ETA is 4 minutes")
	}
	if a.cancelBtn.Update(frame) {
		a.stage = 0
		a.status = "Ride cancelled"
		a.ctx.ClearBadge()
	}
}

func (a *RideApp) Draw(dst draws.Image) {
	a.layout()
	dst.Fill(color.RGBA{216, 224, 218, 255})
	a.drawMap(dst)
	panelY := a.screenH - max(198, a.ctx.SafeArea().Bottom+188)
	panel := roundedRectImage(a.screenW-24, 180, 16, color.RGBA{20, 24, 29, 245})
	sp := draws.NewSprite(panel)
	sp.Locate(12, panelY, draws.LeftTop)
	sp.Draw(dst)
	title := flowText("Ride", 22, color.RGBA{255, 255, 255, 255})
	title.Locate(26, panelY+23, draws.LeftMiddle)
	title.Draw(dst)
	st := flowText(a.status, 13, flowTextSoft)
	st.Locate(86, panelY+24, draws.LeftMiddle)
	st.Draw(dst)
	a.destination.Draw(dst)
	a.requestBtn.Draw(dst)
	a.shareBtn.Draw(dst)
	a.cancelBtn.Draw(dst)
}

func (a *RideApp) drawMap(dst draws.Image) {
	for i := 0; i < 8; i++ {
		x := float32(i) * float32(a.screenW) / 7
		vector.StrokeLine(dst.Image, x, 0, x-90, float32(a.screenH), 3, color.RGBA{180, 194, 184, 255}, true)
	}
	for i := 0; i < 7; i++ {
		y := float32(i) * float32(a.screenH) / 6
		vector.StrokeLine(dst.Image, 0, y, float32(a.screenW), y+40, 3, color.RGBA{190, 202, 192, 255}, true)
	}
	vector.DrawFilledCircle(dst.Image, float32(a.screenW)*0.32, float32(a.screenH)*0.42, 9, color.RGBA{0, 122, 255, 255}, true)
	vector.DrawFilledCircle(dst.Image, float32(a.screenW)*0.68, float32(a.screenH)*0.34, 10, color.RGBA{0, 160, 110, 255}, true)
	if a.stage == 1 {
		vector.StrokeLine(dst.Image, float32(a.screenW)*0.32, float32(a.screenH)*0.42, float32(a.screenW)*0.68, float32(a.screenH)*0.34, 5, color.RGBA{0, 160, 110, 255}, true)
	}
}

type MarketApp struct {
	ctx      mosapp.Context
	screenW  float64
	screenH  float64
	cart     int
	scroll   ui.ScrollBox
	addBtns  []ui.TriggerButton
	checkout ui.Button
	search   ui.TextField
}

func NewMarketApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	a := &MarketApp{ctx: ctx, screenW: sz.X, screenH: sz.Y}
	if v, ok := ctx.Preference("cart"); ok {
		a.cart = parseSmallInt(v)
	}
	a.layout()
	return a
}

func (a *MarketApp) OnCreate(ctx mosapp.Context) {
	a.ctx = ctx
	ctx.SetTitle("Market")
	ctx.SetAccentColor(color.RGBA{255, 149, 0, 255})
}
func (a *MarketApp) OnResume()  { a.ctx.SetBadge(a.cart) }
func (a *MarketApp) OnPause()   { a.persist() }
func (a *MarketApp) OnDestroy() { a.persist() }

func (a *MarketApp) layout() {
	sa := a.ctx.SafeArea()
	top := max(118, sa.Top+112)
	bottom := max(72, sa.Bottom+64)
	a.search = ui.NewTextField(16, sa.Top+58, a.screenW-32, "Search market")
	a.checkout = ui.NewButton("Checkout", 14, 16, a.screenH-bottom+12, a.screenW-32, 38, color.RGBA{255, 149, 0, 255})
	a.scroll.Size = draws.XY{X: a.screenW, Y: a.screenH - top - bottom}
	a.scroll.Locate(0, top, draws.LeftTop)
	a.scroll.ContentSize = draws.XY{X: a.screenW, Y: 4*122 + 26}
	a.addBtns = a.addBtns[:0]
	for i := 0; i < 4; i++ {
		y := a.scroll.Position.Y + 18 + float64(i)*122 - a.scroll.Offset().Y
		a.addBtns = append(a.addBtns, ui.NewTriggerButton(a.screenW-94, y+62, 72, 36))
	}
}

func (a *MarketApp) Update(frame mosapp.Frame) {
	a.layout()
	a.scroll.Update(frame)
	if a.search.Update(frame) && a.search.IsFocused() {
		a.ctx.ShowKeyboard()
	}
	a.search.PollKeyboard()
	for i := range a.addBtns {
		if a.addBtns[i].Update(frame) {
			a.cart++
			a.ctx.SetBadge(a.cart)
			a.ctx.Vibrate(10 * time.Millisecond)
			a.ctx.ShowToast("Added to cart")
		}
	}
	if a.checkout.Update(frame) {
		path, _ := a.ctx.SaveFile("receipt.txt", []byte("MOS Market receipt: "+itoa(a.cart)+" items"))
		a.ctx.PostNotice(mosapp.Notice{Title: "Market", Body: "Receipt saved: " + path})
		a.cart = 0
		a.ctx.ClearBadge()
	}
}

func (a *MarketApp) persist() {
	a.ctx.SetPreference("cart", itoa(a.cart))
}

func (a *MarketApp) Draw(dst draws.Image) {
	a.layout()
	dst.Fill(color.RGBA{245, 246, 248, 255})
	title := flowText("Market", 24, color.RGBA{20, 22, 26, 255})
	title.Locate(16, a.ctx.SafeArea().Top+30, draws.LeftMiddle)
	title.Draw(dst)
	cart := flowText("Cart "+itoa(a.cart), 13, color.RGBA{70, 75, 85, 255})
	cart.Locate(a.screenW-16, a.ctx.SafeArea().Top+31, draws.RightMiddle)
	cart.Draw(dst)
	a.search.Draw(dst)
	a.drawProducts(dst)
	a.checkout.Draw(dst)
}

func (a *MarketApp) drawProducts(dst draws.Image) {
	names := []string{"Everyday Pack", "Creator Light", "Pocket Stand", "Travel Cable"}
	prices := []string{"$42", "$89", "$18", "$12"}
	for i, name := range names {
		y := a.scroll.Position.Y + 18 + float64(i)*122 - a.scroll.Offset().Y
		if y < a.scroll.Position.Y-120 || y > a.scroll.Position.Y+a.scroll.H() {
			continue
		}
		card := roundedRectImage(a.screenW-32, 104, 12, color.RGBA{255, 255, 255, 255})
		sp := draws.NewSprite(card)
		sp.Locate(16, y, draws.LeftTop)
		sp.Draw(dst)
		sw := color.RGBA{uint8(80 + i*34), uint8(130 + i*18), uint8(190 - i*22), 255}
		vector.DrawFilledRect(dst.Image, 30, float32(y+18), 62, 62, sw, true)
		t := flowText(name, 16, color.RGBA{20, 22, 26, 255})
		t.Locate(108, y+22, draws.LeftTop)
		t.Draw(dst)
		p := flowText(prices[i], 14, color.RGBA{90, 96, 106, 255})
		p.Locate(108, y+52, draws.LeftTop)
		p.Draw(dst)
		a.drawAddText(dst, a.screenW-58, y+80)
	}
}

func (a *MarketApp) drawAddText(dst draws.Image, cx, cy float64) {
	t := flowText("Add", 13, color.RGBA{255, 149, 0, 255})
	t.Locate(cx, cy, draws.CenterMiddle)
	t.Draw(dst)
}

func parseSmallInt(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
