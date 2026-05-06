package tween

import (
	"testing"
	"time"
)

// ── Easing functions ──────────────────────────────────────────────────────────

func TestEaseLinear(t *testing.T) {
	const b, c, d = 5.0, 10.0, time.Second

	cases := []struct {
		name string
		at   time.Duration
		want float64
	}{
		{"at start", 0, 5},
		{"at half", 500 * time.Millisecond, 10},
		{"at end", d, 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EaseLinear(tc.at, b, c, d)
			if got != tc.want {
				t.Errorf("EaseLinear(%v) = %v, want %v", tc.at, got, tc.want)
			}
		})
	}
}

func TestEaseOutExponential(t *testing.T) {
	const b, c, d = 0.0, 100.0, time.Second

	t.Run("at t=0 returns begin", func(t *testing.T) {
		got := EaseOutExponential(0, b, c, d)
		if got != b {
			t.Errorf("got %v, want %v", got, b)
		}
	})
	t.Run("at t>=duration returns begin+change", func(t *testing.T) {
		got := EaseOutExponential(d, b, c, d)
		if got != b+c {
			t.Errorf("got %v, want %v", got, b+c)
		}
	})
	t.Run("midpoint is between begin and begin+change", func(t *testing.T) {
		got := EaseOutExponential(500*time.Millisecond, b, c, d)
		if got <= b || got >= b+c {
			t.Errorf("midpoint %v should be in (%v, %v)", got, b, b+c)
		}
	})
	t.Run("monotonically increasing", func(t *testing.T) {
		prev := EaseOutExponential(0, b, c, d)
		steps := []time.Duration{100, 250, 500, 750, 999}
		for _, ms := range steps {
			cur := EaseOutExponential(ms*time.Millisecond, b, c, d)
			if cur <= prev {
				t.Errorf("not monotone at %v ms: %v <= %v", ms, cur, prev)
			}
			prev = cur
		}
	})
}

// ── Transition ────────────────────────────────────────────────────────────────

func TestTransition_Snap(t *testing.T) {
	var tr Transition

	tr.Snap(42)
	if v := tr.Value(); v != 42 {
		t.Errorf("Value() = %v, want 42", v)
	}
	if !tr.Done() {
		t.Error("Done() should be true after Snap")
	}
}

func TestTransition_ZeroValue(t *testing.T) {
	var tr Transition // zero value
	if v := tr.Value(); v != 0 {
		t.Errorf("zero-value Transition.Value() = %v, want 0", v)
	}
	if !tr.Done() {
		t.Error("zero-value Transition.Done() should be true")
	}
}

func TestTransition_SnapOverwrite(t *testing.T) {
	var tr Transition
	tr.Snap(10)
	tr.Snap(99)
	if v := tr.Value(); v != 99 {
		t.Errorf("Value() after second Snap = %v, want 99", v)
	}
}

func TestTransition_To_DoneEventually(t *testing.T) {
	var tr Transition
	tr.To(100, 50*time.Millisecond, EaseLinear)
	if tr.Done() {
		t.Error("Done() should be false immediately after To")
	}
	time.Sleep(60 * time.Millisecond)
	if !tr.Done() {
		t.Error("Done() should be true after duration elapsed")
	}
	if v := tr.Value(); v != 100 {
		t.Errorf("Value() after done = %v, want 100", v)
	}
}

func TestTransition_To_Retarget(t *testing.T) {
	// Retarget mid-flight: value must not jump backwards.
	var tr Transition
	tr.Snap(0)
	tr.To(100, 100*time.Millisecond, EaseLinear)
	time.Sleep(40 * time.Millisecond)
	mid := tr.Value()
	if mid <= 0 {
		t.Fatalf("mid-flight value %v should be > 0", mid)
	}
	// Retarget to 0 from current mid position.
	tr.To(0, 100*time.Millisecond, EaseLinear)
	// Immediately after retarget, from == mid so value should still be ~ mid.
	after := tr.Value()
	const tol = 5.0
	if after < mid-tol || after > mid+tol {
		t.Errorf("retarget jumped: before=%v after=%v, expected ~%v", mid, after, mid)
	}
}

// ── Tween ─────────────────────────────────────────────────────────────────────

func TestTween_Empty(t *testing.T) {
	var tw Tween
	if tw.Value() != 0 {
		t.Errorf("empty Tween.Value() = %v, want 0", tw.Value())
	}
	if !tw.IsFinished() {
		t.Error("empty Tween should be finished")
	}
}

func TestTween_SingleUnit_InitialValue(t *testing.T) {
	var tw Tween
	tw.MaxLoop = 1
	tw.Add(0, 100, time.Second, EaseLinear)
	tw.Start()

	// At t≈0, value should be ≈ begin (0).
	v := tw.Value()
	const tol = 1.0
	if v < -tol || v > tol {
		t.Errorf("initial Value() = %v, want ≈ 0", v)
	}
	if tw.IsFinished() {
		t.Error("single-unit tween should not be finished immediately")
	}
}

func TestTween_SingleUnit_FinishesAfterDuration(t *testing.T) {
	var tw Tween
	tw.MaxLoop = 1
	tw.Add(0, 100, 50*time.Millisecond, EaseLinear)
	tw.Start()

	time.Sleep(60 * time.Millisecond)
	tw.Update()

	if !tw.IsFinished() {
		t.Error("tween should be finished after duration")
	}
	v := tw.Value()
	if v != 100 {
		t.Errorf("final Value() = %v, want 100", v)
	}
}

func TestTween_Stop(t *testing.T) {
	var tw Tween
	tw.MaxLoop = 1
	tw.Add(0, 100, time.Second, EaseLinear)
	tw.Start()
	tw.Stop()

	if !tw.IsFinished() {
		t.Error("IsFinished() should be true after Stop()")
	}
}

func TestTween_InfiniteLoop(t *testing.T) {
	var tw Tween
	// MaxLoop == 0 means infinite
	tw.Add(0, 100, 30*time.Millisecond, EaseLinear)
	tw.Start()

	time.Sleep(70 * time.Millisecond)
	tw.Update()

	// Should NOT be finished even after 2+ iterations.
	if tw.IsFinished() {
		t.Error("infinite-loop tween should never finish on its own")
	}
}

func TestTween_MultiUnit(t *testing.T) {
	// Use 60ms per unit (120ms total) with generous sleep margins.
	var tw Tween
	tw.MaxLoop = 1
	tw.Add(0, 50, 60*time.Millisecond, EaseLinear)  // unit 0: 0→50
	tw.Add(50, 50, 60*time.Millisecond, EaseLinear) // unit 1: 50→100
	tw.Start()

	// Mid unit 0 (at ~30ms).
	time.Sleep(30 * time.Millisecond)
	v := tw.Value()
	if v < 0 || v > 50 {
		t.Errorf("mid-unit-0 value %v out of [0,50]", v)
	}

	// Past unit 0, into unit 1 (at ~80ms).
	time.Sleep(50 * time.Millisecond)
	tw.Update()
	v = tw.Value()
	if v < 50 || v > 100 {
		t.Errorf("mid-unit-1 value %v out of [50,100]", v)
	}

	// Past unit 1 — tween should be complete (at ~150ms, need 120ms).
	time.Sleep(70 * time.Millisecond)
	tw.Update()
	if !tw.IsFinished() {
		t.Error("two-unit tween should be finished")
	}
	if tw.Value() != 100 {
		t.Errorf("final value = %v, want 100", tw.Value())
	}
}
