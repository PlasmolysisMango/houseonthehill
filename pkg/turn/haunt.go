package turn

import "github.com/plasmolysismango/houseonthehill/pkg/dice"

// HauntTracker counts the omens drawn so far and resolves the Haunt
// Roll triggered by every new omen draw. The roll is "roll 6 dice; if
// total < OmenCount then the haunt begins". The first failed roll wins
// — once Triggered() is true subsequent omens are no-ops so the same
// game can't trigger twice.
//
// HauntTracker is a small value-only state machine; it does NOT know
// about the omen-table → scenario lookup. Callers (game scene) supply
// that mapping after Triggered() flips to true.
type HauntTracker struct {
	omens     int
	triggered bool
	lastTotal int // most recent roll total, for HUD display
	lastRolls int // most recent roll's dice count
}

// NewHauntTracker returns a fresh tracker (zero omens, not triggered).
func NewHauntTracker() *HauntTracker { return &HauntTracker{} }

// OmenCount is the number of omens drawn this game (== current Haunt
// Roll target threshold).
func (h *HauntTracker) OmenCount() int { return h.omens }

// Triggered reports whether the Haunt has already begun.
func (h *HauntTracker) Triggered() bool { return h.triggered }

// LastRoll returns the (totalSum, diceCount) of the most recent
// HauntRoll call. Useful for HUD / event log.
func (h *HauntTracker) LastRoll() (sum, n int) { return h.lastTotal, h.lastRolls }

// Reset clears the tracker; mainly for tests.
func (h *HauntTracker) Reset() {
	h.omens = 0
	h.triggered = false
	h.lastTotal = 0
	h.lastRolls = 0
}

// HauntRoll registers a new omen draw and rolls 6 Betrayal dice.
// Returns true when the Haunt is triggered by this roll. Subsequent
// calls after the first triggering roll are silent no-ops so the
// caller doesn't need its own guard. Pass a *dice.Die seeded by the
// scene's RNG for reproducible test runs.
//
// Rule: total of 6 dice < OmenCount (after this draw) → trigger.
// (Exact equality does NOT trigger — matches the rulebook.)
func (h *HauntTracker) HauntRoll(d *dice.Die) bool {
	if h.triggered {
		return false
	}
	h.omens++
	const dice6 = 6
	total := 0
	if d != nil {
		total = d.Roll(dice6)
	}
	h.lastTotal = total
	h.lastRolls = dice6
	if total < h.omens {
		h.triggered = true
		return true
	}
	return false
}
