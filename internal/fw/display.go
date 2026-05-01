package fw

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
	id      int
	spec    string
	powered bool
}
