// Package cards models the three Betrayal card decks: Event, Item and
// Omen. Each card has a localized text and zero or more Effects which
// can be applied to a player at draw time. Complex / situational cards
// declare Effect{Kind: EffectManual} and the runtime simply shows the
// text — leaving resolution to the players, never blocking the flow.
//
// Construct decks via NewDeck(cards, seed). Decks are mutable: Draw
// removes the top card, Discard sends one to the discard pile, and
// when the draw pile is exhausted the discard pile is reshuffled in.
package cards

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/plasmolysismango/houseonthehill/pkg/player"
)

// Kind is the deck a card belongs to.
type Kind string

const (
	KindEvent Kind = "event"
	KindItem  Kind = "item"
	KindOmen  Kind = "omen"
)

// EffectKind enumerates the small set of mechanical effects a card may
// declare. Anything more complex is encoded as EffectManual + Note so
// players resolve it themselves from the printed text.
type EffectKind string

const (
	EffectStatShift EffectKind = "stat_shift"
	EffectManual    EffectKind = "manual"
	EffectPerTurn   EffectKind = "per_turn" // stat_shift executed once per turn while card is held
	// Future: EffectDrawCard / EffectMove / EffectRequireRoll / EffectSpawnToken.
)

// Target picks who an Effect applies to. "self" = drawing player.
type Target string

const (
	TargetSelf Target = "self"
	TargetAll  Target = "all"
)

// Stat is the localized stat string used in cards.yaml. It maps onto
// player.StatKind via parseStat.
type Stat string

const (
	StatMight     Stat = "might"
	StatSpeed     Stat = "speed"
	StatSanity    Stat = "sanity"
	StatKnowledge Stat = "knowledge"
)

// Effect is one resolvable action declared by a card.
type Effect struct {
	Kind   EffectKind `yaml:"kind"`
	Target Target     `yaml:"target,omitempty"`
	Stat   Stat       `yaml:"stat,omitempty"`
	Delta  int        `yaml:"delta,omitempty"`
	Note   string     `yaml:"note,omitempty"`
}

// Card is one face from any of the three decks.
type Card struct {
	ID      int      `yaml:"id"`
	NameCN  string   `yaml:"name_cn"`
	NameEN  string   `yaml:"name_en"`
	Kind    Kind     `yaml:"kind"`
	Text    string   `yaml:"text"`
	Effects []Effect `yaml:"effects,omitempty"`
}

// Label returns the bilingual title used in HUD lines / log entries.
func (c *Card) Label() string {
	if c == nil {
		return "(no card)"
	}
	if c.NameCN != "" && c.NameEN != "" {
		return fmt.Sprintf("%s / %s", c.NameCN, c.NameEN)
	}
	if c.NameCN != "" {
		return c.NameCN
	}
	return c.NameEN
}

// IsHeldCard reports whether the card should stay with the player
// after being drawn (go into inventory) rather than be immediately
// discarded. Items and omens are always kept; events are discarded.
func IsHeldCard(c *Card) bool {
	if c == nil {
		return false
	}
	return c.Kind == KindItem || c.Kind == KindOmen
}

// PerTurnEffects extracts effects that should fire once per turn while
// the card is held (EffectPerTurn). Each is converted to a stat_shift
// Effect so callers can pass it straight to applyOne.
func PerTurnEffects(c *Card) []Effect {
	if c == nil {
		return nil
	}
	var out []Effect
	for _, e := range c.Effects {
		if e.Kind == EffectPerTurn {
			out = append(out, Effect{
				Kind:   EffectStatShift,
				Target: e.Target,
				Stat:   e.Stat,
				Delta:  e.Delta,
			})
		}
	}
	return out
}

// EquipBonus returns the total static combat/roll bonus this card
// provides for stat. Currently returns 0 — the combat system is
// planned for stage 6; this stub lets callers accumulate without
// branching.
func EquipBonus(c *Card, stat Stat) int {
	_ = c
	_ = stat
	return 0
}

// Deck is a shuffled draw pile + discard pile.
type Deck struct {
	kind    Kind
	draw    []*Card
	discard []*Card
	rng     *rand.Rand
}

// NewDeck shuffles the given cards into a draw pile. The Kind is taken
// from the first non-nil card (or KindEvent if cards is empty) so HUD
// labels stay correct even when callers pass mixed slices by mistake.
func NewDeck(cs []*Card, seed int64) *Deck {
	d := &Deck{rng: rand.New(rand.NewSource(seed))}
	for _, c := range cs {
		if c == nil {
			continue
		}
		if d.kind == "" {
			d.kind = c.Kind
		}
		d.draw = append(d.draw, c)
	}
	d.shuffle()
	return d
}

// Kind returns the deck's card kind (event / item / omen).
func (d *Deck) Kind() Kind { return d.kind }

// Remaining is the number of cards in the draw pile (not the discard).
func (d *Deck) Remaining() int { return len(d.draw) }

// DiscardSize is the number of cards currently in the discard pile.
func (d *Deck) DiscardSize() int { return len(d.discard) }

// Draw pops the top card. When the draw pile is empty the discard pile
// is reshuffled in. Returns nil only when both piles are empty.
func (d *Deck) Draw() *Card {
	if len(d.draw) == 0 {
		if len(d.discard) == 0 {
			return nil
		}
		d.draw, d.discard = d.discard, d.draw[:0]
		d.shuffle()
	}
	n := len(d.draw) - 1
	c := d.draw[n]
	d.draw = d.draw[:n]
	return c
}

// Discard sends a card to the discard pile. Safe to call with nil.
func (d *Deck) Discard(c *Card) {
	if c == nil {
		return
	}
	d.discard = append(d.discard, c)
}

func (d *Deck) shuffle() {
	d.rng.Shuffle(len(d.draw), func(i, j int) {
		d.draw[i], d.draw[j] = d.draw[j], d.draw[i]
	})
}

// Apply resolves a card's Effects on the drawing player and the seated
// table. Returns one description string per effect, suitable for
// piping into the event log / flash hint. EffectManual entries return
// their Note prefixed with "(manual)" so the UI layer can highlight
// them.
//
// table may be nil; in that case TargetAll effects fall back to TargetSelf.
func Apply(c *Card, self *player.Player, table []*player.Player) []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Effects))
	for _, eff := range c.Effects {
		out = append(out, applyOne(eff, self, table))
	}
	return out
}

func applyOne(eff Effect, self *player.Player, table []*player.Player) string {
	switch eff.Kind {
	case EffectStatShift:
		return applyStatShift(eff, self, table)
	case EffectManual:
		note := strings.TrimSpace(eff.Note)
		if note == "" {
			note = "resolve as printed"
		}
		return "(manual) " + note
	default:
		return fmt.Sprintf("(unknown effect %q)", eff.Kind)
	}
}

func applyStatShift(eff Effect, self *player.Player, table []*player.Player) string {
	kind, ok := parseStat(eff.Stat)
	if !ok {
		return fmt.Sprintf("(stat_shift: bad stat %q)", eff.Stat)
	}
	targets := pickTargets(eff.Target, self, table)
	if len(targets) == 0 {
		return "(stat_shift: no target)"
	}
	parts := make([]string, 0, len(targets))
	for _, p := range targets {
		if p == nil {
			continue
		}
		before := p.StatValue(kind)
		p.Shift(kind, eff.Delta)
		after := p.StatValue(kind)
		parts = append(parts, fmt.Sprintf("%s %s %+d (%d\u2192%d)",
			labelOf(p), eff.Stat, eff.Delta, before, after))
	}
	return strings.Join(parts, "; ")
}

func pickTargets(t Target, self *player.Player, table []*player.Player) []*player.Player {
	switch t {
	case TargetAll:
		if len(table) > 0 {
			return table
		}
		return []*player.Player{self}
	case TargetSelf, "":
		return []*player.Player{self}
	default:
		return []*player.Player{self}
	}
}

func parseStat(s Stat) (player.StatKind, bool) {
	switch s {
	case StatMight:
		return player.Might, true
	case StatSpeed:
		return player.Speed, true
	case StatSanity:
		return player.Sanity, true
	case StatKnowledge:
		return player.Knowledge, true
	}
	return 0, false
}

func labelOf(p *player.Player) string {
	if p == nil {
		return "(nil)"
	}
	if p.Char != nil && p.Char.NameEN != "" {
		return p.Char.NameEN
	}
	return fmt.Sprintf("P%d", p.ID)
}
