package apps

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
	"github.com/hndada/mos/internal/tween"
	"github.com/hndada/mos/ui"
)

const (
	messageStatusH   = 24.0
	messageBarH      = 52.0
	messageComposerH = 60.0
	messagePad       = 14.0
	messageGap       = 10.0
)

var (
	messageBg       = color.RGBA{13, 16, 22, 255}
	messageBarBg    = color.RGBA{20, 24, 32, 255}
	messageListBg   = color.RGBA{24, 29, 38, 255}
	messageMineBg   = color.RGBA{10, 132, 255, 255}
	messageTheirBg  = color.RGBA{44, 49, 61, 255}
	messageDivider  = color.RGBA{255, 255, 255, 24}
	messageCompose  = color.RGBA{19, 23, 31, 255}
	messageSendBg   = color.RGBA{10, 132, 255, 255}
	messageBackBg   = color.RGBA{38, 43, 54, 255}
	messageAvatarBg = color.RGBA{99, 91, 255, 255}
)

type messageView int

const (
	messageInbox messageView = iota
	messageThread
)

type messageBubble struct {
	Body string
	Mine bool
	At   time.Time
}

type messageThreadModel struct {
	Name     string
	Preview  string
	Unread   int
	Messages []messageBubble
}

type MessageApp struct {
	ctx     mosapp.Context
	screenW float64
	screenH float64
	view    messageView
	active  int

	createdAt   time.Time
	transition  tween.Tween
	moving      bool
	moveFrom    messageView
	moveTo      messageView
	moveForward bool

	threads []messageThreadModel
	tiles   []ui.ListTile

	title       draws.Text
	subtitle    draws.Text
	empty       draws.Text
	back        ui.Button
	send        ui.Button
	composer    ui.TextField
	msgScroll   ui.ScrollBox
	layoutStamp string

	unsubs []func()
}

func NewMessageApp(ctx mosapp.Context) mosapp.Content {
	sz := ctx.ScreenSize()
	m := &MessageApp{
		ctx:       ctx,
		screenW:   sz.X,
		screenH:   sz.Y,
		active:    -1,
		createdAt: time.Now(),
		threads: []messageThreadModel{
			{
				Name:    "Ari",
				Preview: "Dinner after the simulator run?",
				Unread:  2,
				Messages: []messageBubble{
					{Body: "Dinner after the simulator run?", At: time.Now().Add(-32 * time.Minute)},
					{Body: "I found a place with excellent noodles.", At: time.Now().Add(-31 * time.Minute)},
				},
			},
			{
				Name:    "Mina",
				Preview: "The new app API feels pretty nice.",
				Messages: []messageBubble{
					{Body: "The new app API feels pretty nice.", At: time.Now().Add(-2 * time.Hour)},
					{Body: "Agreed. Context, events, and widgets are enough for small apps.", Mine: true, At: time.Now().Add(-118 * time.Minute)},
				},
			},
			{
				Name:    "MOS Bot",
				Preview: "Tap a thread, type with your keyboard, then send.",
				Messages: []messageBubble{
					{Body: "Tap a thread, type with your keyboard, then send.", At: time.Now().Add(-4 * time.Hour)},
				},
			},
		},
	}
	m.initText()
	m.layout()
	return m
}

func (m *MessageApp) OnCreate(ctx mosapp.Context) {
	m.ctx = ctx
	if bus := ctx.Bus(); bus != nil {
		m.unsubs = append(m.unsubs, bus.Subscribe(event.KindCustom, func(e event.Event) {
			ce, ok := e.(event.Custom)
			if !ok || ce.Topic != "mos/message/incoming" {
				return
			}
			msg, ok := ce.Data.(messageIncoming)
			if !ok {
				return
			}
			m.receive(msg.From, msg.Body)
		}))
	}
}

func (m *MessageApp) OnResume() {}
func (m *MessageApp) OnPause()  { m.ctx.HideKeyboard() }
func (m *MessageApp) OnDestroy() {
	for _, unsub := range m.unsubs {
		unsub()
	}
	m.unsubs = nil
}

type messageIncoming struct {
	From string
	Body string
}

func (m *MessageApp) initText() {
	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 21
	m.title = draws.NewText("Messages")
	m.title.SetFace(titleOpts)

	subOpts := draws.NewFaceOptions()
	subOpts.Size = 13
	m.subtitle = draws.NewText("")
	m.subtitle.SetFace(subOpts)

	emptyOpts := draws.NewFaceOptions()
	emptyOpts.Size = 16
	m.empty = draws.NewText("No conversations")
	m.empty.SetFace(emptyOpts)
}

func (m *MessageApp) layout() {
	sa := m.ctx.SafeArea()
	stamp := strings.Join([]string{
		ftoa(m.screenW), ftoa(m.screenH), ftoa(sa.Top), ftoa(sa.Bottom),
	}, ":")
	if stamp == m.layoutStamp {
		return
	}
	m.layoutStamp = stamp

	top := max(messageStatusH, sa.Top)
	m.title.Locate(messagePad, top+messageBarH/2, draws.LeftMiddle)
	m.subtitle.Locate(messagePad, top+messageBarH/2+15, draws.LeftMiddle)
	m.empty.Locate(m.screenW/2, m.screenH/2, draws.CenterMiddle)

	m.back = ui.NewButton("<", 18, messagePad, top+10, 34, 32, messageBackBg)
	sendW := 64.0
	fieldX := messagePad
	fieldW := m.screenW - messagePad*3 - sendW
	composerTop := m.composerTop()
	oldValue := m.composer.Value
	wasFocused := m.composer.IsFocused()
	m.composer = ui.NewTextField(fieldX, composerTop+8, fieldW, "Message")
	m.composer.MaxLen = 180
	m.composer.Value = oldValue
	if wasFocused {
		m.composer.Focus()
	}
	m.send = ui.NewButton("Send", 13, fieldX+fieldW+messagePad, composerTop+8, sendW, ui.TextFieldH, messageSendBg)

	m.msgScroll.Size = draws.XY{X: m.screenW, Y: max(0, composerTop-top-messageBarH)}
	m.msgScroll.Locate(0, top+messageBarH, draws.LeftTop)
	m.rebuildTiles()
}

func (m *MessageApp) composerTop() float64 {
	sa := m.ctx.SafeArea()
	return m.screenH - max(messageComposerH, sa.Bottom+messageComposerH)
}

func (m *MessageApp) rebuildTiles() {
	m.tiles = m.tiles[:0]
	listTop := max(messageStatusH, m.ctx.SafeArea().Top) + messageBarH
	for i, th := range m.threads {
		tile := ui.NewListTile(58, listTop+float64(i)*ui.ListTileH, m.screenW-58, th.Name, th.Preview)
		if th.Unread > 0 {
			tile.TrailingText = itoa(th.Unread)
		}
		m.tiles = append(m.tiles, tile)
	}
}

func (m *MessageApp) Update(frame mosapp.Frame) {
	m.layout()
	m.updateTransition()
	if m.moving {
		return
	}
	switch m.view {
	case messageInbox:
		m.updateInbox(frame)
	case messageThread:
		m.updateThread(frame)
	}
}

func (m *MessageApp) updateInbox(frame mosapp.Frame) {
	for i := range m.tiles {
		if m.tiles[i].Update(frame) {
			m.active = i
			m.threads[i].Unread = 0
			m.ctx.HideKeyboard()
			m.rebuildTiles()
			m.scrollMessagesToEnd()
			m.startTransition(messageThread)
			return
		}
	}
}

func (m *MessageApp) updateThread(frame mosapp.Frame) {
	if m.back.Update(frame) {
		m.composer.Blur()
		m.ctx.HideKeyboard()
		m.startTransition(messageInbox)
		return
	}

	m.msgScroll.Update(frame)
	if m.composer.Update(frame) {
		if m.composer.IsFocused() {
			m.ctx.ShowKeyboard()
		} else {
			m.ctx.HideKeyboard()
		}
	}
	m.composer.PollKeyboard()

	if m.send.Update(frame) {
		m.sendCurrent()
	}
	for _, ev := range frame.Events {
		if ev.Kind == input.EventUp && ev.Pos.Y < m.composerTop() && !m.composer.IsFocused() {
			m.ctx.HideKeyboard()
		}
	}
}

func (m *MessageApp) sendCurrent() {
	if m.active < 0 || m.active >= len(m.threads) {
		return
	}
	body := strings.TrimSpace(m.composer.Value)
	if body == "" {
		return
	}
	th := &m.threads[m.active]
	th.Messages = append(th.Messages, messageBubble{Body: body, Mine: true, At: time.Now()})
	th.Preview = body
	m.composer.Clear()
	m.ctx.PostNotice(mosapp.Notice{Title: "Message sent", Body: th.Name + ": " + truncate(body, 40)})
	if bus := m.ctx.Bus(); bus != nil {
		bus.Publish(event.Custom{
			Topic: "mos/message/sent",
			Data:  messageIncoming{From: th.Name, Body: body},
		})
	}
	m.scrollMessagesToEnd()
}

func (m *MessageApp) receive(from, body string) {
	idx := -1
	for i := range m.threads {
		if m.threads[i].Name == from {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.threads = append([]messageThreadModel{{Name: from}}, m.threads...)
		idx = 0
	}
	th := &m.threads[idx]
	th.Messages = append(th.Messages, messageBubble{Body: body, At: time.Now()})
	th.Preview = body
	if m.view != messageThread || m.active != idx {
		th.Unread++
	}
	m.rebuildTiles()
	m.ctx.PostNotice(mosapp.Notice{Title: from, Body: body})
}

func (m *MessageApp) scrollMessagesToEnd() {
	m.layoutMessages()
	m.msgScroll.ScrollBy(draws.XY{Y: m.msgScroll.ContentSize.Y})
}

func (m *MessageApp) startTransition(to messageView) {
	if m.view == to {
		return
	}
	m.moveFrom = m.view
	m.moveTo = to
	m.moveForward = to == messageThread
	m.moving = true
	m.transition = tween.Tween{MaxLoop: 1}
	m.transition.Add(0, 1, 260*time.Millisecond, tween.EaseOutExponential)
	m.transition.Start()
}

func (m *MessageApp) updateTransition() {
	if !m.moving {
		return
	}
	m.transition.Update()
	if !m.transition.IsFinished() {
		return
	}
	m.moving = false
	m.view = m.moveTo
	if m.view == messageInbox {
		m.active = -1
	}
}

func (m *MessageApp) Draw(dst draws.Image) {
	m.layout()
	if m.moving {
		m.drawTransition(dst)
		return
	}
	m.drawView(dst, m.view)
}

func (m *MessageApp) drawTransition(dst draws.Image) {
	dst.Fill(messageBg)
	p := m.transition.Value()
	if p < 0 {
		p = 0
	} else if p > 1 {
		p = 1
	}

	from := draws.CreateImage(m.screenW, m.screenH)
	to := draws.CreateImage(m.screenW, m.screenH)
	m.drawView(from, m.moveFrom)
	m.drawView(to, m.moveTo)

	fromX, toX := -m.screenW*p*0.28, m.screenW*(1-p)
	if !m.moveForward {
		fromX, toX = m.screenW*p*0.28, -m.screenW*(1-p)
	}
	m.drawPanel(dst, from, fromX, 1-p*0.55)
	m.drawPanel(dst, to, toX, 0.35+p*0.65)
}

func (m *MessageApp) drawPanel(dst draws.Image, img draws.Image, x, alpha float64) {
	sp := draws.NewSprite(img)
	sp.Locate(x, 0, draws.LeftTop)
	sp.ColorScale.ScaleAlpha(float32(alpha))
	sp.Draw(dst)
}

func (m *MessageApp) drawView(dst draws.Image, view messageView) {
	dst.Fill(messageBg)
	m.drawBar(dst, view)
	if view == messageInbox {
		m.drawInbox(dst)
		return
	}
	m.drawThread(dst)
}

func (m *MessageApp) drawBar(dst draws.Image, view messageView) {
	top := max(messageStatusH, m.ctx.SafeArea().Top)
	bar := draws.CreateImage(m.screenW, messageBarH)
	bar.Fill(messageBarBg)
	sp := draws.NewSprite(bar)
	sp.Locate(0, top, draws.LeftTop)
	sp.Draw(dst)
	if view == messageThread && m.active >= 0 {
		m.back.Draw(dst)
		m.title.Text = m.threads[m.active].Name
		m.title.Locate(messagePad+48, top+20, draws.LeftMiddle)
		m.subtitle.Text = "Messages"
		m.subtitle.Locate(messagePad+48, top+37, draws.LeftMiddle)
		m.title.Draw(dst)
		m.subtitle.ColorScale.Scale(0.65, 0.65, 0.65, 1)
		m.subtitle.Draw(dst)
		m.subtitle.ColorScale = ebiten.ColorScale{}
		return
	}
	m.title.Text = "Messages"
	m.title.Locate(messagePad, top+messageBarH/2, draws.LeftMiddle)
	m.title.Draw(dst)
}

func (m *MessageApp) drawInbox(dst draws.Image) {
	if len(m.threads) == 0 {
		m.empty.Draw(dst)
		return
	}
	listTop := max(messageStatusH, m.ctx.SafeArea().Top) + messageBarH
	for i := range m.tiles {
		rowDelay := time.Duration(i) * 45 * time.Millisecond
		rowP := easeSince(m.createdAt.Add(rowDelay), 360*time.Millisecond)
		rowX := (1 - rowP) * 26
		row := draws.CreateImage(m.screenW, ui.ListTileH)
		row.Fill(messageListBg)
		rowSp := draws.NewSprite(row)
		rowSp.Locate(rowX, listTop+float64(i)*ui.ListTileH, draws.LeftTop)
		rowSp.ColorScale.ScaleAlpha(float32(rowP))
		rowSp.Draw(dst)
		m.drawAvatar(dst, messagePad+18+rowX, listTop+float64(i)*ui.ListTileH+ui.ListTileH/2, initials(m.threads[i].Name), rowP)
		tile := ui.NewListTile(58+rowX, listTop+float64(i)*ui.ListTileH, m.screenW-58-rowX, m.threads[i].Name, m.threads[i].Preview)
		if m.threads[i].Unread > 0 {
			tile.TrailingText = itoa(m.threads[i].Unread)
		}
		tile.Draw(dst)
		div := draws.CreateImage(m.screenW-messagePad*2, 1)
		div.Fill(messageDivider)
		divSp := draws.NewSprite(div)
		divSp.Locate(messagePad+rowX, listTop+float64(i+1)*ui.ListTileH-1, draws.LeftTop)
		divSp.ColorScale.ScaleAlpha(float32(rowP))
		divSp.Draw(dst)
	}
}

func (m *MessageApp) drawThread(dst draws.Image) {
	if m.active < 0 || m.active >= len(m.threads) {
		return
	}
	m.layoutMessages()
	m.drawMessages(dst)
	m.drawComposer(dst)
}

func (m *MessageApp) layoutMessages() {
	if m.active < 0 || m.active >= len(m.threads) {
		return
	}
	y := messagePad
	for _, msg := range m.threads[m.active].Messages {
		lines := wrapText(msg.Body, 30)
		y += float64(len(lines))*18 + 24 + messageGap
	}
	m.msgScroll.ContentSize = draws.XY{X: m.screenW, Y: y + messagePad}
}

func (m *MessageApp) drawMessages(dst draws.Image) {
	top := max(messageStatusH, m.ctx.SafeArea().Top) + messageBarH
	bottom := m.composerTop()
	viewportH := max(0, bottom-top)
	if viewportH <= 0 {
		return
	}
	canvasH := max(viewportH, m.msgScroll.ContentSize.Y)
	canvas := draws.CreateImage(m.screenW, canvasH)
	canvas.Fill(messageBg)

	y := messagePad - m.msgScroll.Offset().Y
	for _, msg := range m.threads[m.active].Messages {
		y = m.drawBubble(canvas, msg, y)
	}

	clipY := 0
	clipH := int(min(viewportH, canvasH))
	if clipH <= 0 {
		return
	}
	sub := canvas.Image.SubImage(image.Rect(0, clipY, int(m.screenW), clipY+clipH)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, top)
	dst.DrawImage(sub, op)
}

func (m *MessageApp) drawBubble(dst draws.Image, msg messageBubble, y float64) float64 {
	lines := wrapText(msg.Body, 30)
	textH := float64(len(lines)) * 18
	bubbleW := min(m.screenW*0.72, maxTextLine(lines)*7+28)
	bubbleH := textH + 20
	p := easeSince(msg.At, 280*time.Millisecond)
	y += (1 - p) * 12
	x := messagePad
	bg := messageTheirBg
	if msg.Mine {
		x = m.screenW - messagePad - bubbleW
		bg = messageMineBg
		x += (1 - p) * 20
	} else {
		x -= (1 - p) * 20
	}

	img := roundedRectImage(bubbleW, bubbleH, 12, bg)
	sp := draws.NewSprite(img)
	sp.Locate(x, y, draws.LeftTop)
	sp.ColorScale.ScaleAlpha(float32(p))
	sp.Draw(dst)

	opts := draws.NewFaceOptions()
	opts.Size = 14
	for i, line := range lines {
		t := draws.NewText(line)
		t.SetFace(opts)
		t.Locate(x+14, y+13+float64(i)*18, draws.LeftTop)
		t.ColorScale.ScaleAlpha(float32(p))
		t.Draw(dst)
	}
	return y + bubbleH + messageGap - (1-p)*12
}

func (m *MessageApp) drawComposer(dst draws.Image) {
	top := m.composerTop()
	bg := draws.CreateImage(m.screenW, m.screenH-top)
	bg.Fill(messageCompose)
	sp := draws.NewSprite(bg)
	sp.Locate(0, top, draws.LeftTop)
	sp.Draw(dst)
	m.composer.Draw(dst)
	m.send.Draw(dst)
}

func (m *MessageApp) drawAvatar(dst draws.Image, cx, cy float64, label string, alpha float64) {
	img := roundedRectImage(36, 36, 18, messageAvatarBg)
	sp := draws.NewSprite(img)
	sp.Locate(cx, cy, draws.CenterMiddle)
	sp.ColorScale.ScaleAlpha(float32(alpha))
	sp.Draw(dst)
	opts := draws.NewFaceOptions()
	opts.Size = 13
	t := draws.NewText(label)
	t.SetFace(opts)
	t.Locate(cx, cy, draws.CenterMiddle)
	t.ColorScale.ScaleAlpha(float32(alpha))
	t.Draw(dst)
}

func roundedRectImage(w, h, r float64, clr color.RGBA) draws.Image {
	img := draws.CreateImage(w, h)
	fw, fh, fr := float32(w), float32(h), float32(r)
	vector.DrawFilledRect(img.Image, fr, 0, fw-2*fr, fh, clr, true)
	vector.DrawFilledRect(img.Image, 0, fr, fw, fh-2*fr, clr, true)
	vector.DrawFilledCircle(img.Image, fr, fr, fr, clr, true)
	vector.DrawFilledCircle(img.Image, fw-fr, fr, fr, clr, true)
	vector.DrawFilledCircle(img.Image, fr, fh-fr, fr, clr, true)
	vector.DrawFilledCircle(img.Image, fw-fr, fh-fr, fr, clr, true)
	return img
}

func wrapText(s string, maxRunes int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := ""
	for _, word := range words {
		if line == "" {
			line = word
			continue
		}
		if len([]rune(line))+1+len([]rune(word)) <= maxRunes {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func maxTextLine(lines []string) float64 {
	longest := 0
	for _, line := range lines {
		if n := len([]rune(line)); n > longest {
			longest = n
		}
	}
	return float64(longest)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "..."
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	r := []rune(parts[0])
	if len(r) == 0 {
		return "?"
	}
	return strings.ToUpper(string(r[0]))
}

func ftoa(v float64) string {
	return itoa(int(v * 100))
}

func easeSince(start time.Time, duration time.Duration) float64 {
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0
	}
	if elapsed >= duration {
		return 1
	}
	x := elapsed.Seconds() / duration.Seconds()
	return 1 - math.Pow(1-x, 3)
}
