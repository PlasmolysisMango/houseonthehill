package assets

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/plasmolysismango/houseonthehill/pkg/tile"
)

const (
	MainMap       = "主地图.jpg"
	MainMapBack   = "主地图-背面.jpg"
	EntryGround   = "入口大厅.png"
	ExtendMap     = "扩展地图.jpg"
	ExtendMapBack = "扩展地图-背面.jpg"
)

//go:embed image
var imageAssets embed.FS

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

// Door auto-detection.
//
// Door markers in the artwork are bright yellow brackets at the edge of a tile.
// We sample a band running from each edge ~25% inward and count pixels that
// match the door yellow color. If the ratio exceeds doorYellowRatio, the side
// is considered to have a door.
const (
	doorBandRatio   = 0.25   // how far the band extends from the edge
	doorYellowRatio = 0.0015 // strict-yellow pixel ratio threshold per band
)

func isDoorYellow(r8, g8, b8 int) bool {
	return r8 > 200 && g8 > 180 && b8 < 100 && r8-b8 > 100 && g8-b8 > 80
}

func bandYellowRatio(img *ebiten.Image, x0, y0, x1, y1 int) float64 {
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	yellow, total := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if isDoorYellow(int(r>>8), int(g>>8), int(b>>8)) {
				yellow++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(yellow) / float64(total)
}

// detectDoors infers which sides of img have a yellow door bracket.
// Side index follows tile.Side*: 0=up, 1=right, 2=down, 3=left.
func detectDoors(img *ebiten.Image) [4]bool {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w <= 0 || h <= 0 {
		return [4]bool{}
	}
	bandX := int(float64(w) * doorBandRatio)
	if bandX < 4 {
		bandX = 4
	}
	bandY := int(float64(h) * doorBandRatio)
	if bandY < 4 {
		bandY = 4
	}
	var doors [4]bool
	// up: top strip across full width
	doors[0] = bandYellowRatio(img, b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+bandY) > doorYellowRatio
	// right
	doors[1] = bandYellowRatio(img, b.Max.X-bandX, b.Min.Y, b.Max.X, b.Max.Y) > doorYellowRatio
	// down
	doors[2] = bandYellowRatio(img, b.Min.X, b.Max.Y-bandY, b.Max.X, b.Max.Y) > doorYellowRatio
	// left
	doors[3] = bandYellowRatio(img, b.Min.X, b.Min.Y, b.Min.X+bandX, b.Max.Y) > doorYellowRatio
	return doors
}

// LoadStarterTiles returns the three pre-placed entrance rooms.
// They are always shown face-up so Front == Back. The image is laid out
// left-to-right as: [0] Grand Staircase, [1] Foyer, [2] Entrance Hall.
func LoadStarterTiles() ([]*tile.RoomTile, error) {
	img, err := LoadImage(EntryGround)
	if err != nil {
		return nil, err
	}
	imgs := cropGrid(img, 3, 1)
	out := make([]*tile.RoomTile, 0, len(imgs))
	for i, im := range imgs {
		doors := detectDoors(im)
		// The Grand Staircase tile (index 0) has no yellow door brackets in
		// the artwork; the visible staircase implies a single door at the
		// south edge connecting to the Foyer above it in our world layout.
		if i == 0 && doors == ([4]bool{}) {
			doors[tile.SideDown] = true
		}
		out = append(out, &tile.RoomTile{
			ID:     1000 + i,
			Source: tile.SourceStarter,
			Front:  im,
			Back:   im,
			Doors:  doors,
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
		doors := detectDoors(frontTiles[i])
		// Skip blank tiles (some grid cells are pure background filler).
		if isBlank(frontTiles[i]) {
			continue
		}
		// Skip tiles where no door could be detected; they cannot be legally
		// connected to any other room and would soft-lock the deck.
		has := false
		for _, d := range doors {
			if d {
				has = true
				break
			}
		}
		if !has {
			continue
		}
		out = append(out, &tile.RoomTile{
			ID:     idBase + i,
			Source: src,
			Front:  frontTiles[i],
			Back:   backTiles[i],
			Doors:  doors,
		})
	}
	return out, nil
}

// isBlank reports whether the cropped tile image is essentially solid black,
// which happens when the source sheet has filler cells (e.g. r4 of 主地图.jpg).
func isBlank(img *ebiten.Image) bool {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w <= 0 || h <= 0 {
		return true
	}
	step := w / 16
	if step < 1 {
		step = 1
	}
	nonBlack := 0
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			if int(r>>8)+int(g>>8)+int(bl>>8) > 30 {
				nonBlack++
				if nonBlack > 4 {
					return false
				}
			}
		}
	}
	return true
}

// LoadBaseDeck loads the base game's 50 explorable rooms (10 columns x 5 rows).
func LoadBaseDeck() ([]*tile.RoomTile, error) {
	return loadDeckTiles(MainMap, MainMapBack, 10, 5, 0, tile.SourceBase)
}

// LoadExtensionDeck loads the DLC's 20 explorable rooms (10 columns x 2 rows).
func LoadExtensionDeck() ([]*tile.RoomTile, error) {
	return loadDeckTiles(ExtendMap, ExtendMapBack, 10, 2, 500, tile.SourceExtension)
}
