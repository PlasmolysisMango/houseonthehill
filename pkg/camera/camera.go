package camera

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/plasmolysismango/houseonthehill/pkg/utils"
)

// Camera maps world coordinates to screen coordinates.
// X, Y is the world position currently centred on the screen.
type Camera struct {
	X, Y    float64
	Scale   float64
	ScreenW int
	ScreenH int
}

func New(screenW, screenH int) *Camera {
	return &Camera{Scale: 1.0, ScreenW: screenW, ScreenH: screenH}
}

// Resize updates the screen dimensions used for centring.
func (c *Camera) Resize(w, h int) {
	c.ScreenW, c.ScreenH = w, h
}

// Update consumes mouse drag, wheel zoom and middle-click reset.
func (c *Camera) Update(m *utils.Mouse) {
	dx, dy := m.LeftDrag()
	if dx != 0 || dy != 0 {
		c.X -= float64(dx) / c.Scale
		c.Y -= float64(dy) / c.Scale
	}
	_, wy := m.Wheel()
	if wy != 0 {
		factor := 1.0 + float64(wy)*0.1
		c.Scale *= factor
		if c.Scale < 0.1 {
			c.Scale = 0.1
		}
		if c.Scale > 5 {
			c.Scale = 5
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		c.X, c.Y = 0, 0
		c.Scale = 1
	}
}

// GeoM returns the world->screen transform.
func (c *Camera) GeoM() ebiten.GeoM {
	var g ebiten.GeoM
	g.Translate(-c.X, -c.Y)
	g.Scale(c.Scale, c.Scale)
	g.Translate(float64(c.ScreenW)/2, float64(c.ScreenH)/2)
	return g
}

// ScreenToWorld converts a pixel position to world coordinates.
func (c *Camera) ScreenToWorld(sx, sy int) (float64, float64) {
	wx := (float64(sx)-float64(c.ScreenW)/2)/c.Scale + c.X
	wy := (float64(sy)-float64(c.ScreenH)/2)/c.Scale + c.Y
	return wx, wy
}

// CenterOn centres the camera on a world position.
func (c *Camera) CenterOn(wx, wy float64) { c.X, c.Y = wx, wy }
