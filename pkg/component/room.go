package component

import (
	"github.com/hajimehoshi/ebiten/v2"
)

var _ Displayable = &Room{}

func NewRoom(img *ebiten.Image, name string) *Room {
	return &Room{
		img:  img,
		name: name,
	}
}

type Room struct {
	img  *ebiten.Image
	name string
}

func (r *Room) Render() *ebiten.Image {
	return r.img
}

func (r *Room) Update() error {
	return nil
}
