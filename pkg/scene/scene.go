package scene

import "github.com/hajimehoshi/ebiten/v2"

// Scene is one screen of the game (menu, gameplay, etc).
// Update returns the next scene to switch to (or itself to stay).
// Returning nil signals graceful termination.
type Scene interface {
	Update() (Scene, error)
	Draw(screen *ebiten.Image)
}
