package player

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/plasmolysismango/houseonthehill/pkg/board"
)

// Player is the single explorer pawn.
type Player struct {
	Pos board.Cell
}

func New(start board.Cell) *Player { return &Player{Pos: start} }

// Draw paints the pawn at its current cell, transformed by the supplied camera matrix.
func (p *Player) Draw(dst *ebiten.Image, tileSize float64, cam ebiten.GeoM) {
	wx := float64(p.Pos.X)*tileSize + tileSize/2
	wy := float64(p.Pos.Y)*tileSize + tileSize/2
	sx, sy := cam.Apply(wx, wy)
	scale := cam.Element(0, 0)
	if scale <= 0 {
		scale = 1
	}
	r := float32(tileSize * 0.18 * scale)
	// outline + fill for visibility on busy backgrounds
	vector.DrawFilledCircle(dst, float32(sx), float32(sy), r+2, color.RGBA{R: 30, G: 30, B: 30, A: 255}, true)
	vector.DrawFilledCircle(dst, float32(sx), float32(sy), r, color.RGBA{R: 220, G: 50, B: 50, A: 255}, true)
}
