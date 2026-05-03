package windowing

import (
	"image/color"

	"github.com/hndada/mos/apps"
	mosapp "github.com/hndada/mos/internal/app"
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
	// Height returns the pixels reserved at the bottom of the screen while
	// the keyboard is shown (0 when hidden). Reported as the final target
	// height even mid-slide so app layout doesn't reflow each frame.
	Height() float64
	Update()
	Draw(dst draws.Image)
}

type StatusBar interface {
	// Height returns the pixels reserved at the top of the screen.
	Height() float64
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
	// Update receives the per-frame input frame. While IsLocked, the
	// windowing server gates all other input layers and routes events here
	// only — typically a swipe-up gesture unlocks.
	Update(frame mosapp.Frame)
	Draw(dst draws.Image)
}
