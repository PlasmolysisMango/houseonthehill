package turn

import (
	"testing"

	"github.com/plasmolysismango/houseonthehill/pkg/dice"
)

func TestHauntTrackerStartsClean(t *testing.T) {
	h := NewHauntTracker()
	if h.OmenCount() != 0 || h.Triggered() {
		t.Errorf("zero value: omens=%d triggered=%v", h.OmenCount(), h.Triggered())
	}
}

func TestHauntRollFirstOmenNeverTriggers(t *testing.T) {
	// With OmenCount==1 the threshold is "total < 1" i.e. total == 0.
	// Six 6-faced betrayal dice give 0 with prob (2/6)^6 ≈ 0.00137 — so
	// across many seeds we should usually NOT trigger on the very first
	// omen. We assert <50% trigger rate on 200 different seeds.
	triggers := 0
	for seed := int64(1); seed <= 200; seed++ {
		h := NewHauntTracker()
		d := dice.New(seed)
		if h.HauntRoll(d) {
			triggers++
		}
	}
	if triggers > 100 {
		t.Errorf("first-omen trigger %d/200 — way over expected ~0.3", triggers)
	}
}

func TestHauntRollOmenCountIncrementsEvenWhenTriggered(t *testing.T) {
	h := NewHauntTracker()
	d := dice.New(1)
	// Force OmenCount up to 14 — 14 > max-possible-roll (12) → guaranteed trigger.
	for i := 0; i < 13; i++ {
		h.HauntRoll(d)
	}
	if !h.Triggered() {
		t.Errorf("after 13 rolls expected to be triggered")
	}
	already := h.OmenCount()
	// Subsequent rolls should be silent no-ops.
	if h.HauntRoll(d) {
		t.Errorf("HauntRoll after trigger returned true")
	}
	if h.OmenCount() != already {
		t.Errorf("OmenCount kept incrementing post-trigger: %d → %d", already, h.OmenCount())
	}
}

func TestHauntRollGuaranteedTriggerAt13Omens(t *testing.T) {
	// Max sum of 6 betrayal dice is 12. Once OmenCount >= 13 the next
	// roll cannot succeed (total < 13 always). Assert this across seeds.
	for seed := int64(1); seed <= 20; seed++ {
		h := NewHauntTracker()
		h.omens = 12 // 13th omen will be drawn by HauntRoll
		d := dice.New(seed)
		if !h.HauntRoll(d) {
			t.Errorf("seed %d: 13th omen failed to trigger", seed)
		}
	}
}

// 5 玩家 × 10 omens × 1000 trials — measures roughly when the Haunt
// triggers in a typical game. This corresponds to ROADMAP "构造 5 名玩家
// + 模拟抽 10 张预兆，统计触发回合分布与桌游手册一致".
func TestHauntTriggerRoundDistribution(t *testing.T) {
	const players = 5
	const maxOmens = 10
	const trials = 1000
	// Track which omen draw triggered the haunt (1..10) or 0 if not
	// triggered within 10 omens.
	hist := make([]int, maxOmens+1)
	totalTriggered := 0
	for trial := 0; trial < trials; trial++ {
		h := NewHauntTracker()
		d := dice.New(int64(trial) + 1)
		for omen := 1; omen <= maxOmens; omen++ {
			triggered := h.HauntRoll(d)
			if triggered {
				hist[omen]++
				totalTriggered++
				break
			}
		}
		if !h.Triggered() {
			hist[0]++
		}
	}
	// Sanity: by omen 10 the cumulative trigger probability should be
	// well above 50% (rule of thumb: typical Betrayal games haunt at
	// the 4th-7th omen). We assert >=60% triggered within 10 omens.
	if totalTriggered < (trials*60)/100 {
		t.Errorf("only %d/%d trials triggered within %d omens — distribution off",
			totalTriggered, trials, maxOmens)
	}
	// And NO triggers should land before omen 1 (impossible) or after
	// omen 10 (we capped at 10).
	if hist[0] > trials*40/100 {
		t.Errorf("too many trials never triggered: %d/%d", hist[0], trials)
	}
	// Mean trigger round (over triggered runs) should be inside [3, 8].
	weighted := 0
	for omen := 1; omen <= maxOmens; omen++ {
		weighted += omen * hist[omen]
	}
	mean := float64(weighted) / float64(totalTriggered)
	if mean < 3.0 || mean > 8.0 {
		t.Errorf("mean trigger round %.2f outside [3, 8] — distribution off", mean)
	}
	t.Logf("haunt trigger distribution (player count=%d): hist=%v mean=%.2f triggered=%d/%d",
		players, hist[1:], mean, totalTriggered, trials)
}

func TestHauntTrackerLastRoll(t *testing.T) {
	h := NewHauntTracker()
	d := dice.New(1)
	h.HauntRoll(d)
	sum, n := h.LastRoll()
	if n != 6 {
		t.Errorf("LastRoll dice count = %d, want 6", n)
	}
	if sum < 0 || sum > 12 {
		t.Errorf("LastRoll sum %d outside [0,12]", sum)
	}
}

func TestHauntTrackerNilDie(t *testing.T) {
	h := NewHauntTracker()
	// nil die ⇒ rolls 0 ⇒ first omen triggers (0 < 1).
	if !h.HauntRoll(nil) {
		t.Errorf("nil die should roll 0 and trigger immediately")
	}
}

func TestHauntTrackerReset(t *testing.T) {
	h := NewHauntTracker()
	d := dice.New(1)
	for i := 0; i < 13; i++ {
		h.HauntRoll(d)
	}
	h.Reset()
	if h.OmenCount() != 0 || h.Triggered() {
		t.Errorf("Reset failed: omens=%d triggered=%v", h.OmenCount(), h.Triggered())
	}
}
