package component

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/plasmolysismango/houseonthehill/pkg/tile"
)

// Room is a placed room on the board.
type Room struct {
	Tile     *tile.RoomTile
	Revealed bool // when false the back side is rendered
}

func NewRoom(t *tile.RoomTile) *Room {
	return &Room{Tile: t, Revealed: true}
}

// DrawTile renders a tile at world position (worldX, worldY) occupying a
// tileSize-by-tileSize square. useFront chooses front/back artwork. alpha is
// in [0,1]. The camera matrix is applied last to convert world->screen.
func DrawTile(dst *ebiten.Image, t *tile.RoomTile, useFront bool, worldX, worldY, tileSize float64, cam ebiten.GeoM, alpha float32) {
	var img *ebiten.Image
	if useFront || t.Back == nil {
		img = t.Front
	} else {
		img = t.Back
	}
	if img == nil {
		return
	}
	src := img.Bounds()
	sw, sh := float64(src.Dx()), float64(src.Dy())
	if sw == 0 || sh == 0 {
		return
	}

	var geo ebiten.GeoM
	// 1. scale source artwork to tileSize x tileSize
	geo.Scale(tileSize/sw, tileSize/sh)
	// 2. rotate around tile centre
	geo.Translate(-tileSize/2, -tileSize/2)
	geo.Rotate(float64(t.Rotation) * (math.Pi / 2))
	geo.Translate(tileSize/2, tileSize/2)
	// 3. translate to world position
	geo.Translate(worldX, worldY)
	// 4. apply camera (world->screen)
	geo.Concat(cam)

	op := &ebiten.DrawImageOptions{}
	op.GeoM = geo
	op.Filter = ebiten.FilterLinear
	if alpha < 1 {
		op.ColorScale.ScaleAlpha(alpha)
	}
	dst.DrawImage(img, op)
}

// Draw renders the placed room at full opacity.
func (r *Room) Draw(dst *ebiten.Image, worldX, worldY, tileSize float64, cam ebiten.GeoM) {
	useFront := r.Revealed || r.Tile.Back == nil
	DrawTile(dst, r.Tile, useFront, worldX, worldY, tileSize, cam, 1.0)
}
