// Package ui provides shared text-rendering helpers for the game HUD.
//
// Text is rendered with the Noto Sans CJK (SC) TrueType font embedded under
// assets/font, loaded via Ebiten's native text/v2 API. No third-party font
// packages are required.
//
// DrawAt / DrawColorAt mirror the ebitenutil.DebugPrintAt signature
// (x, y = top-left corner) so existing layout arithmetic is unchanged.
package ui

import (
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"

	"github.com/plasmolysismango/houseonthehill/assets"
)

// FontSize is the pixel size used for all HUD text. Chosen so that an ASCII
// glyph advance is close to GlyphWidth (preserving legacy *6 centring
// arithmetic in menu/game scenes).
const FontSize = 12

// GlyphWidth is the nominal advance width of a single ASCII character,
// retained as a constant so existing layout code keeps working.
const GlyphWidth = 6

var (
	faceOnce sync.Once
	uiFace   *text.GoTextFace
)

// face returns the shared GoTextFace, built lazily on first use.
func face() *text.GoTextFace {
	faceOnce.Do(func() {
		src := assets.UIFontSource()
		if src == nil {
			// Source failed to load; downstream Draw calls will be no-ops.
			return
		}
		uiFace = &text.GoTextFace{
			Source:   src,
			Size:     FontSize,
			Language: language.SimplifiedChinese,
		}
	})
	return uiFace
}

// DrawAt draws s in white with its top-left corner at (x, y).
// It is a drop-in replacement for ebitenutil.DebugPrintAt.
func DrawAt(dst *ebiten.Image, s string, x, y int) {
	DrawColorAt(dst, s, x, y, color.White)
}

// DrawColorAt draws s in the given colour with its top-left corner at (x, y).
func DrawColorAt(dst *ebiten.Image, s string, x, y int, clr color.Color) {
	f := face()
	if f == nil {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(clr)
	// AlignStart on both axes → (x,y) is top-left, matching DebugPrintAt.
	op.PrimaryAlign = text.AlignStart
	op.SecondaryAlign = text.AlignStart
	text.Draw(dst, s, f, op)
}

// LineHeight returns the pixel height of one line of UI text.
func LineHeight() int {
	f := face()
	if f == nil {
		return FontSize
	}
	m := f.Metrics()
	return int(math.Ceil(m.HAscent + m.HDescent + m.HLineGap))
}
