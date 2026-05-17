package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/plasmolysismango/houseonthehill/assets"
	"github.com/plasmolysismango/houseonthehill/pkg/component"
	"github.com/plasmolysismango/houseonthehill/pkg/utils"
	"image/color"
	_ "image/png"
	"log"
)

func NewGame() (*Game, error) {
	img, err := assets.LoadImage(assets.EntryGround)
	if err != nil {
		return nil, err
	}
	return &Game{
		img:      img,
		displays: component.NewRoom(img, assets.EntryGround),
		scale:    1.0,
	}, nil
}

type Game struct {
	img      *ebiten.Image
	displays *component.Room
	scale    float64
	x, y     float64
	mouse    utils.Mouse
}

func (g *Game) Update() error {
	defer func() {
		g.mouse.Update()
	}()
	x, y := g.mouse.LeftDrag()
	g.x += float64(x)
	g.y += float64(y)
	_, wy := g.mouse.Wheel()
	g.scale += float64(wy) / 10
	if g.mouse.IsClick(ebiten.MouseButtonMiddle) {
		g.x, g.y = 0, 0
		g.scale = 1
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	background := ebiten.NewImage(1200, 800)
	background.Fill(color.White)
	g.displays.Draw(background)
	op := &ebiten.DrawImageOptions{}
	minScale := min(1200.0/float64(background.Bounds().Dx()), 800.0/float64(background.Bounds().Dy()))
	op.GeoM.Scale(minScale*g.scale, minScale*g.scale)
	op.GeoM.Translate(g.x, g.y)
	screen.DrawImage(background, op)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return outsideWidth, outsideHeight
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
