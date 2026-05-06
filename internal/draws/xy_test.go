package draws

import (
	"testing"
)

func TestXY_Add(t *testing.T) {
	got := XY{1, 2}.Add(XY{3, 4})
	want := XY{4, 6}
	if got != want {
		t.Errorf("Add = %v, want %v", got, want)
	}
}

func TestXY_Sub(t *testing.T) {
	got := XY{5, 3}.Sub(XY{2, 1})
	want := XY{3, 2}
	if got != want {
		t.Errorf("Sub = %v, want %v", got, want)
	}
}

func TestXY_Mul(t *testing.T) {
	got := XY{2, 3}.Mul(XY{4, 5})
	want := XY{8, 15}
	if got != want {
		t.Errorf("Mul = %v, want %v", got, want)
	}
}

func TestXY_Div(t *testing.T) {
	got := XY{8, 6}.Div(XY{2, 3})
	want := XY{4, 2}
	if got != want {
		t.Errorf("Div = %v, want %v", got, want)
	}
}

func TestXY_Scale(t *testing.T) {
	got := XY{3, 4}.Scale(2)
	want := XY{6, 8}
	if got != want {
		t.Errorf("Scale = %v, want %v", got, want)
	}
}

func TestXY_ScaleZero(t *testing.T) {
	got := XY{10, 20}.Scale(0)
	want := XY{0, 0}
	if got != want {
		t.Errorf("Scale(0) = %v, want %v", got, want)
	}
}

func TestNewXY(t *testing.T) {
	got := NewXY(7, 9)
	if got.X != 7 || got.Y != 9 {
		t.Errorf("NewXY = %v, want {7 9}", got)
	}
}

func TestNewXYFromInts(t *testing.T) {
	got := NewXYFromInts(3, 5)
	if got.X != 3 || got.Y != 5 {
		t.Errorf("NewXYFromInts = %v, want {3 5}", got)
	}
}

func TestNewXYFromScalar(t *testing.T) {
	got := NewXYFromScalar(11)
	if got.X != 11 || got.Y != 11 {
		t.Errorf("NewXYFromScalar = %v, want {11 11}", got)
	}
}

func TestXY_Values(t *testing.T) {
	x, y := XY{3, 7}.Values()
	if x != 3 || y != 7 {
		t.Errorf("Values() = (%v, %v), want (3, 7)", x, y)
	}
}

func TestXY_IntValues(t *testing.T) {
	x, y := XY{3.9, 7.1}.IntValues()
	if x != 3 || y != 7 {
		t.Errorf("IntValues() = (%v, %v), want (3, 7)", x, y)
	}
}

func TestXY_AddSub_RoundTrip(t *testing.T) {
	a := XY{10, 20}
	b := XY{3, 7}
	if got := a.Add(b).Sub(b); got != a {
		t.Errorf("Add then Sub roundtrip = %v, want %v", got, a)
	}
}

func TestXY_NegativeComponents(t *testing.T) {
	got := XY{-1, -2}.Add(XY{1, 2})
	want := XY{0, 0}
	if got != want {
		t.Errorf("negative Add = %v, want %v", got, want)
	}
}
