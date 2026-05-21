package turn

import (
	"testing"

	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
	"github.com/plasmolysismango/houseonthehill/pkg/player"
)

// makePlayers builds n simple players with deterministic stat tracks so
// each test starts from a known shape (Idx=3, three -1 shifts → death).
func makePlayers(n int) []*player.Player {
	out := make([]*player.Player, n)
	for i := 0; i < n; i++ {
		c := &data.Character{
			ID:        i + 1,
			NameEN:    "P" + string(rune('A'+i)),
			ColorHex:  "#FFFFFF",
			Might:     data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
			Speed:     data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
			Sanity:    data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
			Knowledge: data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
		}
		out[i] = player.New(i+1, c, board.Cell{X: i, Y: 0})
	}
	return out
}

func TestNew_PanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("New([]) did not panic")
		}
	}()
	_ = New(nil)
}

func TestNew_StartsAtFirstPlayerStartPhase(t *testing.T) {
	ps := makePlayers(3)
	e := New(ps)
	if e.Current() != ps[0] {
		t.Errorf("Current = %v, want first player", e.Current())
	}
	if e.Phase() != PhaseStart {
		t.Errorf("Phase = %v, want PhaseStart", e.Phase())
	}
	if e.Round() != 1 {
		t.Errorf("Round = %d, want 1", e.Round())
	}
	if e.IsGameOver() {
		t.Errorf("IsGameOver = true on fresh engine")
	}
}

func TestNextPhase_CyclesThroughOnePlayer(t *testing.T) {
	ps := makePlayers(2)
	e := New(ps)

	want := []Phase{PhaseMove, PhaseAction, PhaseEnd}
	for i, w := range want {
		e.NextPhase()
		if e.Phase() != w {
			t.Errorf("step %d: phase = %v, want %v", i+1, e.Phase(), w)
		}
		if e.Current() != ps[0] {
			t.Errorf("step %d: switched player too early", i+1)
		}
	}
	// One more NextPhase rolls over to player 1's PhaseStart.
	e.NextPhase()
	if e.Current() != ps[1] {
		t.Errorf("after End: Current = %v, want ps[1]", e.Current())
	}
	if e.Phase() != PhaseStart {
		t.Errorf("after rollover: phase = %v, want PhaseStart", e.Phase())
	}
}

func TestEndTurn_ShortCircuits(t *testing.T) {
	ps := makePlayers(3)
	e := New(ps)
	e.NextPhase() // → Move
	e.EndTurn()
	if e.Current() != ps[1] {
		t.Errorf("EndTurn from Move: Current = %v, want ps[1]", e.Current())
	}
	if e.Phase() != PhaseStart {
		t.Errorf("EndTurn from Move: phase = %v, want PhaseStart", e.Phase())
	}
}

func TestRoundIncrementsOnWrap(t *testing.T) {
	ps := makePlayers(2)
	e := New(ps)
	if e.Round() != 1 {
		t.Fatalf("initial round = %d, want 1", e.Round())
	}
	e.EndTurn() // → ps[1] start, still round 1
	if e.Round() != 1 {
		t.Errorf("after first EndTurn round = %d, want 1", e.Round())
	}
	e.EndTurn() // wraps → ps[0] start, round 2
	if e.Round() != 2 {
		t.Errorf("after wrap round = %d, want 2", e.Round())
	}
	if e.Current() != ps[0] {
		t.Errorf("after wrap Current = %v, want ps[0]", e.Current())
	}
}

func TestEndTurn_SkipsDeadPlayer(t *testing.T) {
	ps := makePlayers(3)
	ps[1].Dead = true // middle seat is corpse
	e := New(ps)

	e.EndTurn() // ps[0] → should skip ps[1] → ps[2]
	if e.Current() != ps[2] {
		t.Errorf("Current after skip = %v, want ps[2]", e.Current())
	}
	e.EndTurn() // ps[2] → wraps back to ps[0] (skipping the dead ps[1])
	if e.Current() != ps[0] {
		t.Errorf("Current after wrap = %v, want ps[0]", e.Current())
	}
	if e.Round() != 2 {
		t.Errorf("Round after wrap = %d, want 2", e.Round())
	}
}

func TestEndTurn_AllDeadFromStartIsGameOver(t *testing.T) {
	ps := makePlayers(2)
	for _, p := range ps {
		p.Dead = true
	}
	e := New(ps)
	if !e.IsGameOver() {
		t.Errorf("IsGameOver = false on all-dead seats")
	}
	if e.Current() != nil {
		t.Errorf("Current on game over = %v, want nil", e.Current())
	}
	// NextPhase / EndTurn must be no-ops when over.
	e.NextPhase()
	e.EndTurn()
	if !e.IsGameOver() {
		t.Errorf("game over flag flipped off after no-op calls")
	}
}

func TestEndTurn_TransitionsToGameOverWhenLastDies(t *testing.T) {
	ps := makePlayers(3)
	e := New(ps)
	// Kill player 1 mid-game then advance: ps[0] → skip → ps[2] (alive).
	ps[1].Dead = true
	e.EndTurn()
	if e.Current() != ps[2] {
		t.Fatalf("setup: expected ps[2], got %v", e.Current())
	}
	// Now also kill ps[0] and ps[2]; next EndTurn should hit game over.
	ps[0].Dead = true
	ps[2].Dead = true
	e.EndTurn()
	if !e.IsGameOver() {
		t.Errorf("IsGameOver = false after every seat died")
	}
	if e.Current() != nil {
		t.Errorf("Current = %v, want nil", e.Current())
	}
}

func TestPhaseString(t *testing.T) {
	cases := map[Phase]string{
		PhaseStart: "start", PhaseMove: "move",
		PhaseAction: "action", PhaseEnd: "end",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Phase(%d).String() = %q, want %q", p, got, want)
		}
	}
}

func TestBeginTurnSetsBudgetAndPhase(t *testing.T) {
	ps := makePlayers(2)
	e := New(ps)
	if got := e.StepsLeft(); got != 0 {
		t.Errorf("fresh StepsLeft = %d, want 0", got)
	}
	e.BeginTurn(5)
	if e.Phase() != PhaseMove {
		t.Errorf("Phase = %v, want PhaseMove", e.Phase())
	}
	if e.StepsLeft() != 5 {
		t.Errorf("StepsLeft = %d, want 5", e.StepsLeft())
	}
	e.BeginTurn(-3)
	if e.StepsLeft() != 0 {
		t.Errorf("negative speed not clamped: StepsLeft = %d", e.StepsLeft())
	}
}

func TestSpendStepDrainsBudgetAndAdvancesPhase(t *testing.T) {
	ps := makePlayers(2)
	e := New(ps)
	e.BeginTurn(2)
	if !e.SpendStep() {
		t.Fatalf("SpendStep #1 returned false with budget left")
	}
	if e.Phase() != PhaseMove {
		t.Errorf("phase advanced too early: %v", e.Phase())
	}
	if e.StepsLeft() != 1 {
		t.Errorf("StepsLeft = %d, want 1", e.StepsLeft())
	}
	if !e.SpendStep() {
		t.Fatalf("SpendStep #2 returned false")
	}
	if e.Phase() != PhaseAction {
		t.Errorf("Phase = %v, want PhaseAction after budget drained", e.Phase())
	}
	if e.SpendStep() {
		t.Errorf("SpendStep on empty budget returned true")
	}
}

func TestEndTurnClearsStepsLeft(t *testing.T) {
	ps := makePlayers(3)
	e := New(ps)
	e.BeginTurn(4)
	e.SpendStep()
	e.EndTurn()
	if e.StepsLeft() != 0 {
		t.Errorf("StepsLeft after EndTurn = %d, want 0", e.StepsLeft())
	}
	if e.Phase() != PhaseStart {
		t.Errorf("Phase = %v, want PhaseStart at next seat", e.Phase())
	}
}

func TestBeginTurnNoOpAfterGameOver(t *testing.T) {
	ps := makePlayers(1)
	ps[0].Dead = true
	e := New(ps)
	if !e.IsGameOver() {
		t.Fatalf("want game over with single dead seat")
	}
	e.BeginTurn(7)
	if e.StepsLeft() != 0 {
		t.Errorf("BeginTurn after game over set StepsLeft = %d", e.StepsLeft())
	}
}
