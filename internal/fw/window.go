package fw

import "github.com/hndada/mos/ui"

type Lifecycle int

const (
	LifecycleInitializing Lifecycle = iota
	LifecycleShowing
	LifecycleShown
	LifecycleHiding
	LifecycleHidden
	LifecycleDestroying
	LifecycleDestroyed
)

func (l Lifecycle) Visible() bool {
	return l == LifecycleShowing || l == LifecycleShown || l == LifecycleHiding
}

// Window owns its canvas. The canvas is allocated once and reused every frame.
// dirty=true triggers a repaint; the compositor just blits the cached canvas.
type Window struct {
	ui.Box
	app *App
}
