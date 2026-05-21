package dice

import (
	"math"
	"testing"
)

// TestRollMeanFourDice reproduces the roadmap stage 3 acceptance test:
// 100k rolls of 4 dice should land within 4 ± 0.05 of the expected mean.
func TestRollMeanFourDice(t *testing.T) {
	d := New(42)
	const trials = 100_000
	var sum int
	for i := 0; i < trials; i++ {
		sum += d.Roll(4)
	}
	mean := float64(sum) / float64(trials)
	if math.Abs(mean-4.0) > 0.05 {
		t.Fatalf("4-dice mean %.4f outside [3.95, 4.05] over %d trials", mean, trials)
	}
}

// TestVarianceFourDice asserts dispersion matches the analytic value.
// Var(one die) = E[X^2] - E[X]^2 = (0+0+1+1+4+4)/6 - 1 = 10/6 - 1 = 0.6667.
// Var(4 dice) = 4 * 0.6667 ≈ 2.6667.
func TestVarianceFourDice(t *testing.T) {
	d := New(7)
	const trials = 100_000
	xs := make([]float64, trials)
	var s float64
	for i := 0; i < trials; i++ {
		v := float64(d.Roll(4))
		xs[i] = v
		s += v
	}
	mean := s / float64(trials)
	var ss float64
	for _, v := range xs {
		ss += (v - mean) * (v - mean)
	}
	variance := ss / float64(trials)
	const want = 2.6667
	if math.Abs(variance-want) > 0.06 {
		t.Fatalf("variance %.4f outside [%.4f, %.4f]", variance, want-0.06, want+0.06)
	}
}

// TestRollDetailLengthAndRange ensures every face is in {0,1,2}.
func TestRollDetailLengthAndRange(t *testing.T) {
	d := New(1)
	for n := 1; n <= 8; n++ {
		out := d.RollDetail(n)
		if len(out) != n {
			t.Fatalf("RollDetail(%d) length %d, want %d", n, len(out), n)
		}
		for _, v := range out {
			if v < 0 || v > 2 {
				t.Fatalf("face %d out of range [0,2]", v)
			}
		}
	}
}

// TestRollNonPositive guards the n<=0 fast path.
func TestRollNonPositive(t *testing.T) {
	d := New(1)
	if got := d.Roll(0); got != 0 {
		t.Fatalf("Roll(0) = %d, want 0", got)
	}
	if got := d.Roll(-3); got != 0 {
		t.Fatalf("Roll(-3) = %d, want 0", got)
	}
	if got := d.RollDetail(0); got != nil {
		t.Fatalf("RollDetail(0) = %v, want nil", got)
	}
	if got := d.RollDetail(-1); got != nil {
		t.Fatalf("RollDetail(-1) = %v, want nil", got)
	}
}

// TestSumOf sanity-checks the helper against RollDetail.
func TestSumOf(t *testing.T) {
	d := New(99)
	for n := 1; n <= 6; n++ {
		faces := d.RollDetail(n)
		if got, want := SumOf(faces), naiveSum(faces); got != want {
			t.Fatalf("SumOf(%v) = %d, want %d", faces, got, want)
		}
	}
	if SumOf(nil) != 0 {
		t.Fatalf("SumOf(nil) != 0")
	}
}

func naiveSum(xs []int) int {
	s := 0
	for _, v := range xs {
		s += v
	}
	return s
}

// TestSeededReproducibility documents that same seed → same sequence.
func TestSeededReproducibility(t *testing.T) {
	a := New(1234)
	b := New(1234)
	for i := 0; i < 100; i++ {
		if x, y := a.Roll(4), b.Roll(4); x != y {
			t.Fatalf("step %d diverged: %d vs %d", i, x, y)
		}
	}
}
