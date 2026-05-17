package component

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/plasmolysismango/houseonthehill/pkg/utils"
)

var _ Displayable = &Room{}

func NewRoom(img *ebiten.Image, name string) *Room {
	minPoint := img.Bounds().Min
	maxPoint := img.Bounds().Max
	rect := NewRect(minPoint.X, minPoint.Y, maxPoint.X-minPoint.X, maxPoint.Y-minPoint.Y)
	return &Room{
		Rect: rect,
		img:  img,
		name: name,
	}
}

type Room struct {
	*Rect
	img  *ebiten.Image
	name string
}

func (r *Room) Draw(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(r.X), float64(r.Y))
	scaleRadio := utils.ScaleTo(r.img, screen)
	op.GeoM.Scale(scaleRadio, scaleRadio)
	screen.DrawImage(r.img, op)
}

func (r *Room) Update() error {
	return nil
}
