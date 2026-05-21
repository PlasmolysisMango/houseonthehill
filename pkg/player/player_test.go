package player

import (
	"image/color"
	"testing"

	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
)

// sampleChar returns a deterministic test character whose tracks have a
// well-defined skull-at-zero / max-at-end shape so we can exercise both
// shift directions without depending on the placeholder yaml values.
func sampleChar() *data.Character {
	return &data.Character{
		ID:       99,
		NameCN:   "测试",
		NameEN:   "Tester",
		Age:      30,
		ColorHex: "#0781BF",
		Might:     data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 6}, Start: 3},
		Speed:     data.StatTrack{Tracks: []int{0, 2, 3, 4, 4, 5, 6}, Start: 3},
		Sanity:    data.StatTrack{Tracks: []int{0, 2, 3, 4, 4, 5, 6}, Start: 3},
		Knowledge: data.StatTrack{Tracks: []int{0, 1, 2, 3, 4, 5, 7}, Start: 3},
	}
}

func TestNew_FillsStatsFromCharacter(t *testing.T) {
	p := New(1, sampleChar(), board.Cell{X: 5, Y: 7})
	if p.ID != 1 || p.Pos.X != 5 || p.Pos.Y != 7 {
		t.Fatalf("New: bad header: %+v", p)
	}
	if p.Dead {
		t.Fatalf("fresh player should not be dead")
	}
	if got := p.StatValue(Might); got != 3 {
		t.Errorf("Might.Value = %d, want 3", got)
	}
	if got := p.StatValue(Knowledge); got != 3 {
		t.Errorf("Knowledge.Value = %d, want 3", got)
	}
	// Track must be a copy: mutating it on the original character should
	// not bleed into the player.
	c := sampleChar()
	pp := New(2, c, board.Cell{})
	c.Might.Tracks[3] = 999
	if pp.StatValue(Might) == 999 {
		t.Fatalf("Stat track was not copied; mutation leaked through")
	}
}

func TestNew_NilCharacter(t *testing.T) {
	p := New(0, nil, board.Cell{X: 1, Y: 2})
	if p == nil {
		t.Fatal("New(nil) returned nil")
	}
	for k := Might; k <= Knowledge; k++ {
		if got := p.StatValue(k); got != 0 {
			t.Errorf("nil-char player StatValue(%v) = %d, want 0", k, got)
		}
	}
}

func TestShift_PositiveAndNegative(t *testing.T) {
	p := New(1, sampleChar(), board.Cell{})
	got := p.Shift(Might, +2)
	if got != 5 {
		t.Errorf("Might +2: got %d, want 5", got)
	}
	if p.StatValue(Might) != 5 {
		t.Errorf("Might.Value after +2 = %d, want 5", p.StatValue(Might))
	}
	got = p.Shift(Might, -1)
	if got != 4 {
		t.Errorf("Might -1: got %d, want 4", got)
	}
}

func TestShift_ClampsAtTopOfTrack(t *testing.T) {
	p := New(1, sampleChar(), board.Cell{})
	// Start=3, Track len=7 → max idx = 6, value = 6.
	p.Shift(Might, +99)
	if p.StatValue(Might) != 6 {
		t.Errorf("clamped top: value = %d, want 6", p.StatValue(Might))
	}
	if p.Dead {
		t.Errorf("clamped at top should not kill the player")
	}
}

func TestShift_HittingZeroKillsPlayer(t *testing.T) {
	p := New(1, sampleChar(), board.Cell{})
	// Start=3, three -1 shifts puts Idx at 0 → Track[0]=0, Dead=true.
	p.Shift(Might, -3)
	if !p.Dead {
		t.Fatalf("Idx hit 0 but Dead=false")
	}
	if p.StatValue(Might) != 0 {
		t.Errorf("dead player Might value = %d, want 0", p.StatValue(Might))
	}
	// Further shifts must not resurrect.
	p.Shift(Might, +5)
	if !p.Dead {
		t.Errorf("Shift after death lifted Dead flag")
	}
	if p.StatValue(Might) != 0 {
		t.Errorf("dead player Might after +5 = %d, want 0 (frozen)",
			p.StatValue(Might))
	}
}

func TestColor_FromCharacterHex(t *testing.T) {
	p := New(1, sampleChar(), board.Cell{})
	got := p.Color()
	want := color.RGBA{R: 0x07, G: 0x81, B: 0xBF, A: 255}
	if got != want {
		t.Errorf("Color = %+v, want %+v", got, want)
	}
}

func TestColor_NilCharacterFallback(t *testing.T) {
	p := New(0, nil, board.Cell{})
	got := p.Color()
	// Fallback red matches the pre-stage-2 single-pawn appearance.
	want := color.RGBA{R: 220, G: 50, B: 50, A: 255}
	if got != want {
		t.Errorf("nil-char Color = %+v, want %+v", got, want)
	}
}

func TestColor_BadHexFallsBack(t *testing.T) {
	c := sampleChar()
	c.ColorHex = "not a colour"
	p := New(0, c, board.Cell{})
	got := p.Color()
	if got.R != 220 || got.G != 50 || got.B != 50 {
		t.Errorf("bad-hex Color = %+v, want red fallback", got)
	}
}

func TestParseHex_ShortForm(t *testing.T) {
	c, ok := parseHexRGB("#0AF")
	if !ok {
		t.Fatal("parseHexRGB(\"#0AF\") returned ok=false")
	}
	want := color.RGBA{R: 0x00, G: 0xAA, B: 0xFF, A: 255}
	if c != want {
		t.Errorf("short-form #0AF = %+v, want %+v", c, want)
	}
}

func TestStatKindString(t *testing.T) {
	cases := map[StatKind]string{
		Might: "might", Speed: "speed",
		Sanity: "sanity", Knowledge: "knowledge",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("StatKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
