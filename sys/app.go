package sys

type SignalType uint8

type Signal struct {
	SignalType
	ID   uint16
	Data any
}

type Screen any // TODO: any -> Image

// App developer imports App struct and define the behavior of the app.
type App struct {
	// Handle processes incoming signals.
	Handle func(Signal)
	Draw   func(Screen)
}
