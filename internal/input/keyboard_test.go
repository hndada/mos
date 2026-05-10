package input

import (
	"testing"
	"time"
)

func TestKeyboardReadEventsDrainsTransitionsByTime(t *testing.T) {
	kb := &Keyboard{
		keys: []Key{KeyD, KeyF},
		last: []bool{false, false},
	}

	kb.appendEvents(KeyboardState{
		Time:           1 * time.Millisecond,
		AreKeysPressed: []bool{true, false},
	})
	kb.appendEvents(KeyboardState{
		Time:           2 * time.Millisecond,
		AreKeysPressed: []bool{true, true},
	})
	kb.appendEvents(KeyboardState{
		Time:           3 * time.Millisecond,
		AreKeysPressed: []bool{false, true},
	})

	first := kb.ReadEvents(2 * time.Millisecond)
	if len(first) != 2 {
		t.Fatalf("ReadEvents returned %d events, want 2", len(first))
	}
	if first[0].Key != KeyD || first[0].Kind != KeyEventDown || first[0].Time != time.Millisecond {
		t.Fatalf("first event = %+v, want D down at 1ms", first[0])
	}
	if first[1].Key != KeyF || first[1].Kind != KeyEventDown || first[1].Time != 2*time.Millisecond {
		t.Fatalf("second event = %+v, want F down at 2ms", first[1])
	}

	second := kb.ReadEvents(10 * time.Millisecond)
	if len(second) != 1 {
		t.Fatalf("second ReadEvents returned %d events, want 1", len(second))
	}
	if second[0].Key != KeyD || second[0].Kind != KeyEventUp || second[0].Time != 3*time.Millisecond {
		t.Fatalf("third event = %+v, want D up at 3ms", second[0])
	}

	if rest := kb.ReadEvents(10 * time.Millisecond); len(rest) != 0 {
		t.Fatalf("ReadEvents returned already-drained events: %+v", rest)
	}
}

func TestKeyboardStateBufferOutputTrimsRedundantStates(t *testing.T) {
	kb := NewKeyboardStateBuffer([]KeyboardState{
		{Time: 0, AreKeysPressed: []bool{false}},
		{Time: 1 * time.Millisecond, AreKeysPressed: []bool{false}},
		{Time: 2 * time.Millisecond, AreKeysPressed: []bool{true}},
		{Time: 3 * time.Millisecond, AreKeysPressed: []bool{true}},
	})

	out := kb.Output()
	if len(out) != 2 {
		t.Fatalf("Output returned %d states, want 2", len(out))
	}
	if out[0].Time != 0 || out[0].AreKeysPressed[0] {
		t.Fatalf("first state = %+v, want unpressed at 0", out[0])
	}
	if out[1].Time != 2*time.Millisecond || !out[1].AreKeysPressed[0] {
		t.Fatalf("second state = %+v, want pressed at 2ms", out[1])
	}
}
