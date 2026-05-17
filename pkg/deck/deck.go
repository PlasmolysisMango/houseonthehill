package deck

import (
	"math/rand"

	"github.com/plasmolysismango/houseonthehill/pkg/tile"
)

// Deck is an ordered stack of room tiles. Draw pops the top tile.
type Deck struct {
	tiles []*tile.RoomTile
}

// New creates a shuffled deck from the given tiles. The original slice is not mutated.
func New(tiles []*tile.RoomTile, seed int64) *Deck {
	clone := make([]*tile.RoomTile, len(tiles))
	copy(clone, tiles)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(clone), func(i, j int) { clone[i], clone[j] = clone[j], clone[i] })
	return &Deck{tiles: clone}
}

// Draw removes and returns the top tile, or nil when empty.
func (d *Deck) Draw() *tile.RoomTile {
	if len(d.tiles) == 0 {
		return nil
	}
	t := d.tiles[len(d.tiles)-1]
	d.tiles = d.tiles[:len(d.tiles)-1]
	return t
}

// ReturnTop puts a tile back on top of the deck.
func (d *Deck) ReturnTop(t *tile.RoomTile) {
	if t == nil {
		return
	}
	d.tiles = append(d.tiles, t)
}

// Remaining returns the number of tiles left in the deck.
func (d *Deck) Remaining() int { return len(d.tiles) }
