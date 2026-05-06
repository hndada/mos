package apps

import mosapp "github.com/hndada/mos/internal/app"

// init registers all built-in apps with the OS app registry.
// Third-party apps call mosapp.Register from their own init() functions.
func init() {
	mosapp.Register("gallery", newGalleryApp)
	mosapp.Register("hello", NewHelloApp)

	mosapp.Register("settings", func(ctx mosapp.Context) mosapp.Content {
		sz := ctx.ScreenSize()
		return NewSettings(sz.X, sz.Y)
	})

	mosapp.Register("call", func(ctx mosapp.Context) mosapp.Content {
		sz := ctx.ScreenSize()
		return NewCall(sz.X, sz.Y)
	})

	mosapp.Register("scene-test", func(ctx mosapp.Context) mosapp.Content {
		sz := ctx.ScreenSize()
		return NewSceneTest(sz.X, sz.Y)
	})

	mosapp.Register("showcase", NewShowcaseApp)
}
