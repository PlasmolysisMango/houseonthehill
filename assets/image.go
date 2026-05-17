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

// LoadStarterTiles returns the three pre-placed entrance rooms.
// They are always shown face-up so Front == Back.
func LoadStarterTiles() ([]*tile.RoomTile, error) {
	img, err := LoadImage(EntryGround)
	if err != nil {
		return nil, err
	}
	imgs := cropGrid(img, 3, 1)
	out := make([]*tile.RoomTile, 0, len(imgs))
	for i, im := range imgs {
		out = append(out, &tile.RoomTile{
			ID:     1000 + i,
			Source: tile.SourceStarter,
			Front:  im,
			Back:   im,
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
		out = append(out, &tile.RoomTile{
			ID:     idBase + i,
			Source: src,
			Front:  frontTiles[i],
			Back:   backTiles[i],
		})
	}
	return out, nil
}

// LoadBaseDeck loads the base game's 50 explorable rooms (10 columns x 5 rows).
func LoadBaseDeck() ([]*tile.RoomTile, error) {
	return loadDeckTiles(MainMap, MainMapBack, 10, 5, 0, tile.SourceBase)
}

// LoadExtensionDeck loads the DLC's 20 explorable rooms (10 columns x 2 rows).
func LoadExtensionDeck() ([]*tile.RoomTile, error) {
	return loadDeckTiles(ExtendMap, ExtendMapBack, 10, 2, 500, tile.SourceExtension)
}
