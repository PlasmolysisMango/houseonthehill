package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/plasmolysismango/houseonthehill/pkg/scene"
)

const (
	screenW = 1200
	screenH = 800
)

// Game is the top-level Ebiten game; it delegates to the current Scene.
type Game struct {
	current scene.Scene
}

func (g *Game) Update() error {
	next, err := g.current.Update()
	if err != nil {
		return err
	}
	if next == nil {
		return ebiten.Termination
	}
	g.current = next
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.current != nil {
		g.current.Draw(screen)
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return screenW, screenH
}

func main() {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("House on the Hill")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	g := &Game{current: scene.NewMenuScene(screenW, screenH)}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
