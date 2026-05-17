package tile

import "github.com/hajimehoshi/ebiten/v2"

// Source identifies which set a room tile belongs to.
type Source int

const (
	SourceStarter Source = iota
	SourceBase
	SourceExtension
)

// RoomTile is the immutable artwork for a room plus mutable rotation state.
type RoomTile struct {
	ID       int
	Source   Source
	Front    *ebiten.Image
	Back     *ebiten.Image
	Rotation int // 0..3, each unit = 90 degrees clockwise
}

// RotateCW rotates the tile 90 degrees clockwise.
func (t *RoomTile) RotateCW() {
	t.Rotation = (t.Rotation + 1) % 4
}
