package windowing

import (
	"image/color"

	"github.com/hndada/mos/apps"
	"github.com/hndada/mos/internal/draws"
)

type Wallpaper interface {
	Draw(dst draws.Image)
}

type Home interface {
	Update()
	Draw(dst draws.Image)
	TappedIcon() (pos, size draws.XY, clr color.RGBA, appID string, ok bool)
}

type History interface {
	AddCard(entry apps.HistoryEntry)
	RemoveCard()
	Show()
	Hide()
	IsVisible() bool
	IsInteractive() bool
	TappedCard() (pos, size draws.XY, entry apps.HistoryEntry, ok bool)
	CardRect() (center, size draws.XY)
	Entries() []apps.HistoryEntry
	Update()
	Draw(dst draws.Image)
}

type Keyboard interface {
	Show()
	Hide()
	IsVisible() bool
	Update()
	Draw(dst draws.Image)
}

type StatusBar interface {
	Update()
	Draw(dst draws.Image)
}

type Curtain interface {
	Show()
	Hide()
	Toggle()
	IsVisible() bool
	Update()
	Draw(dst draws.Image)
}

type Lock interface {
	Lock()
	Unlock()
	IsLocked() bool
	Update()
	Draw(dst draws.Image)
}
