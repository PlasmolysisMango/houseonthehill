package utils

type Point struct {
	X, Y int
}

func (p Point) Position() (int, int) {
	return p.X, p.Y
}
