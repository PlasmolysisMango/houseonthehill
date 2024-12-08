package main

import (
	"github.com/plasmolysismango/houseonthehill/assets"
	"github.com/plasmolysismango/houseonthehill/pkg/component"
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

func NewGame() (*Game, error) {
	img, err := assets.LoadImage(assets.EntryGround)
	if err != nil {
		return nil, err
	}
	return &Game{
		img:      img,
		displays: component.NewRoom(img, assets.EntryGround),
	}, nil
}

type Game struct {
	img      *ebiten.Image
	displays component.Displayable
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	displayImg := g.displays.Render()
	op := &ebiten.DrawImageOptions{}
	minScale := min(1200.0/float64(displayImg.Bounds().Dx()), 800.0/float64(displayImg.Bounds().Dy()))
	op.GeoM.Scale(minScale, minScale)
	screen.DrawImage(displayImg, op)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 1200, 800
}

func main() {
	ebiten.SetWindowSize(1200, 800)
	ebiten.SetWindowTitle("Render an image")
	game, err := NewGame()
	if err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
