package cards

import (
	"testing"

	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
	"github.com/plasmolysismango/houseonthehill/pkg/player"
)

func mkPlayer(id int, name string) *player.Player {
	return player.New(id, &data.Character{
		ID:        id,
		NameEN:    name,
		ColorHex:  "#FFFFFF",
		Might:     data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
		Speed:     data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
		Sanity:    data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
		Knowledge: data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
	}, board.Cell{})
}

func sampleEvents() []*Card {
	return []*Card{
		{ID: 1, NameEN: "Tome", Kind: KindEvent, Text: "+1 knw",
			Effects: []Effect{{Kind: EffectStatShift, Target: TargetSelf, Stat: StatKnowledge, Delta: 1}}},
		{ID: 2, NameEN: "Stumble", Kind: KindEvent, Text: "-1 spd",
			Effects: []Effect{{Kind: EffectStatShift, Target: TargetSelf, Stat: StatSpeed, Delta: -1}}},
		{ID: 3, NameEN: "Whisper", Kind: KindEvent, Text: "all -1 san",
			Effects: []Effect{{Kind: EffectStatShift, Target: TargetAll, Stat: StatSanity, Delta: -1}}},
		{ID: 4, NameEN: "Maze", Kind: KindEvent, Text: "manual",
			Effects: []Effect{{Kind: EffectManual, Note: "lose your way"}}},
	}
}

func TestNewDeckShuffleAndKind(t *testing.T) {
	d := NewDeck(sampleEvents(), 1)
	if d.Kind() != KindEvent {
		t.Errorf("Kind = %q, want event", d.Kind())
	}
	if d.Remaining() != 4 {
		t.Errorf("Remaining = %d, want 4", d.Remaining())
	}
}

func TestDrawAndDiscardCycle(t *testing.T) {
	d := NewDeck(sampleEvents(), 7)
	seen := map[int]bool{}
	for i := 0; i < 4; i++ {
		c := d.Draw()
		if c == nil {
			t.Fatalf("Draw %d returned nil", i)
		}
		seen[c.ID] = true
		d.Discard(c)
	}
	if len(seen) != 4 {
		t.Errorf("only saw %d unique cards, want 4", len(seen))
	}
	if d.Remaining() != 0 || d.DiscardSize() != 4 {
		t.Errorf("after draining: draw=%d discard=%d", d.Remaining(), d.DiscardSize())
	}
	// Next draw should reshuffle the discard pile back in.
	if got := d.Draw(); got == nil {
		t.Errorf("Draw after reshuffle returned nil")
	}
	if d.Remaining()+d.DiscardSize() != 3 {
		t.Errorf("after reshuffle+draw: total = %d, want 3",
			d.Remaining()+d.DiscardSize())
	}
}

func TestDrawEmptyReturnsNil(t *testing.T) {
	d := NewDeck(nil, 1)
	if d.Draw() != nil {
		t.Errorf("empty deck Draw should return nil")
	}
}

func TestApplyStatShiftSelf(t *testing.T) {
	p := mkPlayer(1, "A")
	c := sampleEvents()[0] // +1 knowledge
	logs := Apply(c, p, []*player.Player{p})
	if got := p.StatValue(player.Knowledge); got != 4 {
		t.Errorf("Knowledge = %d, want 4 (3+1)", got)
	}
	if len(logs) != 1 {
		t.Fatalf("logs = %v, want 1 entry", logs)
	}
}

func TestApplyStatShiftAllTable(t *testing.T) {
	a := mkPlayer(1, "A")
	b := mkPlayer(2, "B")
	c := mkPlayer(3, "C")
	table := []*player.Player{a, b, c}
	card := sampleEvents()[2] // all -1 sanity
	logs := Apply(card, a, table)
	for _, p := range table {
		if got := p.StatValue(player.Sanity); got != 2 {
			t.Errorf("%s sanity = %d, want 2", labelOf(p), got)
		}
	}
	if len(logs) != 1 {
		t.Errorf("logs len = %d, want 1 (one effect, multi-target rolled into one line)", len(logs))
	}
}

func TestApplyManualReturnsNote(t *testing.T) {
	p := mkPlayer(1, "A")
	card := sampleEvents()[3]
	logs := Apply(card, p, []*player.Player{p})
	if len(logs) != 1 {
		t.Fatalf("logs = %v", logs)
	}
	if logs[0] != "(manual) lose your way" {
		t.Errorf("manual log = %q", logs[0])
	}
}

func TestApplyNilCard(t *testing.T) {
	if got := Apply(nil, nil, nil); got != nil {
		t.Errorf("Apply(nil, ...) = %v, want nil", got)
	}
}

func TestParseStatRoundTrip(t *testing.T) {
	cases := map[Stat]player.StatKind{
		StatMight: player.Might, StatSpeed: player.Speed,
		StatSanity: player.Sanity, StatKnowledge: player.Knowledge,
	}
	for in, want := range cases {
		got, ok := parseStat(in)
		if !ok || got != want {
			t.Errorf("parseStat(%q) = (%v, %v), want (%v, true)", in, got, ok, want)
		}
	}
	if _, ok := parseStat("nonsense"); ok {
		t.Errorf("parseStat unknown should fail")
	}
}

func TestIsHeldCard(t *testing.T) {
	if IsHeldCard(nil) {
		t.Error("nil card should not be held")
	}
	ev := &Card{ID: 1, Kind: KindEvent}
	if IsHeldCard(ev) {
		t.Error("event should not be held")
	}
	it := &Card{ID: 2, Kind: KindItem}
	if !IsHeldCard(it) {
		t.Error("item should be held")
	}
	om := &Card{ID: 3, Kind: KindOmen}
	if !IsHeldCard(om) {
		t.Error("omen should be held")
	}
}

func TestPerTurnEffects(t *testing.T) {
	c := &Card{ID: 1, Kind: KindOmen, Effects: []Effect{
		{Kind: EffectPerTurn, Target: TargetSelf, Stat: StatSanity, Delta: -1},
		{Kind: EffectManual, Note: "held"},
	}}
	pt := PerTurnEffects(c)
	if len(pt) != 1 {
		t.Fatalf("PerTurnEffects got %d, want 1", len(pt))
	}
	if pt[0].Kind != EffectStatShift {
		t.Errorf("converted kind = %q, want stat_shift", pt[0].Kind)
	}
	if pt[0].Delta != -1 {
		t.Errorf("delta = %d, want -1", pt[0].Delta)
	}

	none := PerTurnEffects(&Card{ID: 2, Kind: KindItem})
	if len(none) != 0 {
		t.Errorf("card without per_turn returned %d effects", len(none))
	}
	if PerTurnEffects(nil) != nil {
		t.Error("nil card should return nil")
	}
}

func TestPerTurnApply(t *testing.T) {
	p := mkPlayer(1, "A")
	c := &Card{ID: 1, Kind: KindOmen, Effects: []Effect{
		{Kind: EffectPerTurn, Target: TargetSelf, Stat: StatSanity, Delta: -1},
	}}
	pt := PerTurnEffects(c)
	before := p.StatValue(player.Sanity)
	for _, eff := range pt {
		Apply(&Card{Effects: []Effect{eff}}, p, []*player.Player{p})
	}
	after := p.StatValue(player.Sanity)
	if after != before-1 {
		t.Errorf("sanity %d -> %d, want %d", before, after, before-1)
	}
}
