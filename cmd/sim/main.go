package main

import "github.com/hajimehoshi/ebiten/v2"

// Simulator drives the OS loop as an ebiten.Game.
// Physical buttons (power, display-type toggle) are handled here.
type Simulator struct{}

func (s *Simulator) Update() error { return nil }

func (s *Simulator) Draw(_ *ebiten.Image) {}

func (s *Simulator) Layout(w, h int) (int, int) { return w, h }

func main() {
	ebiten.RunGame(&Simulator{})
}
