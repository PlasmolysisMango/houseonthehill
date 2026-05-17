package board

// Cell represents a discrete grid coordinate on the board.
type Cell struct {
	X, Y int
}

// Add returns a new Cell offset by (dx, dy).
func (c Cell) Add(dx, dy int) Cell { return Cell{c.X + dx, c.Y + dy} }

// Directions lists the four cardinal neighbour offsets: up, right, down, left.
var Directions = []Cell{
	{0, -1},
	{1, 0},
	{0, 1},
	{-1, 0},
}

// Manhattan returns the Manhattan distance between two cells.
func Manhattan(a, b Cell) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// IsAdjacent reports whether b is one of the four neighbours of a.
func IsAdjacent(a, b Cell) bool { return Manhattan(a, b) == 1 }

// Board is a sparse grid that stores values keyed by Cell.
type Board[T any] struct {
	cells    map[Cell]T
	TileSize int
}

// New creates an empty board where each tile is tileSize pixels wide.
func New[T any](tileSize int) *Board[T] {
	return &Board[T]{cells: make(map[Cell]T), TileSize: tileSize}
}

func (b *Board[T]) Place(c Cell, v T) { b.cells[c] = v }

func (b *Board[T]) At(c Cell) (T, bool) {
	v, ok := b.cells[c]
	return v, ok
}

func (b *Board[T]) Has(c Cell) bool {
	_, ok := b.cells[c]
	return ok
}

func (b *Board[T]) All() map[Cell]T { return b.cells }

func (b *Board[T]) Neighbors(c Cell) []Cell {
	out := make([]Cell, 0, 4)
	for _, d := range Directions {
		out = append(out, Cell{c.X + d.X, c.Y + d.Y})
	}
	return out
}

func (b *Board[T]) EmptyNeighbors(c Cell) []Cell {
	out := make([]Cell, 0, 4)
	for _, n := range b.Neighbors(c) {
		if !b.Has(n) {
			out = append(out, n)
		}
	}
	return out
}
