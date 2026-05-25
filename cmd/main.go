package main

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/plasmolysismango/houseonthehill/pkg/scene"
)

// Default internal logical resolution. Window size matches at startup
// but can be resized freely (Ebiten scales the framebuffer to fit).
const (
	defaultScreenW = 1920
	defaultScreenH = 1080
)

// Game is the top-level Ebiten game; it delegates to the current Scene.
type Game struct {
	current  scene.Scene
	w, h     int // internal logical resolution (returned from Layout)
}

func (g *Game) Update() error {
	next, err := g.current.Update()
	if err != nil {
		return err
	}
	if next == nil {
		return ebiten.Termination
	}
	g.current = next
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.current != nil {
		g.current.Draw(screen)
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return g.w, g.h
}

// resolveResolution picks the runtime resolution in this priority order:
//	1. -w / -h command-line flags
//	2. HOTH_W / HOTH_H environment variables
//	3. defaults (1920x1080)
//
// Values <= 0 are rejected and fall back to defaults so a typo in env
// vars can't produce an unrenderable window. Min is clamped to 320x240
// to keep the HUD readable on any reasonable display.
func resolveResolution() (int, int) {
	w := flag.Int("w", 0, "window width  (default 1920, env HOTH_W)")
	h := flag.Int("h", 0, "window height (default 1080, env HOTH_H)")
	flag.Parse()

	pick := func(flagVal int, envKey string, def int) int {
		if flagVal > 0 {
			return flagVal
		}
		if s := os.Getenv(envKey); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				return v
			}
		}
		return def
	}

	sw := pick(*w, "HOTH_W", defaultScreenW)
	sh := pick(*h, "HOTH_H", defaultScreenH)
	if sw < 320 {
		sw = 320
	}
	if sh < 240 {
		sh = 240
	}
	return sw, sh
}

func main() {
	sw, sh := resolveResolution()

	ebiten.SetWindowSize(sw, sh)
	ebiten.SetWindowTitle("山上之屋")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	g := &Game{
		current: scene.NewMenuScene(sw, sh),
		w:       sw,
		h:       sh,
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
