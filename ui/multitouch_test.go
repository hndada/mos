package ui

import (
	"testing"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/input"
)

func TestMultiTouchTrackerPinchScale(t *testing.T) {
	var tracker MultiTouchTracker

	first := tracker.Update(mosapp.Frame{Events: []input.Event{
		{Kind: input.EventDown, Pointer: 1, Pos: draws.XY{X: 0, Y: 0}},
		{Kind: input.EventDown, Pointer: 2, Pos: draws.XY{X: 10, Y: 0}},
	}})
	if !first.Active || first.Distance != 10 || first.Scale != 1 {
		t.Fatalf("first pinch = %+v, want active distance 10 scale 1", first)
	}

	second := tracker.Update(mosapp.Frame{Events: []input.Event{
		{Kind: input.EventMove, Pointer: 2, Pos: draws.XY{X: 20, Y: 0}},
	}})
	if !second.Active || second.Distance != 20 || second.Scale != 2 {
		t.Fatalf("second pinch = %+v, want active distance 20 scale 2", second)
	}

	ended := tracker.Update(mosapp.Frame{Events: []input.Event{
		{Kind: input.EventUp, Pointer: 1, Pos: draws.XY{X: 0, Y: 0}},
	}})
	if ended.Active || tracker.Count() != 1 {
		t.Fatalf("ended pinch = %+v count=%d, want inactive count 1", ended, tracker.Count())
	}
}
