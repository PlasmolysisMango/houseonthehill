package component

import "github.com/hajimehoshi/ebiten/v2"

type Displayable interface {
	Update() error
	// Draw the object on the screen.
	Draw(screen *ebiten.Image)
}
