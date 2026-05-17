package utils

import (
	"github.com/hajimehoshi/ebiten/v2"
)

type Mouse struct {
	X int
	Y int
}

// Update mouse position, 在game的Update方法里defer调用
func (m *Mouse) Update() {
	m.X, m.Y = ebiten.CursorPosition()
}

func (m *Mouse) IsInside(x, y, w, h int) bool {
	return m.X >= x && m.X <= x+w && m.Y >= y && m.Y <= y+h
}

func (m *Mouse) LeftDrag() (int, int) {
	return m.Drag(ebiten.MouseButtonLeft)
}

func (m *Mouse) Drag(mouseButton ebiten.MouseButton) (int, int) {
	if ebiten.IsMouseButtonPressed(mouseButton) {
		x, y := ebiten.CursorPosition()
		return x - m.X, y - m.Y
	}
	return 0, 0
}

func (m *Mouse) Wheel() (int, int) {
	x, y := ebiten.Wheel()
	return int(x), int(y)
}

func (m *Mouse) IsClick(mouseButton ebiten.MouseButton) bool {
	return ebiten.IsMouseButtonPressed(mouseButton)
}
