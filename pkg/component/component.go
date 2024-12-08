package component

import "github.com/hajimehoshi/ebiten/v2"

type Displayable interface {
	Update() error
	// Render the object and return an ebiten.Image
	Render() *ebiten.Image
}
