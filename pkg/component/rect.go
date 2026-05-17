package component

func NewRect(x, y, w, h int) *Rect {
	return &Rect{
		X: x,
		Y: y,
		W: w,
		H: h,
	}
}

type Rect struct {
	X, Y, W, H int
	extend     Extend
}

func (r *Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func (r *Rect) Extend() Extend {
	return Extend{x: r.X, y: r.Y, w: r.W, h: r.H}
}

type Extend struct {
	x, y, w, h int
}

// Up return the up extend position of a rect
func (e *Extend) Up() (int, int) {
	return e.x, e.y - e.h
}

// Down return the down extend position of a rect
func (e *Extend) Down() (int, int) {
	return e.x, e.y + e.h
}

// Left return the left extend position of a rect
func (e *Extend) Left() (int, int) {
	return e.x - e.w, e.y
}

// Right return the right extend position of a rect
func (e *Extend) Right() (int, int) {
	return e.x + e.w, e.y
}

func (e *Extend) UpLeft() (int, int) {
	return e.x, e.y - e.h
}

func (e *Extend) UpMiddle() (int, int) {
	return e.x + e.w/3, e.y - e.h
}

func (e *Extend) UpRight() (int, int) {
	return e.x + e.w*2/3, e.y - e.h
}

func (e *Extend) DownLeft() (int, int) {
	return e.x, e.y + e.h
}

func (e *Extend) DownMiddle() (int, int) {
	return e.x + e.w/3, e.y + e.h
}

func (e *Extend) DownRight() (int, int) {
	return e.x + e.w*2/3, e.y + e.h
}
