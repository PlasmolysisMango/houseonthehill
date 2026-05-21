// Package turn drives the per-round, per-player phase loop.
//
// An Engine owns the seating order (a slice of *player.Player), a cursor
// pointing at whose turn it currently is, and the current Phase within
// that player's turn. Dead players are skipped automatically; once every
// player is dead the engine enters a terminal "game over" state.
//
// Phase progression for one player is fixed:
//
//	start → move → action → end
//
// Calling NextPhase on the End phase rolls over to the next non-dead
// player's Start phase, incrementing Round each time the cursor wraps.
//
// Stage 3 of the roadmap will hang stepsLeft (= Speed at PhaseStart) and
// dice rolls off this engine; the API is shaped to accept those without
// breaking callers.
package turn

import (
	"github.com/plasmolysismango/houseonthehill/pkg/player"
)

// Phase enumerates the sub-states of a single player's turn.
type Phase int

const (
	PhaseStart  Phase = iota // bookkeeping: reset stepsLeft, flush buffs
	PhaseMove                // walking around the board
	PhaseAction              // draw cards / use items / resolve checks
	PhaseEnd                 // end-of-turn triggers, hand off to next
)

// String returns the lower-case label used in HUD/log output.
func (p Phase) String() string {
	switch p {
	case PhaseStart:
		return "start"
	case PhaseMove:
		return "move"
	case PhaseAction:
		return "action"
	case PhaseEnd:
		return "end"
	}
	return "unknown"
}

// Engine is the round/turn state machine. The zero value is unusable;
// construct via New.
type Engine struct {
	players   []*player.Player
	cur       int
	phase     Phase
	round     int // 1-based; bumps when the cursor wraps past the last player
	over      bool
	stepsLeft int // remaining move budget for the current player's turn
}

// New builds an engine seeded at the first non-dead player's start phase.
// Panics if players is empty (callers must guarantee at least one seat).
//
// If every supplied player is already dead the engine starts in the
// terminal "game over" state (Current() == nil, IsGameOver() == true).
func New(players []*player.Player) *Engine {
	if len(players) == 0 {
		panic("turn.New: at least one player required")
	}
	e := &Engine{players: players, cur: 0, phase: PhaseStart, round: 1}
	if e.allDead() {
		e.over = true
		return e
	}
	// Skip any leading dead players so Current() is meaningful.
	for e.players[e.cur].Dead {
		e.cur++
		if e.cur >= len(e.players) {
			// Cannot happen given allDead() check above, but stay safe.
			e.cur = 0
			break
		}
	}
	return e
}

// Players returns the seating slice. Read-only by convention.
func (e *Engine) Players() []*player.Player { return e.players }

// Current returns the player whose turn it currently is, or nil when the
// game is over.
func (e *Engine) Current() *player.Player {
	if e.over {
		return nil
	}
	return e.players[e.cur]
}

// Phase returns the current sub-phase of Current's turn.
func (e *Engine) Phase() Phase { return e.phase }

// Round is the 1-based round counter. It increments each time the cursor
// wraps from the last seat back to the first.
func (e *Engine) Round() int { return e.round }

// IsGameOver reports whether every seat is now Dead.
func (e *Engine) IsGameOver() bool { return e.over }

// StepsLeft is the remaining move budget for the current player's turn.
// 0 means either the budget was used up or BeginTurn has not yet been
// called for this turn.
func (e *Engine) StepsLeft() int { return e.stepsLeft }

// BeginTurn transitions the current player into PhaseMove with the given
// speed budget. Callers should compute speed from the current player's
// Speed stat just before calling so transient stat shifts during the
// previous turn are reflected. No-op once the game is over. Negative
// speeds are clamped to 0.
func (e *Engine) BeginTurn(speed int) {
	if e.over {
		return
	}
	if speed < 0 {
		speed = 0
	}
	e.phase = PhaseMove
	e.stepsLeft = speed
}

// SpendStep decrements the move budget by one. Returns false (and
// changes nothing) when there's no budget left. When the budget hits
// zero phase auto-advances to PhaseAction so the HUD shows the player
// has finished moving for this turn.
func (e *Engine) SpendStep() bool {
	if e.over || e.stepsLeft <= 0 {
		return false
	}
	e.stepsLeft--
	if e.stepsLeft == 0 {
		e.phase = PhaseAction
	}
	return true
}

// NextPhase advances within the current player's turn. Calling NextPhase
// on PhaseEnd rolls over to the next live player's PhaseStart and bumps
// Round when the cursor wraps. No-op once the game is over.
func (e *Engine) NextPhase() {
	if e.over {
		return
	}
	switch e.phase {
	case PhaseStart:
		e.phase = PhaseMove
	case PhaseMove:
		e.phase = PhaseAction
	case PhaseAction:
		e.phase = PhaseEnd
	case PhaseEnd:
		e.endTurn()
	}
}

// EndTurn short-circuits the remaining sub-phases of the current player
// and hands off to the next live seat. Equivalent to fast-forwarding
// NextPhase until PhaseEnd then once more.
func (e *Engine) EndTurn() {
	if e.over {
		return
	}
	e.endTurn()
}

// endTurn rotates the cursor to the next non-dead seat. Sets over=true
// when every seat is dead. Always clears stepsLeft so the next BeginTurn
// (or a missed BeginTurn) doesn't bleed budget across turns.
func (e *Engine) endTurn() {
	e.stepsLeft = 0
	if e.allDead() {
		e.over = true
		return
	}
	n := len(e.players)
	for i := 0; i < n; i++ {
		e.cur++
		if e.cur >= n {
			e.cur = 0
			e.round++
		}
		if !e.players[e.cur].Dead {
			e.phase = PhaseStart
			return
		}
	}
	// Walked the whole ring without finding a live seat → game over.
	e.over = true
}

func (e *Engine) allDead() bool {
	for _, p := range e.players {
		if p != nil && !p.Dead {
			return false
		}
	}
	return true
}
