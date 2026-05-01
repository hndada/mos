package windowing

type DisplayKind int

const (
	DisplayKindBuiltIn DisplayKind = iota
	DisplayKindExternal
	DisplayKindVirtual
)

// Display caches its chrome images so they are never allocated mid-frame.
// Dirty flags drive repaints only when state changes.
type Display struct {
	W, H    float64
	kind    DisplayKind
	powered bool
}

func (d *Display) Powered() bool      { return d.powered }
func (d *Display) SetPowered(on bool) { d.powered = on }
