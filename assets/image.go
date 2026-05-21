package assets

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/plasmolysismango/houseonthehill/assets/datafs"
	"github.com/plasmolysismango/houseonthehill/pkg/cards"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
	"github.com/plasmolysismango/houseonthehill/pkg/tile"
)

const (
	MainMap       = "main_map.jpg"
	MainMapBack   = "main_map_back.jpg"
	EntryGround   = "entry_hall.png"
	ExtendMap     = "extension_map.jpg"
	ExtendMapBack = "extension_map_back.jpg"
)

// TileMeta is the static, rotation-independent description of a room tile.
// It is populated lazily from assets/datafs/tile_meta.yaml on first use.
//   Doors  side order: [up, right, down, left] (matches tile.Side*).
//   Floors order:      [roof, upper, ground, basement] (matches tile.Floor*).
type TileMeta struct {
	Doors  [4]bool
	Floors [4]bool
}

//go:embed image
var imageAssets embed.FS

// tileCache holds everything we read out of tile_meta.yaml so the runtime
// only parses the yaml once. Geometry (Doors/Floors) used to live in the
// hand-generated assets/doors_gen.go file; that file has been removed and
// the yaml is now the single source of truth.
type tileCacheEntry struct {
	Meta      TileMeta
	NameCN    string
	NameEN    string
	StartCell *[2]int
}

var (
	tileCacheOnce sync.Once
	tileCache     map[int]tileCacheEntry
)

func loadTileCache() map[int]tileCacheEntry {
	tileCacheOnce.Do(func() {
		tileCache = map[int]tileCacheEntry{}
		rows, err := data.LoadTiles(datafs.FS)
		if err != nil {
			// Missing yaml is fatal: without it we have no door /
			// floor data at all. Surface to the loader callers via
			// empty map; LoadStarterTiles / loadDeckTiles will skip
			// every tile and the game scene will report no rooms.
			return
		}
		for _, r := range rows {
			var cell *[2]int
			if r.StartCell != nil {
				copy := *r.StartCell
				cell = &copy
			}
			tileCache[r.ID] = tileCacheEntry{
				Meta:      TileMeta{Doors: r.Doors, Floors: r.Floors},
				NameCN:    r.NameCN,
				NameEN:    r.NameEN,
				StartCell: cell,
			}
		}
	})
	return tileCache
}

func tileMetaOf(id int) (TileMeta, bool) {
	e, ok := loadTileCache()[id]
	if !ok {
		return TileMeta{}, false
	}
	return e.Meta, true
}

func tileNames() map[int]struct{ CN, EN string } {
	cache := loadTileCache()
	out := make(map[int]struct{ CN, EN string }, len(cache))
	for id, e := range cache {
		out[id] = struct{ CN, EN string }{CN: e.NameCN, EN: e.NameEN}
	}
	return out
}

// RoomRule returns the special rule text for a named room (matched by
// English name). Rooms without special rules return an empty string.
// The data is loaded lazily from rooms.yaml on first call.
var (
	roomRulesOnce sync.Once
	roomRulesMap  map[string]string
)

func RoomRule(nameEN string) string {
	roomRulesOnce.Do(func() {
		roomRulesMap = make(map[string]string)
		rooms, err := data.LoadRooms(datafs.FS)
		if err != nil {
			return
		}
		for _, r := range rooms {
			if r.RuleText != "" {
				roomRulesMap[r.NameEN] = r.RuleText
			}
		}
	})
	return roomRulesMap[nameEN]
}

func starterCells() map[int][2]int {
	cache := loadTileCache()
	out := map[int][2]int{}
	for id, e := range cache {
		if e.StartCell == nil {
			continue
		}
		out[id] = *e.StartCell
	}
	return out
}

// LoadImage decodes an embedded image file into an *ebiten.Image.
func LoadImage(name string) (*ebiten.Image, error) {
	data, err := imageAssets.ReadFile("image/" + name)
	if err != nil {
		return nil, fmt.Errorf("load image %s failed: %w", name, err)
	}
	rawImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image %s: %w", name, err)
	}
	return ebiten.NewImageFromImage(rawImage), nil
}

// cropGrid splits img into rows*cols sub-images by row-major order.
// Each tile uses floor(width/cols) x floor(height/rows) pixels.
func cropGrid(img *ebiten.Image, cols, rows int) []*ebiten.Image {
	bounds := img.Bounds()
	w := bounds.Dx() / cols
	h := bounds.Dy() / rows
	out := make([]*ebiten.Image, 0, cols*rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			x0 := bounds.Min.X + c*w
			y0 := bounds.Min.Y + r*h
			rect := image.Rect(x0, y0, x0+w, y0+h)
			sub := img.SubImage(rect).(*ebiten.Image)
			out = append(out, sub)
		}
	}
	return out
}

// StarterPlacement bundles a starter tile with the board cell it should be
// placed on. The cell is sourced from `start_cell` in tile_meta.yaml so the
// runtime never hard-codes entrance coordinates.
type StarterPlacement struct {
	Tile *tile.RoomTile
	X, Y int
}

// LoadStarterTiles returns the three pre-placed entrance rooms paired with
// their target board cells. They are always shown face-up so Front == Back.
// The image is laid out left-to-right as: [0] Grand Staircase, [1] Foyer,
// [2] Entrance Hall.
func LoadStarterTiles() ([]StarterPlacement, error) {
	img, err := LoadImage(EntryGround)
	if err != nil {
		return nil, err
	}
	imgs := cropGrid(img, 3, 1)
	cells := starterCells()
	out := make([]StarterPlacement, 0, len(imgs))
	for i, im := range imgs {
		id := 1000 + i
		meta, ok := tileMetaOf(id)
		if !ok {
			continue
		}
		cell, ok := cells[id]
		if !ok {
			return nil, fmt.Errorf("starter tile %d missing start_cell in tile_meta.yaml", id)
		}
		name := tileNames()[id]
		out = append(out, StarterPlacement{
			Tile: &tile.RoomTile{
				ID:     id,
				Source: tile.SourceStarter,
				Front:  im,
				Back:   im,
				NameCN: name.CN,
				NameEN: name.EN,
				Doors:  meta.Doors,
				Floors: meta.Floors,
			},
			X: cell[0],
			Y: cell[1],
		})
	}
	return out, nil
}

func loadDeckTiles(frontName, backName string, cols, rows int, idBase int, src tile.Source) ([]*tile.RoomTile, error) {
	front, err := LoadImage(frontName)
	if err != nil {
		return nil, err
	}
	back, err := LoadImage(backName)
	if err != nil {
		return nil, err
	}
	frontTiles := cropGrid(front, cols, rows)
	backTiles := cropGrid(back, cols, rows)
	if len(frontTiles) != len(backTiles) {
		return nil, fmt.Errorf("front/back tile count mismatch: %d vs %d", len(frontTiles), len(backTiles))
	}
	out := make([]*tile.RoomTile, 0, len(frontTiles))
	for i := range frontTiles {
		id := idBase + i
		meta, ok := tileMetaOf(id)
		if !ok {
			// Pre-generated table omits blank/doorless cells, so skip them.
			continue
		}
		name := tileNames()[id]
		out = append(out, &tile.RoomTile{
			ID:     id,
			Source: src,
			Front:  frontTiles[i],
			Back:   backTiles[i],
			NameCN: name.CN,
			NameEN: name.EN,
			Doors:  meta.Doors,
			Floors: meta.Floors,
		})
	}
	return out, nil
}

// LoadBaseDeck loads the base game's 50 explorable rooms (10 columns x 5 rows).
// LoadCharacters reads assets/datafs/characters.yaml and returns the
// 12 explorer cards (with stat tracks and colour). Thin wrapper around
// data.LoadCharacters that hides the embed.FS detail from callers.
func LoadCharacters() ([]data.Character, error) {
	return data.LoadCharacters(datafs.FS)
}

func LoadBaseDeck() ([]*tile.RoomTile, error) {
	return loadDeckTiles(MainMap, MainMapBack, 10, 5, 0, tile.SourceBase)
}

// LoadExtensionDeck loads the DLC's 20 explorable rooms (10 columns x 2 rows).
func LoadExtensionDeck() ([]*tile.RoomTile, error) {
	return loadDeckTiles(ExtendMap, ExtendMapBack, 10, 2, 500, tile.SourceExtension)
}

// LoadCards parses cards.yaml and returns the three pre-bucketed slices
// (events, items, omens). The runtime then wraps each with a seeded Deck.
func LoadCards() (events, items, omens []*cards.Card, err error) {
	return cards.LoadAll(datafs.FS)
}

// roomKindMap is a name_cn → kind index built from rooms_doc.yaml.
// kind values are: "event" / "omen" / "item" / "base" / "unknown".
var (
	roomKindOnce sync.Once
	roomKindMap  map[string]string
)

// RoomKindOf returns the trigger kind for a room identified by its
// Chinese name. Empty string when the room is not in rooms_doc.yaml.
// Used by the game scene to decide which deck to draw from when a
// player first enters a tile.
func RoomKindOf(nameCN string) string {
	roomKindOnce.Do(func() {
		roomKindMap = make(map[string]string)
		rows, err := data.LoadRoomsDoc(datafs.FS)
		if err != nil {
			return
		}
		for _, r := range rows {
			roomKindMap[r.NameCN] = r.Kind
		}
	})
	return roomKindMap[nameCN]
}
