package assets

import (
	"bytes"
	"embed"
	"log"
	"strings"
	"sync"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// fontAssets embeds the Noto Sans CJK font file shipped with the game.
// We embed only one weight (Regular) to keep the wasm payload small.
//
//go:embed font/NotoSansCJK-Regular.ttc
var fontAssets embed.FS

const uiFontPath = "font/NotoSansCJK-Regular.ttc"

var (
	uiFontOnce sync.Once
	uiFontSrc  *text.GoTextFaceSource
)

// UIFontSource returns the shared Simplified-Chinese font source used by the
// HUD. The underlying .ttc is parsed once on the first call; subsequent calls
// are cheap. Returns nil if the font cannot be loaded.
func UIFontSource() *text.GoTextFaceSource {
	uiFontOnce.Do(func() {
		raw, err := fontAssets.ReadFile(uiFontPath)
		if err != nil {
			log.Printf("assets: read %s failed: %v", uiFontPath, err)
			return
		}
		sources, err := text.NewGoTextFaceSourcesFromCollection(bytes.NewReader(raw))
		if err != nil {
			log.Printf("assets: parse Noto Sans CJK collection failed: %v", err)
			return
		}
		// Prefer the SC face if present (family name like "Noto Sans CJK SC").
		for _, s := range sources {
			if s == nil {
				continue
			}
			if strings.Contains(s.Metadata().Family, "SC") {
				uiFontSrc = s
				return
			}
		}
		if len(sources) > 0 {
			uiFontSrc = sources[0]
		}
	})
	return uiFontSrc
}
