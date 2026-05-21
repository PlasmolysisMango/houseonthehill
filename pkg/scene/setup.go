package scene

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/plasmolysismango/houseonthehill/assets"
	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
	"github.com/plasmolysismango/houseonthehill/pkg/player"
	"github.com/plasmolysismango/houseonthehill/pkg/ui"
)

// SetupScene is the character-selection screen sat between the menu and
// the game. It always seats three players (matching makeStartingPlayers)
// and lets each seat browse the character list with left/right arrows;
// already-picked characters are skipped automatically so seats never
// collide. "Random" reshuffles all three picks; "Start" boots the game.
type SetupScene struct {
	screenW, screenH int
	useExtension     bool

	chars []data.Character // master list, never mutated
	picks [seatCount]int   // index into chars, -1 = empty

	loadErr  error
	exitErr  error
	rng      *rand.Rand
	startErr error

	// HUD button rectangles (recomputed each frame in Draw).
	leftBtns  [seatCount]image.Rectangle
	rightBtns [seatCount]image.Rectangle
	cardBtns  [seatCount]image.Rectangle
	backBtn   image.Rectangle
	randomBtn image.Rectangle
	startBtn  image.Rectangle
}

const seatCount = 3

// NewSetupScene picks the first three available characters as a sane
// default; the user can browse from there. If characters.yaml fails to
// load we keep loadErr around and the layout will degrade gracefully
// (Start will still boot the game with the placeholder seating).
func NewSetupScene(useExtension bool, screenW, screenH int) *SetupScene {
	s := &SetupScene{
		screenW:      screenW,
		screenH:      screenH,
		useExtension: useExtension,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	chars, err := assets.LoadCharacters()
	if err != nil {
		s.loadErr = err
	}
	s.chars = chars
	for i := range s.picks {
		if i < len(chars) {
			s.picks[i] = i
		} else {
			s.picks[i] = -1
		}
	}
	return s
}

func (s *SetupScene) layout() {
	const (
		cardW   = 360
		cardH   = 170
		gap     = 24
		topY    = 110
		arrowW  = 36
		bottomH = 56
	)
	totalW := seatCount*cardW + (seatCount-1)*gap
	startX := (s.screenW - totalW) / 2
	for i := 0; i < seatCount; i++ {
		x := startX + i*(cardW+gap)
		s.cardBtns[i] = image.Rect(x, topY, x+cardW, topY+cardH)
		s.leftBtns[i] = image.Rect(x-arrowW-6, topY+cardH/2-arrowW/2, x-6, topY+cardH/2+arrowW/2)
		s.rightBtns[i] = image.Rect(x+cardW+6, topY+cardH/2-arrowW/2, x+cardW+arrowW+6, topY+cardH/2+arrowW/2)
	}
	by := s.screenH - bottomH - 36
	const bw = 200
	gapBtn := 24
	totalBtnW := bw*3 + gapBtn*2
	bx := (s.screenW - totalBtnW) / 2
	s.backBtn = image.Rect(bx, by, bx+bw, by+bottomH)
	s.randomBtn = image.Rect(bx+bw+gapBtn, by, bx+bw*2+gapBtn, by+bottomH)
	s.startBtn = image.Rect(bx+(bw+gapBtn)*2, by, bx+bw*3+gapBtn*2, by+bottomH)
}

func (s *SetupScene) Update() (Scene, error) {
	if s.startErr != nil {
		return s, s.startErr
	}
	s.layout()

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return NewMenuScene(s.screenW, s.screenH), nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		return s.startGame()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		s.randomize()
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s, nil
	}
	mx, my := ebiten.CursorPosition()
	p := image.Point{X: mx, Y: my}

	switch {
	case p.In(s.backBtn):
		return NewMenuScene(s.screenW, s.screenH), nil
	case p.In(s.randomBtn):
		s.randomize()
		return s, nil
	case p.In(s.startBtn):
		return s.startGame()
	}
	for i := 0; i < seatCount; i++ {
		if p.In(s.leftBtns[i]) {
			s.advance(i, -1)
			return s, nil
		}
		if p.In(s.rightBtns[i]) || p.In(s.cardBtns[i]) {
			s.advance(i, +1)
			return s, nil
		}
	}
	return s, nil
}

// advance moves seat i's character pick by step (+1 or -1), wrapping
// around and skipping anything already picked by another seat. No-op if
// fewer than two unique characters are available.
func (s *SetupScene) advance(seat, step int) {
	if len(s.chars) == 0 {
		return
	}
	cur := s.picks[seat]
	n := len(s.chars)
	for tries := 0; tries < n; tries++ {
		cur = (cur + step + n) % n
		if !s.takenByOther(seat, cur) {
			s.picks[seat] = cur
			return
		}
	}
}

func (s *SetupScene) takenByOther(seat, idx int) bool {
	for i, p := range s.picks {
		if i == seat {
			continue
		}
		if p == idx {
			return true
		}
	}
	return false
}

func (s *SetupScene) randomize() {
	if len(s.chars) < seatCount {
		return
	}
	perm := s.rng.Perm(len(s.chars))
	for i := 0; i < seatCount; i++ {
		s.picks[i] = perm[i]
	}
}

// pickedChars returns the resolved character slice in seat order. Seats
// pointing at -1 (yaml load failed) are skipped so NewGameScene can fall
// back to its placeholder path for them.
func (s *SetupScene) pickedChars() []data.Character {
	out := make([]data.Character, 0, seatCount)
	for _, idx := range s.picks {
		if idx >= 0 && idx < len(s.chars) {
			out = append(out, s.chars[idx])
		}
	}
	return out
}

func (s *SetupScene) startGame() (Scene, error) {
	gs, err := NewGameScene(s.useExtension, s.screenW, s.screenH, s.pickedChars())
	if err != nil {
		s.startErr = err
		return s, err
	}
	return gs, nil
}

func (s *SetupScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 26, G: 22, B: 28, A: 255})
	s.layout()

	mx, my := ebiten.CursorPosition()
	mp := image.Point{X: mx, Y: my}

	title := "Choose your explorers"
	subtitle := "Click a card or arrow to cycle. R / Random shuffles. Enter starts."
	ui.DrawAt(screen, title, s.screenW/2-len(title)*ui.GlyphWidth/2, 32)
	ui.DrawAt(screen, subtitle, s.screenW/2-len(subtitle)*ui.GlyphWidth/2, 56)

	if s.loadErr != nil {
		warn := "characters.yaml failed to load: " + s.loadErr.Error()
		ui.DrawColorAt(screen, warn, 16, 76, color.RGBA{R: 230, G: 120, B: 120, A: 255})
	}

	for i := 0; i < seatCount; i++ {
		s.drawSeatCard(screen, i, mp)
	}

	drawButton(screen, s.backBtn, "Back [Esc]", mp.In(s.backBtn))
	drawButton(screen, s.randomBtn, "Random [R]", mp.In(s.randomBtn))
	drawButton(screen, s.startBtn, "Start [Enter]", mp.In(s.startBtn))
}

func (s *SetupScene) drawSeatCard(dst *ebiten.Image, seat int, mp image.Point) {
	r := s.cardBtns[seat]
	hover := mp.In(r)
	bg := color.RGBA{R: 50, G: 46, B: 60, A: 255}
	if hover {
		bg = color.RGBA{R: 70, G: 60, B: 84, A: 255}
	}
	x, y, w, h := r.Min.X, r.Min.Y, r.Dx(), r.Dy()
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), bg, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 2,
		color.RGBA{R: 220, G: 200, B: 180, A: 255}, false)

	heading := fmt.Sprintf("Seat %d", seat+1)
	ui.DrawAt(dst, heading, x+12, y+10)

	idx := s.picks[seat]
	if idx < 0 || idx >= len(s.chars) {
		ui.DrawColorAt(dst, "(no character)", x+12, y+44, color.RGBA{R: 180, G: 180, B: 180, A: 255})
	} else {
		ch := s.chars[idx]
		// colour swatch
		swatch := float32(20)
		sx := float32(x + 12)
		sy := float32(y + 32)
		fill := pawnPreviewColor(ch)
		vector.DrawFilledRect(dst, sx, sy, swatch, swatch, fill, false)
		vector.StrokeRect(dst, sx, sy, swatch, swatch, 1,
			color.RGBA{R: 30, G: 30, B: 30, A: 255}, false)

		name := ch.NameEN
		if ch.NameCN != "" {
			name = fmt.Sprintf("%s / %s", ch.NameCN, ch.NameEN)
		}
		ui.DrawAt(dst, name, x+12+int(swatch)+8, y+34)
		ageLine := fmt.Sprintf("Age %d   #%d", ch.Age, ch.ID)
		ui.DrawColorAt(dst, ageLine, x+12+int(swatch)+8, y+50,
			color.RGBA{R: 200, G: 200, B: 200, A: 255})

		// stat preview: starting values
		stats := fmt.Sprintf("MIG %d   SPD %d   SAN %d   KNW %d",
			ch.Might.Tracks[clampStart(ch.Might)],
			ch.Speed.Tracks[clampStart(ch.Speed)],
			ch.Sanity.Tracks[clampStart(ch.Sanity)],
			ch.Knowledge.Tracks[clampStart(ch.Knowledge)])
		ui.DrawAt(dst, stats, x+12, y+82)
		hint := "click / \u2192 next   \u2190 prev"
		ui.DrawColorAt(dst, hint, x+12, y+h-22,
			color.RGBA{R: 170, G: 170, B: 170, A: 255})
	}

	// arrows
	drawArrow(dst, s.leftBtns[seat], "<", mp.In(s.leftBtns[seat]))
	drawArrow(dst, s.rightBtns[seat], ">", mp.In(s.rightBtns[seat]))
}

// pawnPreviewColor reuses the player package's hex-parser via a one-shot
// Player so the swatch always matches what the pawn will actually look
// like in-game (including the red fallback for unparseable hex).
func pawnPreviewColor(ch data.Character) color.RGBA {
	pp := player.New(0, &ch, board.Cell{})
	return pp.Color()
}

func clampStart(t data.StatTrack) int {
	if t.Start < 0 {
		return 0
	}
	if t.Start >= len(t.Tracks) {
		if len(t.Tracks) == 0 {
			return 0
		}
		return len(t.Tracks) - 1
	}
	return t.Start
}

func drawArrow(dst *ebiten.Image, r image.Rectangle, glyph string, hover bool) {
	bg := color.RGBA{R: 60, G: 56, B: 70, A: 255}
	if hover {
		bg = color.RGBA{R: 90, G: 78, B: 110, A: 255}
	}
	x, y, w, h := r.Min.X, r.Min.Y, r.Dx(), r.Dy()
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), bg, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 2,
		color.RGBA{R: 220, G: 200, B: 180, A: 255}, false)
	tx := x + w/2 - len(glyph)*ui.GlyphWidth/2
	ty := y + h/2 - 8
	ui.DrawAt(dst, glyph, tx, ty)
}
