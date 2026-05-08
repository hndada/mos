package apps

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

type homeIcon struct {
	sprite draws.Sprite
	label  draws.Text
	color  color.RGBA
	appID  string
	folder int
}

type homeAppSpec struct {
	id   string
	name string
	kind string
	clr  color.RGBA
}

type homeFolderSpec struct {
	name string
	apps []homeAppSpec
}

// DefaultHome renders a grid of placeholder app icon slots and detects taps.
type DefaultHome struct {
	icons       []homeIcon
	folders     []homeFolderSpec
	folderIcons [][]homeIcon
	openFolder  int
	folderBg    draws.Sprite
	folderTitle draws.Text
	tappedPos   draws.XY
	tappedSize  draws.XY
	tappedColor color.RGBA
	tappedAppID string
	hasTap      bool
}

var homeApps = []homeAppSpec{
	{id: "gallery", name: "Gallery", kind: "gallery", clr: color.RGBA{0, 122, 255, 255}},
	{id: "settings", name: "Settings", kind: "settings", clr: color.RGBA{95, 99, 110, 255}},
	{id: "call", name: "Call", kind: "call", clr: color.RGBA{52, 199, 89, 255}},
	{id: "scene-test", name: "Scene", kind: "scene", clr: color.RGBA{255, 149, 0, 255}},
	{id: "hello", name: "Hello", kind: "hello", clr: color.RGBA{88, 86, 214, 255}},
	{id: "showcase", name: "Showcase", kind: "showcase", clr: color.RGBA{175, 82, 222, 255}},
	{id: "message", name: "Messages", kind: "message", clr: color.RGBA{255, 45, 85, 255}},
	{id: "color", name: "Color", kind: "color", clr: color.RGBA{90, 200, 250, 255}},
}

var homeFolders = []homeFolderSpec{
	{name: "Essentials", apps: []homeAppSpec{
		{id: "gallery", name: "Gallery", kind: "gallery", clr: color.RGBA{0, 122, 255, 255}},
		{id: "settings", name: "Settings", kind: "settings", clr: color.RGBA{95, 99, 110, 255}},
		{id: "call", name: "Call", kind: "call", clr: color.RGBA{52, 199, 89, 255}},
		{id: "message", name: "Messages", kind: "message", clr: color.RGBA{255, 45, 85, 255}},
	}},
	{name: "Lab", apps: []homeAppSpec{
		{id: "scene-test", name: "Scene", kind: "scene", clr: color.RGBA{255, 149, 0, 255}},
		{id: "hello", name: "Hello", kind: "hello", clr: color.RGBA{88, 86, 214, 255}},
		{id: "showcase", name: "Showcase", kind: "showcase", clr: color.RGBA{175, 82, 222, 255}},
		{id: "color", name: "Color", kind: "color", clr: color.RGBA{90, 200, 250, 255}},
	}},
}

func NewDefaultHome(screenW, screenH float64) *DefaultHome {
	const cols = 4
	rows := (len(homeApps) + len(homeFolders) + cols - 1) / cols
	const iconScale = 0.58
	const topPad = 0.12

	cellW := screenW / cols
	cellH := min(screenH*0.19, (screenH*(1-topPad)*0.62)/float64(rows))
	side := min(cellW, cellH) * iconScale

	icons := make([]homeIcon, 0, len(homeApps)+len(homeFolders))
	h := &DefaultHome{folders: homeFolders, openFolder: -1}
	for r := range rows {
		for c := range cols {
			idx := r*cols + c
			if idx >= len(homeApps)+len(homeFolders) {
				break
			}
			cx := (float64(c) + 0.5) * cellW
			cy := screenH*topPad + (float64(r)+0.5)*cellH

			if idx < len(homeApps) {
				spec := homeApps[idx]
				icons = append(icons, makeHomeIcon(cx, cy, side, spec, -1))
				continue
			}
			folderIdx := idx - len(homeApps)
			folder := homeFolders[folderIdx]
			img := newFolderIconImage(side, folder.apps)

			sp := draws.NewSprite(img)
			sp.Locate(cx, cy, draws.CenterMiddle)

			labelOpts := draws.NewFaceOptions()
			labelOpts.Size = max(9, side*0.16)
			label := draws.NewText(folder.name)
			label.SetFace(labelOpts)
			label.ColorScale.Scale(1, 1, 1, 1)
			label.Locate(cx, cy+side/2+10, draws.CenterTop)

			icons = append(icons, homeIcon{sprite: sp, label: label, color: color.RGBA{58, 62, 72, 255}, folder: folderIdx})
		}
	}

	h.icons = icons
	h.buildFolderViews(screenW, screenH, side)
	return h
}

func makeHomeIcon(cx, cy, side float64, spec homeAppSpec, folder int) homeIcon {
	sp := draws.NewSprite(newAppIconImage(side, spec))
	sp.Locate(cx, cy, draws.CenterMiddle)

	labelOpts := draws.NewFaceOptions()
	labelOpts.Size = max(9, side*0.16)
	label := draws.NewText(spec.name)
	label.SetFace(labelOpts)
	label.ColorScale.Scale(1, 1, 1, 1)
	label.Locate(cx, cy+side/2+10, draws.CenterTop)

	return homeIcon{sprite: sp, label: label, color: spec.clr, appID: spec.id, folder: folder}
}

func newAppIconImage(side float64, spec homeAppSpec) draws.Image {
	img := draws.CreateImage(side, side)
	s := float32(side)
	drawRoundedRect(img, 0, 0, s, s, s*0.23, spec.clr)
	vector.DrawFilledCircle(img.Image, s*0.78, s*0.22, s*0.18, color.RGBA{255, 255, 255, 28}, true)
	drawIconGlyph(img, s, spec.kind)
	return img
}

func newFolderIconImage(side float64, apps []homeAppSpec) draws.Image {
	img := draws.CreateImage(side, side)
	s := float32(side)
	drawRoundedRect(img, 0, 0, s, s, s*0.23, color.RGBA{54, 58, 70, 245})
	const pad = 0.16
	const cell = 0.28
	for i, app := range apps {
		if i >= 4 {
			break
		}
		x := s * (pad + float32(i%2)*0.36)
		y := s * (pad + float32(i/2)*0.36)
		drawRoundedRect(img, x, y, s*cell, s*cell, s*0.055, app.clr)
	}
	return img
}

func (h *DefaultHome) buildFolderViews(screenW, screenH, iconSide float64) {
	bgW := screenW * 0.82
	bgH := screenH * 0.48
	bg := draws.CreateImage(bgW, bgH)
	bg.Fill(color.RGBA{28, 31, 42, 238})
	h.folderBg = draws.NewSprite(bg)
	h.folderBg.Locate(screenW/2, screenH*0.42, draws.CenterMiddle)

	titleOpts := draws.NewFaceOptions()
	titleOpts.Size = 20
	h.folderTitle = draws.NewText("")
	h.folderTitle.SetFace(titleOpts)
	h.folderTitle.Locate(screenW/2, h.folderBg.Position.Y-bgH/2+24, draws.CenterTop)

	h.folderIcons = make([][]homeIcon, len(h.folders))
	folderIconSide := iconSide * 0.82
	const cols = 3
	cellW := bgW / cols
	cellH := folderIconSide + 42
	startX := h.folderBg.Position.X - bgW/2 + cellW/2
	startY := h.folderBg.Position.Y - bgH/2 + 82
	for i, folder := range h.folders {
		icons := make([]homeIcon, 0, len(folder.apps))
		for j, app := range folder.apps {
			cx := startX + float64(j%cols)*cellW
			cy := startY + float64(j/cols)*cellH
			icons = append(icons, makeHomeIcon(cx, cy, folderIconSide, app, -1))
		}
		h.folderIcons[i] = icons
	}
}

func drawRoundedRect(dst draws.Image, x, y, w, h, r float32, clr color.RGBA) {
	vector.DrawFilledRect(dst.Image, x+r, y, w-2*r, h, clr, true)
	vector.DrawFilledRect(dst.Image, x, y+r, w, h-2*r, clr, true)
	vector.DrawFilledCircle(dst.Image, x+r, y+r, r, clr, true)
	vector.DrawFilledCircle(dst.Image, x+w-r, y+r, r, clr, true)
	vector.DrawFilledCircle(dst.Image, x+r, y+h-r, r, clr, true)
	vector.DrawFilledCircle(dst.Image, x+w-r, y+h-r, r, clr, true)
}

func drawIconGlyph(dst draws.Image, s float32, kind string) {
	white := color.RGBA{255, 255, 255, 235}
	soft := color.RGBA{255, 255, 255, 160}
	switch kind {
	case "gallery":
		drawRoundedRect(dst, s*.22, s*.25, s*.56, s*.44, s*.05, white)
		vector.DrawFilledCircle(dst.Image, s*.62, s*.38, s*.055, color.RGBA{255, 214, 80, 255}, true)
		vector.StrokeLine(dst.Image, s*.28, s*.62, s*.43, s*.49, s*.045, color.RGBA{40, 160, 120, 255}, true)
		vector.StrokeLine(dst.Image, s*.43, s*.49, s*.57, s*.62, s*.045, color.RGBA{40, 160, 120, 255}, true)
		vector.StrokeLine(dst.Image, s*.52, s*.62, s*.66, s*.52, s*.04, color.RGBA{40, 120, 200, 255}, true)
	case "settings":
		vector.DrawFilledCircle(dst.Image, s*.5, s*.5, s*.22, white, true)
		vector.DrawFilledCircle(dst.Image, s*.5, s*.5, s*.105, color.RGBA{95, 99, 110, 255}, true)
		for _, p := range [][2]float32{{.5, .2}, {.5, .8}, {.2, .5}, {.8, .5}, {.29, .29}, {.71, .71}} {
			vector.DrawFilledCircle(dst.Image, s*p[0], s*p[1], s*.045, soft, true)
		}
	case "call":
		vector.StrokeLine(dst.Image, s*.34, s*.64, s*.66, s*.36, s*.12, white, true)
		vector.DrawFilledCircle(dst.Image, s*.33, s*.65, s*.085, white, true)
		vector.DrawFilledCircle(dst.Image, s*.67, s*.35, s*.085, white, true)
	case "scene":
		drawRoundedRect(dst, s*.24, s*.28, s*.36, s*.32, s*.04, white)
		drawRoundedRect(dst, s*.40, s*.40, s*.36, s*.32, s*.04, soft)
		vector.StrokeLine(dst.Image, s*.31, s*.46, s*.52, s*.46, s*.045, color.RGBA{255, 149, 0, 255}, true)
		vector.StrokeLine(dst.Image, s*.52, s*.46, s*.46, s*.40, s*.045, color.RGBA{255, 149, 0, 255}, true)
		vector.StrokeLine(dst.Image, s*.52, s*.46, s*.46, s*.52, s*.045, color.RGBA{255, 149, 0, 255}, true)
	case "hello":
		drawRoundedRect(dst, s*.22, s*.30, s*.56, s*.36, s*.09, white)
		vector.DrawFilledCircle(dst.Image, s*.36, s*.48, s*.035, color.RGBA{88, 86, 214, 255}, true)
		vector.DrawFilledCircle(dst.Image, s*.50, s*.48, s*.035, color.RGBA{88, 86, 214, 255}, true)
		vector.DrawFilledCircle(dst.Image, s*.64, s*.48, s*.035, color.RGBA{88, 86, 214, 255}, true)
	case "showcase":
		for r := range 2 {
			for c := range 2 {
				drawRoundedRect(dst, s*(.27+float32(c)*.25), s*(.27+float32(r)*.25), s*.19, s*.19, s*.035, white)
			}
		}
	case "message":
		drawRoundedRect(dst, s*.20, s*.30, s*.60, s*.34, s*.09, white)
		vector.StrokeLine(dst.Image, s*.38, s*.64, s*.29, s*.76, s*.08, white, true)
		vector.StrokeLine(dst.Image, s*.32, s*.43, s*.68, s*.43, s*.035, color.RGBA{255, 45, 85, 255}, true)
		vector.StrokeLine(dst.Image, s*.32, s*.53, s*.58, s*.53, s*.035, color.RGBA{255, 45, 85, 255}, true)
	case "color":
		vector.DrawFilledCircle(dst.Image, s*.48, s*.50, s*.26, white, true)
		vector.DrawFilledCircle(dst.Image, s*.39, s*.41, s*.04, color.RGBA{255, 70, 70, 255}, true)
		vector.DrawFilledCircle(dst.Image, s*.54, s*.39, s*.04, color.RGBA{255, 210, 60, 255}, true)
		vector.DrawFilledCircle(dst.Image, s*.58, s*.55, s*.04, color.RGBA{70, 190, 100, 255}, true)
		vector.DrawFilledCircle(dst.Image, s*.44, s*.59, s*.04, color.RGBA{40, 130, 255, 255}, true)
		vector.DrawFilledCircle(dst.Image, s*.63, s*.66, s*.075, color.RGBA{90, 200, 250, 255}, true)
	}
}

func (h *DefaultHome) Update() {
	h.hasTap = false
	if !input.IsMouseButtonJustPressed(input.MouseButtonLeft) {
		return
	}
	x, y := input.MouseCursorPosition()
	cursor := draws.XY{X: x, Y: y}

	if h.openFolder >= 0 {
		for _, icon := range h.folderIcons[h.openFolder] {
			if icon.sprite.In(cursor) {
				h.tappedPos = icon.sprite.Position
				h.tappedSize = icon.sprite.Size
				h.tappedColor = icon.color
				h.tappedAppID = icon.appID
				h.hasTap = true
				h.openFolder = -1
				return
			}
		}
		if !h.folderBg.In(cursor) {
			h.openFolder = -1
		}
		return
	}

	for _, icon := range h.icons {
		if icon.sprite.In(cursor) {
			if icon.folder >= 0 {
				h.openFolder = icon.folder
				return
			}
			h.tappedPos = icon.sprite.Position
			h.tappedSize = icon.sprite.Size
			h.tappedColor = icon.color
			h.tappedAppID = icon.appID
			h.hasTap = true
			return
		}
	}
}

func (h *DefaultHome) TappedIcon() (pos, size draws.XY, clr color.RGBA, appID string, ok bool) {
	return h.tappedPos, h.tappedSize, h.tappedColor, h.tappedAppID, h.hasTap
}

func (h *DefaultHome) Draw(dst draws.Image) {
	for _, icon := range h.icons {
		icon.sprite.Draw(dst)
		icon.label.Draw(dst)
	}
	if h.openFolder < 0 {
		return
	}
	h.folderBg.Draw(dst)
	h.folderTitle.Text = h.folders[h.openFolder].name
	h.folderTitle.Draw(dst)
	for _, icon := range h.folderIcons[h.openFolder] {
		icon.sprite.Draw(dst)
		icon.label.Draw(dst)
	}
}
