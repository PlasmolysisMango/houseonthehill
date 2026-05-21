// Package player models a single explorer pawn at the table.
//
// A Player is constructed from a *data.Character (loaded from
// assets/datafs/characters.yaml) plus a starting cell. The character
// supplies four monotonic stat tracks (Might / Speed / Sanity / Knowledge)
// and a colour hex; the Player owns a mutable index into each track
// (Stats[k].Idx) which is shifted up or down by gameplay events.
//
// Index conventions match the boardgame: Track[0] is the skull / death
// marker, so Idx <= 0 means the player has died.
package player

import (
	"image/color"

	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
)

// StatKind enumerates the four explorer stats. Order matches the on-card
// layout (Might first, then Speed, Sanity, Knowledge). Use Stats[k] to
// look up a track on a Player.
type StatKind int

const (
	Might StatKind = iota
	Speed
	Sanity
	Knowledge
	StatCount = 4
)

// String returns a short, lower-case label suitable for log lines.
func (k StatKind) String() string {
	switch k {
	case Might:
		return "might"
	case Speed:
		return "speed"
	case Sanity:
		return "sanity"
	case Knowledge:
		return "knowledge"
	}
	return "unknown"
}

// Stat is one mutable attribute axis. Track is fixed at construction
// (copied from the character card); Idx is the current pointer.
type Stat struct {
	Track []int
	Idx   int
}

// Value returns the numeric value at the current index, clamped.
func (s Stat) Value() int {
	if len(s.Track) == 0 {
		return 0
	}
	i := s.Idx
	if i < 0 {
		i = 0
	} else if i >= len(s.Track) {
		i = len(s.Track) - 1
	}
	return s.Track[i]
}

// Player is one explorer pawn. The zero value is unusable; construct via
// New. Char may be nil for the placeholder pawn used before character
// selection wires in (pre-stage-2.4 game scene).
type Player struct {
	ID    int
	Char  *data.Character
	Pos   board.Cell
	Stats [StatCount]Stat
	Dead  bool
}

// New builds a player from a character at start. If c is nil the pawn has
// empty stat tracks (callers should treat it as a transient placeholder).
func New(id int, c *data.Character, start board.Cell) *Player {
	p := &Player{ID: id, Char: c, Pos: start}
	if c != nil {
		p.Stats[Might] = newStat(c.Might)
		p.Stats[Speed] = newStat(c.Speed)
		p.Stats[Sanity] = newStat(c.Sanity)
		p.Stats[Knowledge] = newStat(c.Knowledge)
	}
	return p
}

func newStat(t data.StatTrack) Stat {
	track := make([]int, len(t.Tracks))
	copy(track, t.Tracks)
	return Stat{Track: track, Idx: t.Start}
}

// StatValue is a shortcut for p.Stats[k].Value().
func (p *Player) StatValue(k StatKind) int {
	return p.Stats[k].Value()
}

// Shift moves the indicator on stat k by delta (positive = better).
// Hitting Idx 0 sets Dead = true. After death further shifts are no-ops.
// Returns the new value at the (possibly clamped) index.
func (p *Player) Shift(k StatKind, delta int) int {
	if p.Dead {
		return p.StatValue(k)
	}
	if int(k) < 0 || int(k) >= StatCount {
		return 0
	}
	s := &p.Stats[k]
	if len(s.Track) == 0 {
		return 0
	}
	s.Idx += delta
	if s.Idx <= 0 {
		s.Idx = 0
		p.Dead = true
	} else if s.Idx >= len(s.Track) {
		s.Idx = len(s.Track) - 1
	}
	return s.Value()
}

// Color returns the pawn fill colour. If Char.ColorHex parses successfully
// (formats: "#RRGGBB" / "RRGGBB" / "#RGB"), that colour is used; otherwise
// a red fallback is returned (matches the pre-stage-2 single-pawn look).
func (p *Player) Color() color.RGBA {
	if p.Char != nil {
		if c, ok := parseHexRGB(p.Char.ColorHex); ok {
			return c
		}
	}
	return color.RGBA{R: 220, G: 50, B: 50, A: 255}
}

// PawnColor returns the colour the pawn should be rendered with on the
// board: Color() for living players, a desaturated grey for the dead.
// Kept on the player type so renderers don't have to special-case Dead.
func (p *Player) PawnColor() color.RGBA {
	if p.Dead {
		return color.RGBA{R: 90, G: 90, B: 90, A: 200}
	}
	return p.Color()
}

// parseHexRGB parses "#RRGGBB", "RRGGBB" or "#RGB" forms. Returns ok=false
// on any malformed input.
func parseHexRGB(s string) (color.RGBA, bool) {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	switch len(s) {
	case 6:
		r, ok1 := parseHexByte(s[0:2])
		g, ok2 := parseHexByte(s[2:4])
		b, ok3 := parseHexByte(s[4:6])
		if ok1 && ok2 && ok3 {
			return color.RGBA{R: r, G: g, B: b, A: 255}, true
		}
	case 3:
		r, ok1 := parseHexNibble(s[0])
		g, ok2 := parseHexNibble(s[1])
		b, ok3 := parseHexNibble(s[2])
		if ok1 && ok2 && ok3 {
			return color.RGBA{R: r * 17, G: g * 17, B: b * 17, A: 255}, true
		}
	}
	return color.RGBA{}, false
}

func parseHexByte(s string) (uint8, bool) {
	hi, ok1 := parseHexNibble(s[0])
	lo, ok2 := parseHexNibble(s[1])
	if !ok1 || !ok2 {
		return 0, false
	}
	return hi<<4 | lo, true
}

func parseHexNibble(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return 10 + c - 'a', true
	case c >= 'A' && c <= 'F':
		return 10 + c - 'A', true
	}
	return 0, false
}
