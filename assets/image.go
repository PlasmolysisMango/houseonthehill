package assets

import (
	"bytes"
	"embed"
	_ "embed"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

const (
	MainMap         = "主地图.jpg"
	MainMapBack     = "主地图-背面.jpg"
	EntryGround     = "入口大厅.png"
	ExtendMap       = "扩展地图.png"
	EntryGroundBack = "扩展地图-背面.jpg"
)

//go:embed image
var imageAssets embed.FS

func LoadImage(name string) (*ebiten.Image, error) {
	data, err := imageAssets.ReadFile("image/" + name)
	if err != nil {
		return nil, fmt.Errorf("load image %s failed: %w", name, err)
	}
	rawImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}
	return ebiten.NewImageFromImage(rawImage), nil
}

func cropRoomImages(baseImg image.Image, xCnt, yCnt int) {

}
