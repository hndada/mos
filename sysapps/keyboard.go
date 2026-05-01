package sysapps

import (
	"time"
)

const DurationKB = 220 * time.Millisecond

var keyboardRowsQwerty = [][]string{
	{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
	{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
	{"z", "x", "c", "v", "b", "n", "m"},
	{"space"},
}

type Keyboard interface {
	// Toggle uses slide animation. Lifecycle is driven by the tween OnDone.
	Toggle()
}
type DefaultKeyboard struct{}
