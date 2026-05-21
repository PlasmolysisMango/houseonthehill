package scene

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/plasmolysismango/houseonthehill/pkg/ui"
)

// MenuScene is the title screen presenting Start / Toggle Extension / Exit.
type MenuScene struct {
	screenW, screenH int
	useExtension     bool
	exitRequested    bool
	startBtn         image.Rectangle
	toggleBtn        image.Rectangle
	exitBtn          image.Rectangle
}

func NewMenuScene(screenW, screenH int) *MenuScene {
	return &MenuScene{
		screenW: screenW,
		screenH: screenH,
	}
}

func (s *MenuScene) layoutButtons() {
	const bw, bh = 360, 60
	cx := s.screenW / 2
	top := s.screenH/2 - 40
	s.startBtn = image.Rect(cx-bw/2, top, cx+bw/2, top+bh)
	s.toggleBtn = image.Rect(cx-bw/2, top+bh+20, cx+bw/2, top+bh+20+bh)
	s.exitBtn = image.Rect(cx-bw/2, top+(bh+20)*2, cx+bw/2, top+(bh+20)*2+bh)
}

func (s *MenuScene) Update() (Scene, error) {
	if s.exitRequested {
		return nil, ebiten.Termination
	}
	s.layoutButtons()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return nil, ebiten.Termination
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		p := image.Point{X: mx, Y: my}
		switch {
		case p.In(s.startBtn):
			return NewSetupScene(s.useExtension, s.screenW, s.screenH), nil
		case p.In(s.toggleBtn):
			s.useExtension = !s.useExtension
		case p.In(s.exitBtn):
			s.exitRequested = true
		}
	}
	return s, nil
}

func (s *MenuScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 26, G: 22, B: 28, A: 255})
	s.layoutButtons()

	mx, my := ebiten.CursorPosition()
	mp := image.Point{X: mx, Y: my}

	// title
	title := "House on the Hill"
	subtitle := "use mouse: drag to pan, wheel to zoom, middle-click resets"
	titleX := s.screenW/2 - len(title)*ui.GlyphWidth/2
	subtitleX := s.screenW/2 - len(subtitle)*ui.GlyphWidth/2
	ui.DrawAt(screen, title, titleX, s.screenH/2-160)
	ui.DrawAt(screen, subtitle, subtitleX, s.screenH/2-130)

	drawButton(screen, s.startBtn, "Start Game", mp.In(s.startBtn))
	toggleLabel := "Extension Map: OFF"
	if s.useExtension {
		toggleLabel = "Extension Map: ON"
	}
	drawButton(screen, s.toggleBtn, toggleLabel, mp.In(s.toggleBtn))
	drawButton(screen, s.exitBtn, "Exit", mp.In(s.exitBtn))

	// footer
	footer := fmt.Sprintf("Base deck: 50 rooms  Extension: +20 rooms (%s)", boolOnOff(s.useExtension))
	ui.DrawAt(screen, footer, s.screenW/2-len(footer)*ui.GlyphWidth/2, s.screenH-40)
}

func boolOnOff(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}

func drawButton(dst *ebiten.Image, r image.Rectangle, label string, hover bool) {
	bg := color.RGBA{R: 60, G: 56, B: 70, A: 255}
	if hover {
		bg = color.RGBA{R: 90, G: 78, B: 110, A: 255}
	}
	x, y, w, h := r.Min.X, r.Min.Y, r.Dx(), r.Dy()
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), bg, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{R: 220, G: 200, B: 180, A: 255}, false)
	tx := x + w/2 - len(label)*ui.GlyphWidth/2
	ty := y + h/2 - 8
	ui.DrawAt(dst, label, tx, ty)
}
