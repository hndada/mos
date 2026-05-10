package draws

import "testing"

func TestViewportRelativeCoordinates(t *testing.T) {
	vp := NewViewport(400, 800)

	if got, want := vp.X(0.25), 100.0; got != want {
		t.Fatalf("X(.25) = %v, want %v", got, want)
	}
	if got, want := vp.Y(0.25), 200.0; got != want {
		t.Fatalf("Y(.25) = %v, want %v", got, want)
	}
	if got, want := vp.U(0.25), 100.0; got != want {
		t.Fatalf("U(.25) = %v, want %v", got, want)
	}

	r := vp.Rect(0.1, 0.2, 0.3, 0.4)
	if r.X != 40 || r.Y != 160 || r.W != 120 || r.H != 320 {
		t.Fatalf("Rect = %+v", r)
	}
}
