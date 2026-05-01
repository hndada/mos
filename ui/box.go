package ui

import "github.com/hndada/mos/internal/draws"

type Box struct {
	draws.Box
	ID        uint64
	isFocused bool
}

// TODO: drag and drop, drawing ghost window
