package apps

import (
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/ui"
)

const (
	flowTopH    = 56.0
	flowTabH    = 58.0
	flowPad     = 14.0
	flowCardGap = 12.0
)

var (
	flowBg       = color.RGBA{12, 14, 18, 255}
	flowPanel    = color.RGBA{22, 25, 31, 255}
	flowPanel2   = color.RGBA{29, 33, 41, 255}
	flowTextSoft = color.RGBA{255, 255, 255, 165}
	flowLine     = color.RGBA{255, 255, 255, 24}
	flowAccent   = color.RGBA{255, 72, 112, 255}
	flowBlue     = color.RGBA{48, 146, 255, 255}
)

type flowTab int

const (
	flowFeed flowTab = iota
	flowSearch
	flowInbox
	flowProfile
)

type flowPost struct {
	Author   string
	Handle   string
	Caption  string
	Palette  []color.RGBA
	Likes    int
	Comments int
	Location string
	Liked    bool
	Saved    bool
	Mine     bool
}

// FlowApp is a compact rewrite of a mainstream social feed app using only the
// MOS app API: lifecycle, preferences, notices, share/open intents,
// permissions, photo picker, safe areas, scroll, badges, and haptics.
type FlowApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64

	tab            flowTab
	posts          []flowPost
	scroll         ui.ScrollBox
	tabButtons     []ui.TriggerButton
	likeButtons    []ui.TriggerButton
	saveButtons    []ui.TriggerButton
	shareButtons   []ui.TriggerButton
	commentButtons []ui.TriggerButton
	composeBtn     ui.Button
	cameraBtn      ui.Button
	photoBtn       ui.Button
	clearBtn       ui.Button
	query          ui.TextField
	status         string
	createdAt      time.Time
	layoutStamp    string
}

func NewFlowApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	a := &FlowApp{
		ctx:       ctx,
		screenW:   sz.X,
		screenH:   sz.Y,
		createdAt: ctx.Now(),
		posts: []flowPost{
			{
				Author: "Mira Studio", Handle: "@mira", Location: "Seoul",
				Caption: "A pocket OS should make the happy path feel tiny: draw, update, ask the system.",
				Palette: []color.RGBA{{255, 107, 107, 255}, {255, 211, 105, 255}, {75, 192, 192, 255}},
				Likes:   1280, Comments: 92,
			},
			{
				Author: "Northline", Handle: "@northline", Location: "Fold Lab",
				Caption: "Same code, bar phone to foldable. Safe areas and window commands do the boring work.",
				Palette: []color.RGBA{{48, 146, 255, 255}, {120, 86, 255, 255}, {25, 221, 160, 255}},
				Likes:   842, Comments: 47,
			},
			{
				Author: "Ari", Handle: "@ari", Location: "Simulator",
				Caption: "Posting from a local-first social app rewrite. No backend yet, but the surface area is here.",
				Palette: []color.RGBA{{255, 72, 112, 255}, {255, 149, 0, 255}, {44, 49, 61, 255}},
				Likes:   403, Comments: 18,
			},
		},
		status: "Ready",
	}
	a.layout()
	return a
}

func (a *FlowApp) OnCreate(ctx mosapp.Context) {
	a.ctx = ctx
	a.restoreState()
	ctx.SetTitle("Flow")
	ctx.SetAccentColor(flowAccent)
	ctx.SetStatusBarStyle(mosapp.BarStyleLight)
	ctx.SetNavigationBarStyle(mosapp.BarStyleDark)
	ctx.SetBackHandler(func() bool {
		if a.tab != flowFeed {
			a.tab = flowFeed
			a.status = "Feed"
			a.ctx.Invalidate()
			return true
		}
		return false
	})
}

func (a *FlowApp) OnResume() {
	a.ctx.SetBadge(a.unreadCount())
	a.ctx.ScheduleNotice("flow-break", mosapp.Notice{
		Title: "Flow",
		Body:  "New posts are waiting in your feed.",
	}, a.ctx.Now().Add(10*time.Second))
}

func (a *FlowApp) OnPause() {
	a.persistState()
}

func (a *FlowApp) OnDestroy() {
	a.ctx.CancelNotice("flow-break")
	a.ctx.ClearBackHandler()
	a.persistState()
}

func (a *FlowApp) layout() {
	sa := a.ctx.SafeArea()
	stamp := strings.Join([]string{ftoa(a.screenW), ftoa(a.screenH), ftoa(sa.Top), ftoa(sa.Bottom)}, ":")
	if stamp == a.layoutStamp {
		return
	}
	a.layoutStamp = stamp

	top := max(flowTopH, sa.Top+flowTopH)
	bottom := max(flowTabH, sa.Bottom+flowTabH)
	a.scroll.Size = draws.XY{X: a.screenW, Y: max(0, a.screenH-top-bottom)}
	a.scroll.Locate(0, top, draws.LeftTop)

	a.tabButtons = make([]ui.TriggerButton, 4)
	tabW := a.screenW / 4
	for i := range a.tabButtons {
		a.tabButtons[i] = ui.NewTriggerButton(float64(i)*tabW, a.screenH-bottom, tabW, flowTabH)
	}

	a.query = ui.NewTextField(flowPad, top+12, a.screenW-flowPad*2, "Search Flow")
	a.composeBtn = ui.NewButton("Post", 14, a.screenW-flowPad-72, sa.Top+12, 72, 32, flowAccent)
	a.cameraBtn = ui.NewButton("Camera", 13, flowPad, top+64, 86, 34, flowAccent)
	a.photoBtn = ui.NewButton("Photo", 13, flowPad+98, top+64, 86, 34, flowBlue)
	a.clearBtn = ui.NewButton("Clear", 13, a.screenW-flowPad-86, top+64, 86, 34, color.RGBA{66, 72, 84, 255})
	a.layoutFeedButtons()
}

func (a *FlowApp) layoutFeedButtons() {
	a.likeButtons = a.likeButtons[:0]
	a.saveButtons = a.saveButtons[:0]
	a.shareButtons = a.shareButtons[:0]
	a.commentButtons = a.commentButtons[:0]

	y := flowPad
	cardW := a.screenW - flowPad*2
	for i := range a.posts {
		h := a.postHeight(a.posts[i])
		actionY := a.scroll.Position.Y + y + h - 48 - a.scroll.Offset().Y
		a.likeButtons = append(a.likeButtons, ui.NewTriggerButton(flowPad+12, actionY, 70, 36))
		a.commentButtons = append(a.commentButtons, ui.NewTriggerButton(flowPad+92, actionY, 80, 36))
		a.shareButtons = append(a.shareButtons, ui.NewTriggerButton(flowPad+cardW-132, actionY, 58, 36))
		a.saveButtons = append(a.saveButtons, ui.NewTriggerButton(flowPad+cardW-68, actionY, 56, 36))
		y += h + flowCardGap
	}
	a.scroll.ContentSize = draws.XY{X: a.screenW, Y: y + flowPad}
}

func (a *FlowApp) postHeight(p flowPost) float64 {
	return 318 + float64(len(wrapText(p.Caption, 34)))*17
}

func (a *FlowApp) Update(frame mosapp.Frame) {
	a.layout()
	for i := range a.tabButtons {
		if a.tabButtons[i].Update(frame) {
			a.tab = flowTab(i)
			a.status = a.tabName(a.tab)
			a.ctx.Vibrate(12 * time.Millisecond)
			a.ctx.HideKeyboard()
		}
	}

	switch a.tab {
	case flowFeed:
		a.updateFeed(frame)
	case flowSearch:
		a.updateSearch(frame)
	case flowInbox:
		a.updateInbox(frame)
	case flowProfile:
		a.updateProfile(frame)
	}
}

func (a *FlowApp) updateFeed(frame mosapp.Frame) {
	a.scroll.Update(frame)
	a.layoutFeedButtons()
	for i := range a.posts {
		if a.likeButtons[i].Update(frame) {
			a.toggleLike(i)
		}
		if a.commentButtons[i].Update(frame) {
			a.posts[i].Comments++
			a.ctx.ShowToast("Comment added")
			a.ctx.Announce("Comment added")
		}
		if a.shareButtons[i].Update(frame) {
			a.ctx.ShareText(a.posts[i].Caption)
			a.status = "Shared post"
		}
		if a.saveButtons[i].Update(frame) {
			a.posts[i].Saved = !a.posts[i].Saved
			a.ctx.CopyText(a.posts[i].Caption)
			a.status = "Saved to collection"
		}
	}
	if a.composeBtn.Update(frame) {
		a.addDraftPost(false)
	}
}

func (a *FlowApp) updateSearch(frame mosapp.Frame) {
	if a.query.Update(frame) {
		if a.query.IsFocused() {
			a.ctx.RequestFocus()
			a.ctx.ShowKeyboard()
		}
	}
	a.query.PollKeyboard()
	if a.cameraBtn.Update(frame) {
		if _, ok := a.ctx.CapturePhoto(); ok {
			a.addDraftPost(true)
			a.status = "Captured photo"
			return
		}
		a.status = "Camera unavailable"
	}
	if a.photoBtn.Update(frame) {
		if a.ctx.RequestPermission(mosapp.PermissionPhotos) != mosapp.PermissionGranted {
			a.status = "Photos permission denied"
			return
		}
		if _, ok := a.ctx.PickPhoto(); ok {
			a.addDraftPost(true)
			return
		}
		a.status = "No photo picked"
		a.ctx.ShowToast("Take a screenshot first, then pick photo")
	}
}

func (a *FlowApp) updateInbox(frame mosapp.Frame) {
	if a.clearBtn.Update(frame) {
		a.ctx.ClearBadge()
		a.status = "Inbox cleared"
	}
}

func (a *FlowApp) updateProfile(frame mosapp.Frame) {
	if a.clearBtn.Update(frame) {
		a.ctx.OpenURL("https://example.com/flow/profile")
		a.status = "Opened profile link"
	}
}

func (a *FlowApp) toggleLike(i int) {
	if i < 0 || i >= len(a.posts) {
		return
	}
	if a.posts[i].Liked {
		a.posts[i].Liked = false
		a.posts[i].Likes--
		a.status = "Like removed"
		return
	}
	a.posts[i].Liked = true
	a.posts[i].Likes++
	a.ctx.Vibrate(18 * time.Millisecond)
	a.ctx.PostNotice(mosapp.Notice{Title: "Flow", Body: "Liked " + a.posts[i].Author + "'s post"})
	a.status = "Liked"
}

func (a *FlowApp) addDraftPost(fromPhoto bool) {
	caption := strings.TrimSpace(a.query.Value)
	if caption == "" {
		caption = "Built with MOS app APIs: storage, notifications, sharing, permissions, and safe areas."
	}
	if fromPhoto {
		caption = "Photo post: " + caption
	}
	post := flowPost{
		Author: "You", Handle: "@you", Location: "MOS",
		Caption: caption, Mine: true, Likes: 1, Comments: 0, Liked: true,
		Palette: []color.RGBA{{34, 197, 94, 255}, {48, 146, 255, 255}, {255, 72, 112, 255}},
	}
	a.posts = append([]flowPost{post}, a.posts...)
	a.query.Clear()
	a.scroll.ScrollBy(draws.XY{Y: -a.scroll.Offset().Y})
	a.status = "Posted"
	a.ctx.PostNotice(mosapp.Notice{Title: "Flow", Body: "Your post is live"})
	a.persistState()
}

func (a *FlowApp) persistState() {
	a.ctx.SetPreference("likes", a.stateBits(func(p flowPost) bool { return p.Liked }))
	a.ctx.SetPreference("saved", a.stateBits(func(p flowPost) bool { return p.Saved }))
}

func (a *FlowApp) restoreState() {
	a.applyBits("likes", func(i int, on bool) {
		if a.posts[i].Liked == on {
			return
		}
		a.posts[i].Liked = on
		if on {
			a.posts[i].Likes++
		} else {
			a.posts[i].Likes--
		}
	})
	a.applyBits("saved", func(i int, on bool) { a.posts[i].Saved = on })
}

func (a *FlowApp) stateBits(fn func(flowPost) bool) string {
	var b strings.Builder
	for _, p := range a.posts {
		if fn(p) {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

func (a *FlowApp) applyBits(key string, set func(i int, on bool)) {
	bits, ok := a.ctx.Preference(key)
	if !ok {
		return
	}
	for i := 0; i < len(a.posts) && i < len(bits); i++ {
		set(i, bits[i] == '1')
	}
}

func (a *FlowApp) unreadCount() int { return 3 }

func (a *FlowApp) tabName(tab flowTab) string {
	switch tab {
	case flowSearch:
		return "Search"
	case flowInbox:
		return "Inbox"
	case flowProfile:
		return "Profile"
	default:
		return "Feed"
	}
}

func (a *FlowApp) Draw(dst draws.Image) {
	a.layout()
	dst.Fill(flowBg)
	a.drawTopBar(dst)
	switch a.tab {
	case flowFeed:
		a.drawFeed(dst)
	case flowSearch:
		a.drawSearch(dst)
	case flowInbox:
		a.drawInbox(dst)
	case flowProfile:
		a.drawProfile(dst)
	}
	a.drawTabs(dst)
}

func (a *FlowApp) drawTopBar(dst draws.Image) {
	sa := a.ctx.SafeArea()
	bar := draws.CreateImage(a.screenW, max(flowTopH, sa.Top+flowTopH))
	bar.Fill(color.RGBA{13, 15, 20, 245})
	sp := draws.NewSprite(bar)
	sp.Locate(0, 0, draws.LeftTop)
	sp.Draw(dst)

	title := flowText("Flow", 24, color.RGBA{255, 255, 255, 255})
	title.Locate(flowPad, sa.Top+28, draws.LeftMiddle)
	title.Draw(dst)

	meta := flowText(a.status, 12, flowTextSoft)
	meta.Locate(flowPad+64, sa.Top+29, draws.LeftMiddle)
	meta.Draw(dst)
	if a.tab == flowFeed {
		a.composeBtn.Draw(dst)
	}
}

func (a *FlowApp) drawFeed(dst draws.Image) {
	canvasH := max(a.scroll.H(), a.scroll.ContentSize.Y)
	canvas := draws.CreateImage(a.screenW, canvasH)
	canvas.Fill(flowBg)
	y := flowPad - a.scroll.Offset().Y
	for _, post := range a.posts {
		h := a.postHeight(post)
		a.drawPost(canvas, post, flowPad, y, a.screenW-flowPad*2, h)
		y += h + flowCardGap
	}
	a.drawScrollCanvas(dst, canvas)
}

func (a *FlowApp) drawScrollCanvas(dst, canvas draws.Image) {
	clipH := int(min(a.scroll.H(), canvas.Size().Y))
	if clipH <= 0 {
		return
	}
	sub := canvas.Image.SubImage(image.Rect(0, 0, int(a.screenW), clipH)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, a.scroll.Position.Y)
	dst.DrawImage(sub, op)
}

func (a *FlowApp) drawPost(dst draws.Image, p flowPost, x, y, w, h float64) {
	card := roundedRectImage(w, h, 14, flowPanel)
	sp := draws.NewSprite(card)
	sp.Locate(x, y, draws.LeftTop)
	sp.Draw(dst)

	a.drawAvatar(dst, x+26, y+28, p.Author)
	author := flowText(p.Author, 15, color.RGBA{255, 255, 255, 245})
	author.Locate(x+54, y+19, draws.LeftTop)
	author.Draw(dst)
	handle := flowText(p.Handle+"  "+p.Location, 12, flowTextSoft)
	handle.Locate(x+54, y+40, draws.LeftTop)
	handle.Draw(dst)

	media := draws.CreateImage(w-24, 188)
	a.paintGradient(media, p.Palette)
	msp := draws.NewSprite(media)
	msp.Locate(x+12, y+66, draws.LeftTop)
	msp.Draw(dst)

	captionY := y + 266
	for i, line := range wrapText(p.Caption, 34) {
		t := flowText(line, 13, color.RGBA{255, 255, 255, 220})
		t.Locate(x+14, captionY+float64(i)*17, draws.LeftTop)
		t.Draw(dst)
	}

	actionY := y + h - 38
	a.drawAction(dst, x+14, actionY, heartLabel(p.Liked)+" "+itoa(p.Likes), p.Liked)
	a.drawAction(dst, x+94, actionY, "C "+itoa(p.Comments), false)
	a.drawAction(dst, x+w-128, actionY, "Share", false)
	a.drawAction(dst, x+w-64, actionY, saveLabel(p.Saved), p.Saved)
}

func (a *FlowApp) drawAvatar(dst draws.Image, cx, cy float64, name string) {
	img := roundedRectImage(34, 34, 17, flowAccent)
	sp := draws.NewSprite(img)
	sp.Locate(cx, cy, draws.CenterMiddle)
	sp.Draw(dst)
	txt := flowText(initials(name), 12, color.RGBA{255, 255, 255, 255})
	txt.Locate(cx, cy, draws.CenterMiddle)
	txt.Draw(dst)
}

func (a *FlowApp) paintGradient(dst draws.Image, cols []color.RGBA) {
	if len(cols) == 0 {
		dst.Fill(flowPanel2)
		return
	}
	size := dst.Size()
	for i, c := range cols {
		x := float32(size.X) * float32(i) / float32(len(cols))
		w := float32(size.X)/float32(len(cols)) + 1
		vector.DrawFilledRect(dst.Image, x, 0, w, float32(size.Y), c, true)
	}
	vector.DrawFilledCircle(dst.Image, float32(size.X)*0.72, float32(size.Y)*0.34, float32(size.X)*0.18, color.RGBA{255, 255, 255, 42}, true)
	vector.DrawFilledCircle(dst.Image, float32(size.X)*0.28, float32(size.Y)*0.68, float32(size.X)*0.20, color.RGBA{0, 0, 0, 45}, true)
}

func (a *FlowApp) drawSearch(dst draws.Image) {
	a.query.Draw(dst)
	a.cameraBtn.Draw(dst)
	a.photoBtn.Draw(dst)
	top := a.scroll.Position.Y + 112
	a.drawPanelText(dst, "Discover", "Search posts, capture a simulated camera photo, or import the latest screenshot as a post.", top)
}

func (a *FlowApp) drawInbox(dst draws.Image) {
	a.clearBtn.Draw(dst)
	top := a.scroll.Position.Y + 28
	a.drawInboxRow(dst, top, "Mira Studio", "Liked your MOS rewrite", "now")
	a.drawInboxRow(dst, top+64, "Northline", "Started following you", "2m")
	a.drawInboxRow(dst, top+128, "Ari", "Commented: this API is enough", "8m")
}

func (a *FlowApp) drawProfile(dst draws.Image) {
	a.clearBtn.Draw(dst)
	top := a.scroll.Position.Y + 30
	a.drawAvatar(dst, a.screenW/2, top+36, "You")
	name := flowText("You", 22, color.RGBA{255, 255, 255, 255})
	name.Locate(a.screenW/2, top+76, draws.CenterTop)
	name.Draw(dst)
	stats := flowText(itoa(len(a.posts))+" posts  "+itoa(a.savedCount())+" saved  online", 13, flowTextSoft)
	stats.Locate(a.screenW/2, top+110, draws.CenterTop)
	stats.Draw(dst)
	a.drawPanelText(dst, "Local account", "Profile state is stored through MOS preferences and system intents handle links.", top+160)
}

func (a *FlowApp) drawPanelText(dst draws.Image, title, body string, y float64) {
	panel := roundedRectImage(a.screenW-flowPad*2, 112, 12, flowPanel)
	sp := draws.NewSprite(panel)
	sp.Locate(flowPad, y, draws.LeftTop)
	sp.Draw(dst)
	t := flowText(title, 17, color.RGBA{255, 255, 255, 245})
	t.Locate(flowPad+16, y+18, draws.LeftTop)
	t.Draw(dst)
	for i, line := range wrapText(body, 38) {
		txt := flowText(line, 13, flowTextSoft)
		txt.Locate(flowPad+16, y+50+float64(i)*18, draws.LeftTop)
		txt.Draw(dst)
	}
}

func (a *FlowApp) drawInboxRow(dst draws.Image, y float64, title, body, when string) {
	a.drawAvatar(dst, flowPad+20, y+28, title)
	t := flowText(title, 15, color.RGBA{255, 255, 255, 240})
	t.Locate(flowPad+52, y+13, draws.LeftTop)
	t.Draw(dst)
	b := flowText(body, 12, flowTextSoft)
	b.Locate(flowPad+52, y+36, draws.LeftTop)
	b.Draw(dst)
	w := flowText(when, 12, flowTextSoft)
	w.Locate(a.screenW-flowPad, y+18, draws.RightTop)
	w.Draw(dst)
}

func (a *FlowApp) drawTabs(dst draws.Image) {
	sa := a.ctx.SafeArea()
	tabH := max(flowTabH, sa.Bottom+flowTabH)
	bg := draws.CreateImage(a.screenW, tabH)
	bg.Fill(color.RGBA{15, 17, 22, 248})
	sp := draws.NewSprite(bg)
	sp.Locate(0, a.screenH-tabH, draws.LeftTop)
	sp.Draw(dst)

	labels := []string{"Home", "Find", "Inbox", "You"}
	tabW := a.screenW / 4
	for i, label := range labels {
		c := flowTextSoft
		if flowTab(i) == a.tab {
			c = color.RGBA{255, 255, 255, 255}
			vector.DrawFilledRect(dst.Image, float32(i)*float32(tabW)+float32(tabW)*0.32, float32(a.screenH-tabH+6), float32(tabW)*0.36, 3, flowAccent, true)
		}
		t := flowText(label, 12, c)
		t.Locate(float64(i)*tabW+tabW/2, a.screenH-tabH+29, draws.CenterMiddle)
		t.Draw(dst)
	}
}

func (a *FlowApp) drawAction(dst draws.Image, x, y float64, label string, active bool) {
	c := color.RGBA{255, 255, 255, 210}
	if active {
		c = flowAccent
	}
	t := flowText(label, 12, c)
	t.Locate(x, y, draws.LeftMiddle)
	t.Draw(dst)
}

func (a *FlowApp) savedCount() int {
	n := 0
	for _, p := range a.posts {
		if p.Saved {
			n++
		}
	}
	return n
}

func flowText(s string, size float64, c color.RGBA) draws.Text {
	t := draws.NewText(s)
	opts := draws.NewFaceOptions()
	opts.Size = size
	t.SetFace(opts)
	r, g, b, a := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255
	t.ColorScale.Scale(r, g, b, a)
	return t
}

func heartLabel(on bool) string {
	if on {
		return "Love"
	}
	return "Like"
}

func saveLabel(on bool) string {
	if on {
		return "Saved"
	}
	return "Save"
}
