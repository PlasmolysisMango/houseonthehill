package tile

import "github.com/hajimehoshi/ebiten/v2"

// Source identifies which set a room tile belongs to.
type Source int

const (
	SourceStarter Source = iota
	SourceBase
	SourceExtension
)

// Side is a world-space cardinal direction. Indices match board.Directions.
const (
	SideUp    = 0 // -y
	SideRight = 1 // +x
	SideDown  = 2 // +y
	SideLeft  = 3 // -x
)

// Floor identifies a vertical floor of the haunted mansion. The values match
// the layout used by tileMeta in the assets package: [roof, upper, ground,
// basement]. Tiles carry a [4]bool mask declaring which floors they may be
// drawn on.
const (
	FloorRoof     = 0
	FloorUpper    = 1
	FloorGround   = 2
	FloorBasement = 3
)

// OppositeSide returns the side index facing the opposite direction.
func OppositeSide(s int) int { return (s + 2) % 4 }

// RoomTile is the immutable artwork for a room plus mutable rotation state.
type RoomTile struct {
	ID       int
	Source   Source
	Front    *ebiten.Image
	Back     *ebiten.Image
	Rotation int // 0..3, each unit = 90 degrees clockwise

	// NameCN/NameEN are the human-readable room names sourced from
	// assets/datafs/tile_meta.yaml. They are empty when the yaml row has
	// not been filled in yet, in which case callers should fall back to
	// the tile ID.
	NameCN string
	NameEN string

	// Doors holds whether the tile has a door on each original side
	// (rotation==0). Index order matches the Side* constants.
	Doors [4]bool

	// Floors records on which floors the tile may be placed. Index order
	// matches the Floor* constants. Rotation does not affect this mask.
	Floors [4]bool
}

// RotateCW rotates the tile 90 degrees clockwise.
func (t *RoomTile) RotateCW() {
	t.Rotation = (t.Rotation + 1) % 4
}

// HasDoor reports whether, at the tile's current rotation, the given world
// side has a door. CW rotation by r maps original side i -> world side (i+r)%4,
// so to look up world side w we read original side (w-r) mod 4.
func (t *RoomTile) HasDoor(worldSide int) bool {
	idx := ((worldSide-t.Rotation)%4 + 4) % 4
	return t.Doors[idx]
}

// DoorCount returns the number of doors regardless of rotation.
func (t *RoomTile) DoorCount() int {
	n := 0
	for _, d := range t.Doors {
		if d {
			n++
		}
	}
	return n
}

// AllowsFloor reports whether the tile may be placed on the given floor.
func (t *RoomTile) AllowsFloor(floor int) bool {
	if floor < 0 || floor >= len(t.Floors) {
		return false
	}
	return t.Floors[floor]
}
