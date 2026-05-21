// Package dice models the Betrayal-style six-sided die whose faces are
// labelled 0,0,1,1,2,2 — i.e. each face contributes 0/1/2 with equal
// probability. The expected value of one die is 1; rolling N dice has
// mean N and variance ~N*0.667.
//
// Construct a Die with a seed for deterministic rolls (tests / replay)
// and call Roll(n) for the summed total or RollDetail(n) for per-face
// results suitable for UI animations.
package dice

import "math/rand"

// Faces are the six labelled sides of one Betrayal die.
var Faces = [6]int{0, 0, 1, 1, 2, 2}

// Die is a seeded roller. Zero value is unusable; construct via New.
type Die struct {
	rng *rand.Rand
}

// New returns a Die seeded with the given value. Pass time.Now().UnixNano()
// for live games, a fixed seed in tests.
func New(seed int64) *Die {
	return &Die{rng: rand.New(rand.NewSource(seed))}
}

// Roll throws n dice and returns the summed total. n<=0 returns 0 so
// callers can pass StatValue() directly without bounds-checking.
func (d *Die) Roll(n int) int {
	if n <= 0 {
		return 0
	}
	sum := 0
	for i := 0; i < n; i++ {
		sum += Faces[d.rng.Intn(6)]
	}
	return sum
}

// RollDetail rolls n dice and returns each face value. Returns nil for
// n<=0. Useful for UI animations that need to display individual dice.
func (d *Die) RollDetail(n int) []int {
	if n <= 0 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		out[i] = Faces[d.rng.Intn(6)]
	}
	return out
}

// SumOf returns the sum of a precomputed face slice. Trivial helper but
// keeps callers from importing two packages just to add ints.
func SumOf(faces []int) int {
	s := 0
	for _, v := range faces {
		s += v
	}
	return s
}
