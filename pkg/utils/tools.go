package utils

import "github.com/hajimehoshi/ebiten/v2"

func ScaleTo(src, dst *ebiten.Image) float64 {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	dstW := dst.Bounds().Dx()
	dstH := dst.Bounds().Dy()
	minScale := min(float64(srcW)/float64(dstW), float64(srcH)/float64(dstH))
	return minScale
}
