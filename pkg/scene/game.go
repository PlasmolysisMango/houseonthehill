package scene

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/plasmolysismango/houseonthehill/assets"
	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/camera"
	"github.com/plasmolysismango/houseonthehill/pkg/cards"
	"github.com/plasmolysismango/houseonthehill/pkg/component"
	"github.com/plasmolysismango/houseonthehill/pkg/data"
	"github.com/plasmolysismango/houseonthehill/pkg/deck"
	"github.com/plasmolysismango/houseonthehill/pkg/dice"
	"github.com/plasmolysismango/houseonthehill/pkg/player"
	"github.com/plasmolysismango/houseonthehill/pkg/tile"
	"github.com/plasmolysismango/houseonthehill/pkg/turn"
	"github.com/plasmolysismango/houseonthehill/pkg/ui"
	"github.com/plasmolysismango/houseonthehill/pkg/utils"
)

// TileSize is the world-space pixel size of a single board cell.
const TileSize = 450

const clickPixelThreshold = 5

type gameState int

const (
	stateIdle gameState = iota
	stateDrawing
)

// GameScene runs the core exploration loop.
type GameScene struct {
	screenW, screenH int

	mouse  *utils.Mouse
	cam    *camera.Camera
	board  *board.Board[*component.Room]
	deck   *deck.Deck
	players []*player.Player // seating order, indexed by engine.cur
	engine  *turn.Engine     // round / phase / dead-skipping state machine

	state     gameState
	drawn     *tile.RoomTile
	target    board.Cell
	entrySide int  // world side of the drawn tile that must face the player
	useFront  bool // toggle whether to flip drawn tile preview face up

	// click vs drag detection
	pressX, pressY int
	dragged        bool

	// HUD button rectangles (recomputed every frame in drawHUD).
	drawBtn    image.Rectangle
	endTurnBtn image.Rectangle

	// dice tester (roadmap stage 3 acceptance criterion). Toggled with T.
	dice      *dice.Die
	diceShow  bool
	diceCount int
	diceFaces []int
	diceTotal int

	// Roadmap stage 4 — three card decks + Haunt tracker. Decks are
	// nil-tolerant: if cards.yaml is missing the runtime degrades to a
	// stage-3 game (no triggers, no Haunt Roll).
	eventDeck *cards.Deck
	itemDeck  *cards.Deck
	omenDeck  *cards.Deck
	haunt     *turn.HauntTracker
	cardDlg   *cardDialog

	// Inventory: held cards per player (keyed by player.ID) and cards
	// dropped on the floor when a player dies (keyed by board cell).
	// Lives in GameScene to avoid a pkg/player → pkg/cards circular dep.
	inventory    map[int][]*cards.Card
	droppedItems map[board.Cell][]*cards.Card

	// transient hint
	hintMsg     string
	hintExpires time.Time
}

// NewGameScene loads assets and lays down the starter rooms. The picks
// slice (length 3 expected) provides the characters seated at the table;
// pass nil to fall back to the first three from characters.yaml so the
// scene stays usable from quick-start paths and tests.
func NewGameScene(useExtension bool, screenW, screenH int, picks []data.Character) (*GameScene, error) {
	starters, err := assets.LoadStarterTiles()
	if err != nil {
		return nil, fmt.Errorf("load starter tiles: %w", err)
	}
	baseTiles, err := assets.LoadBaseDeck()
	if err != nil {
		return nil, fmt.Errorf("load base deck: %w", err)
	}
	allTiles := baseTiles
	if useExtension {
		ext, err := assets.LoadExtensionDeck()
		if err != nil {
			return nil, fmt.Errorf("load extension deck: %w", err)
		}
		allTiles = append(allTiles, ext...)
	}

	g := &GameScene{
		screenW:   screenW,
		screenH:   screenH,
		mouse:     &utils.Mouse{},
		cam:       camera.New(screenW, screenH),
		board:     board.New[*component.Room](TileSize),
		deck:      deck.New(allTiles, time.Now().UnixNano()),
		state:     stateIdle,
		dice:      dice.New(time.Now().UnixNano()),
		diceCount: 4,
		haunt:        turn.NewHauntTracker(),
		inventory:    make(map[int][]*cards.Card),
		droppedItems: make(map[board.Cell][]*cards.Card),
	}

	// Card decks (roadmap stage 4). Failure to load cards.yaml is
	// non-fatal — we log a soft warning via the hint and let the rest
	// of the scene proceed without triggers.
	if evs, its, oms, err := assets.LoadCards(); err == nil {
		seed := time.Now().UnixNano()
		g.eventDeck = cards.NewDeck(evs, seed+1)
		g.itemDeck = cards.NewDeck(its, seed+2)
		g.omenDeck = cards.NewDeck(oms, seed+3)
	}

	// Place starter rooms at the cells listed in tile_meta.yaml's
	// start_cell field. The Entrance Hall (id 1002) becomes the player's
	// spawn cell; if it is missing we fall back to (0,0).
	spawn := board.Cell{X: 0, Y: 0}
	for _, p := range starters {
		cell := board.Cell{X: p.X, Y: p.Y}
		g.board.Place(cell, component.NewRoom(p.Tile))
		if p.Tile.ID == 1002 {
			spawn = cell
		}
	}
	g.players = makeStartingPlayers(spawn, picks)
	g.engine = turn.New(g.players)
	g.engine.BeginTurn(speedOf(g.cur()))

	// centre camera on the leading player's tile centre
	g.cam.CenterOn(float64(g.curPos().X)*TileSize+TileSize/2,
		float64(g.curPos().Y)*TileSize+TileSize/2)
	// fit comfortably: scale so a tile is about 200px on screen
	g.cam.Scale = 200.0 / TileSize

	g.flash("Click an adjacent cell to move or draw a room. Or press D / click Draw. E ends turn.")
	return g, nil
}

// makeStartingPlayers builds the seating list. If picks is non-empty
// each entry becomes one seated player (in order). When picks is nil or
// short we top up from the first characters in characters.yaml; if that
// also fails we fall back to character-less placeholders so the scene
// is still playable.
func makeStartingPlayers(spawn board.Cell, picks []data.Character) []*player.Player {
	const seats = 3
	var fallback []data.Character
	if len(picks) < seats {
		if chars, err := assets.LoadCharacters(); err == nil {
			fallback = chars
		}
	}
	out := make([]*player.Player, 0, seats)
	for i := 0; i < seats; i++ {
		var c *player.Player
		switch {
		case i < len(picks):
			ch := picks[i]
			c = player.New(i+1, &ch, spawn)
		case i < len(fallback):
			ch := fallback[i]
			c = player.New(i+1, &ch, spawn)
		default:
			c = player.New(i+1, nil, spawn)
		}
		out = append(out, c)
	}
	return out
}

// cur returns the player whose turn it currently is, or nil when the
// game is over.
func (g *GameScene) cur() *player.Player { return g.engine.Current() }

// curPos returns the current player's board position. When the engine
// has no live player (game over) it returns the zero cell so callers
// don't have to special-case nil before computing UI overlays.
func (g *GameScene) curPos() board.Cell {
	if p := g.cur(); p != nil {
		return p.Pos
	}
	return board.Cell{}
}

// playerLabel returns a short identifier for HUD lines, preferring the
// English character name and falling back to a numeric seat tag.
func playerLabel(p *player.Player) string {
	if p == nil {
		return "--"
	}
	if p.Char != nil && p.Char.NameEN != "" {
		return p.Char.NameEN
	}
	return fmt.Sprintf("P%d", p.ID)
}

// speedOf returns the player's current Speed stat value, or 0 when p is
// nil. Used to seed turn.Engine's stepsLeft budget at PhaseStart.
func speedOf(p *player.Player) int {
	if p == nil {
		return 0
	}
	return p.StatValue(player.Speed)
}

func (g *GameScene) flash(msg string) {
	g.hintMsg = msg
	g.hintExpires = time.Now().Add(4 * time.Second)
}

// Update drives input handling and state transitions.
func (g *GameScene) Update() (Scene, error) {
	defer g.mouse.Update()

	if inpututil.IsKeyJustPressed(ebiten.KeyQ) && ebiten.IsKeyPressed(ebiten.KeyControl) {
		return nil, ebiten.Termination
	}

	g.cam.Update(g.mouse)
	g.layoutDrawButton()
	g.layoutEndTurnButton()

	// click vs drag detection on the left button
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.pressX, g.pressY = ebiten.CursorPosition()
		g.dragged = false
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		cx, cy := ebiten.CursorPosition()
		if absInt(cx-g.pressX) > clickPixelThreshold || absInt(cy-g.pressY) > clickPixelThreshold {
			g.dragged = true
		}
	}
	clicked := inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && !g.dragged

	cx, cy := ebiten.CursorPosition()
	wx, wy := g.cam.ScreenToWorld(cx, cy)
	hover := worldToCell(wx, wy)

	// HUD button + keyboard shortcut for the "draw a room" action. They
	// pick the first cardinal neighbour that has a door from the current
	// room and is currently empty, then enter stateDrawing as if the user
	// had clicked that yellow-highlighted cell.
	hudPoint := image.Point{X: cx, Y: cy}
	hudClicked := clicked && hudPoint.In(g.drawBtn)
	if g.state == stateIdle && (inpututil.IsKeyJustPressed(ebiten.KeyD) || hudClicked) {
		g.tryDrawAtCandidate()
		if hudClicked {
			// Consume the click so it can't also fire as a board move.
			return g, nil
		}
	}

	// End Turn: HUD button or E key. Idle state only so it can't be hit
	// in the middle of a placement orientation.
	endClicked := clicked && hudPoint.In(g.endTurnBtn)
	if g.state == stateIdle && !g.engine.IsGameOver() &&
		(inpututil.IsKeyJustPressed(ebiten.KeyE) || endClicked) {
		g.engine.EndTurn()
		if cur := g.cur(); cur != nil {
			g.engine.BeginTurn(speedOf(cur))
			g.onTurnStart(cur)
			g.cam.CenterOn(float64(cur.Pos.X)*TileSize+TileSize/2,
				float64(cur.Pos.Y)*TileSize+TileSize/2)
			g.flash(fmt.Sprintf("Turn: %s   Speed %d", playerLabel(cur), g.engine.StepsLeft()))
		} else {
			g.flash("Game over.")
		}
		if endClicked {
			return g, nil
		}
	}

	// Card dialog (roadmap stage 4) has top input priority. While open
	// it swallows ALL keys / clicks so the rest of the pipeline can't
	// see them and accidentally double-fire stat shifts, end-turn, etc.
	// Input behaviour depends on card kind:
	//   event: [A] Apply+close  [M/Esc] close (discard)
	//   item:  [K/Esc] Keep in bag  [U] Use (apply+discard)
	//   omen:  [A] Apply effects  [K/Esc] Keep (omens always kept)
	if g.cardDlg != nil {
		c := g.cardDlg.card
		switch {
		case c.Kind == cards.KindEvent:
			if inpututil.IsKeyJustPressed(ebiten.KeyA) {
				g.applyCard()
				g.closeCard()
			} else if inpututil.IsKeyJustPressed(ebiten.KeyM) ||
				inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				g.closeCard()
			}
		case c.Kind == cards.KindItem:
			if inpututil.IsKeyJustPressed(ebiten.KeyK) ||
				inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				g.keepCard()
			} else if inpututil.IsKeyJustPressed(ebiten.KeyU) {
				g.applyCard()
				g.closeCard() // discard after use
			}
		case c.Kind == cards.KindOmen:
			if inpututil.IsKeyJustPressed(ebiten.KeyA) {
				g.applyCard() // show results, stay open
			} else if inpututil.IsKeyJustPressed(ebiten.KeyK) ||
				inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				g.keepCard()
			}
		}
		return g, nil
	}

	// Dice tester (roadmap stage 3 acceptance criterion). T toggles a
	// modal panel; while it's open digits 1..8 select dice count and
	// Space / R re-rolls. The modal also swallows the rest of the input
	// pipeline so digit keys don't leak through to the stat-shift
	// shortcuts below.
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.diceShow = !g.diceShow
		if g.diceShow {
			g.rollDice()
		}
		return g, nil
	}
	if g.diceShow {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.diceShow = false
		} else if inpututil.IsKeyJustPressed(ebiten.KeyR) ||
			inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.rollDice()
		} else {
			for n := 1; n <= 8; n++ {
				key := ebiten.Key(int(ebiten.KeyDigit0) + n)
				if inpututil.IsKeyJustPressed(key) {
					g.diceCount = n
					g.rollDice()
					break
				}
			}
		}
		return g, nil
	}

	// Card test keys (roadmap stage 4): force-draw from each deck
	// without entering a new room. Useful for verifying the modal +
	// Haunt Roll independently of map exploration.
	if cur := g.cur(); cur != nil && !cur.Dead && g.state == stateIdle {
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyN):
			g.openCard(g.eventDeck)
		case inpututil.IsKeyJustPressed(ebiten.KeyI):
			g.openCard(g.itemDeck)
		case inpututil.IsKeyJustPressed(ebiten.KeyO):
			g.openCard(g.omenDeck)
		case inpututil.IsKeyJustPressed(ebiten.KeyG):
			// Pick up dropped cards at the current cell.
			if g.pickupDropped(cur) {
				g.flash(fmt.Sprintf("%s picked up dropped cards!", playerLabel(cur)))
			}
		}
	}

	// Stat-shift test shortcuts (roadmap stage 2 acceptance criterion):
	// 1/2/3/4 push the current player's Might/Speed/Sanity/Knowledge
	// indicator one slot to the right; hold Shift to push left. Hitting
	// the skull (Idx 0) marks the player Dead; the next EndTurn will skip
	// past them automatically.
	if cur := g.cur(); cur != nil && !cur.Dead && g.state == stateIdle {
		delta := 1
		if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
			delta = -1
		}
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyDigit1):
			cur.Shift(player.Might, delta)
			g.flash(fmt.Sprintf("%s Might %+d\u2192%d", playerLabel(cur), delta, cur.StatValue(player.Might)))
		case inpututil.IsKeyJustPressed(ebiten.KeyDigit2):
			cur.Shift(player.Speed, delta)
			g.flash(fmt.Sprintf("%s Speed %+d\u2192%d", playerLabel(cur), delta, cur.StatValue(player.Speed)))
		case inpututil.IsKeyJustPressed(ebiten.KeyDigit3):
			cur.Shift(player.Sanity, delta)
			g.flash(fmt.Sprintf("%s Sanity %+d\u2192%d", playerLabel(cur), delta, cur.StatValue(player.Sanity)))
		case inpututil.IsKeyJustPressed(ebiten.KeyDigit4):
			cur.Shift(player.Knowledge, delta)
			g.flash(fmt.Sprintf("%s Knowledge %+d\u2192%d", playerLabel(cur), delta, cur.StatValue(player.Knowledge)))
		}
		if cur.Dead {
			g.dropInventory(cur)
			g.flash(playerLabel(cur) + " has died.")
		}
	}

	switch g.state {
	case stateIdle:
		if clicked && board.IsAdjacent(g.curPos(), hover) {
			side, ok := sideFromDelta(g.curPos(), hover)
			if !ok {
				break
			}
			curRoom, _ := g.board.At(g.curPos())
			if curRoom == nil || !curRoom.Tile.HasDoor(side) {
				g.flash("No door on that side of this room.")
				break
			}
			if g.engine.StepsLeft() <= 0 {
				g.flash("No moves left this turn. Press E to end turn.")
				break
			}
			if room, ok := g.board.At(hover); ok {
				if !room.Tile.HasDoor(tile.OppositeSide(side)) {
					g.flash("The next room has no door facing here.")
					break
				}
				if cur := g.cur(); cur != nil {
					cur.Pos = hover
					// Check for dropped items at the new cell.
					if len(g.droppedItems[hover]) > 0 {
						g.flash(fmt.Sprintf("Dropped items here (%d)! Press G to pick up.", len(g.droppedItems[hover])))
					}
				}
				g.engine.SpendStep()
				g.flash(fmt.Sprintf("Moved.   Steps left: %d", g.engine.StepsLeft()))
			} else {
				if t := g.deck.Draw(); t != nil {
					g.drawn = t
					g.target = hover
					g.entrySide = tile.OppositeSide(side) // side of new tile facing the player
					g.useFront = true
					g.state = stateDrawing
					// auto-rotate so the entry side has a door if possible
					g.autoOrientDrawn()
					g.flash("R rotate, click to confirm (door must face you), Esc cancel.")
				} else {
					g.flash("Deck is empty.")
				}
			}
		}
	case stateDrawing:
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.drawn.RotateCW()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF) {
			g.useFront = !g.useFront
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.deck.ReturnTop(g.drawn)
			g.drawn = nil
			g.state = stateIdle
			g.flash("Cancelled.")
		}
		if clicked {
			if !g.drawn.HasDoor(g.entrySide) {
				g.flash("This rotation has no door facing you. Press R to rotate.")
			} else {
				room := component.NewRoom(g.drawn)
				room.Revealed = true
				g.board.Place(g.target, room)
				if cur := g.cur(); cur != nil {
					cur.Pos = g.target
				}
				g.engine.SpendStep()
				placedRoom := room
				g.drawn = nil
				g.state = stateIdle
				g.flash(fmt.Sprintf("Room placed.   Steps left: %d", g.engine.StepsLeft()))
				if placedRoom != nil {
					g.triggerRoomCard(placedRoom.Tile.NameCN)
				}
			}
		}
	}

	return g, nil
}

// Draw renders the board, highlights, player, drawn tile preview and HUD.
func (g *GameScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 14, G: 12, B: 18, A: 255})
	camGeo := g.cam.GeoM()

	// 1. placed rooms
	for cell, room := range g.board.All() {
		room.Draw(screen, float64(cell.X)*TileSize, float64(cell.Y)*TileSize, TileSize, camGeo)
	}

	// 2. neighbour highlights from the player's cell, only on sides that have
	// a door. Green = can move (target also has door back). Yellow = can draw
	// a new room. Greyed-out targets that lack a return door are skipped.
	curRoom, _ := g.board.At(g.curPos())
	for i, n := range g.board.Neighbors(g.curPos()) {
		if curRoom == nil || !curRoom.Tile.HasDoor(i) {
			continue
		}
		var col color.Color
		if room, ok := g.board.At(n); ok {
			if !room.Tile.HasDoor(tile.OppositeSide(i)) {
				continue // walled-off neighbour
			}
			col = color.RGBA{R: 80, G: 200, B: 120, A: 220} // can move
		} else {
			col = color.RGBA{R: 230, G: 200, B: 80, A: 220} // can draw
		}
		drawCellRect(screen, n, camGeo, col, 3)
	}

	// 3. hover indicator
	cx, cy := ebiten.CursorPosition()
	wx, wy := g.cam.ScreenToWorld(cx, cy)
	hover := worldToCell(wx, wy)
	drawCellRect(screen, hover, camGeo, color.RGBA{R: 255, G: 255, B: 255, A: 90}, 1)

	// 4. drawn tile preview (semi-transparent over target)
	if g.state == stateDrawing && g.drawn != nil {
		component.DrawTile(screen, g.drawn, g.useFront,
			float64(g.target.X)*TileSize, float64(g.target.Y)*TileSize,
			TileSize, camGeo, 0.75)
		drawCellRect(screen, g.target, camGeo, color.RGBA{R: 255, G: 80, B: 80, A: 255}, 4)
	}

	// 5. player pawns. All seats render in their character colour; the
	// current player gets an extra gold ring so it is obvious whose turn
	// it is.
	current := g.cur()
	for _, p := range g.players {
		drawPawn(screen, p, TileSize, camGeo)
	}
	if current != nil {
		drawCurrentPawnRing(screen, current, TileSize, camGeo)
	}

	// 6. HUD
	g.drawHUD(screen)
}

func (g *GameScene) drawHUD(screen *ebiten.Image) {
	// top bar
	vector.DrawFilledRect(screen, 0, 0, float32(g.screenW), 28, color.RGBA{R: 0, G: 0, B: 0, A: 160}, false)
	state := "IDLE"
	if g.state == stateDrawing {
		state = "DRAWING"
	}
	turnLabel := "--"
	if cur := g.cur(); cur != nil {
		turnLabel = playerLabel(cur)
	}
	hud := fmt.Sprintf("Round %d  Turn: %s [%s]   State: %s   Steps: %d   Omens: %d   Deck: %d left   Pos: (%d,%d)   Rooms: %d",
		g.engine.Round(), turnLabel, g.engine.Phase().String(),
		state, g.engine.StepsLeft(), g.haunt.OmenCount(),
		g.deck.Remaining(), g.curPos().X, g.curPos().Y, len(g.board.All()))
	ui.DrawAt(screen, hud, 8, 6)

	// hover-room line: shows the names from tile_meta.yaml. CJK glyphs
	// cannot be rendered by the built-in debug font; once a CJK face is
	// wired in (roadmap stage 8) the same string will show 中文.
	cx, cy := ebiten.CursorPosition()
	wx, wy := g.cam.ScreenToWorld(cx, cy)
	if room, ok := g.board.At(worldToCell(wx, wy)); ok && room != nil {
		ui.DrawAt(screen, "Hover: "+roomLabel(room.Tile), 8, 48)
		if rule := assets.RoomRule(room.Tile.NameEN); rule != "" {
			// rule_text is CJK; once a CJK font is wired in it will
			// render automatically. For now show a truncated snippet.
			snippet := rule
			if len([]rune(snippet)) > 60 {
				snippet = string([]rune(snippet)[:60]) + "..."
			}
			ui.DrawAt(screen, "Effect: "+snippet, 8, 64)
		}
	}

	// Draw-room button (top-right). Only enabled in stateIdle so the user
	// can't accidentally throw away the in-flight tile while orienting it.
	g.layoutDrawButton()
	g.layoutEndTurnButton()
	hover := image.Point{X: cx, Y: cy}.In(g.drawBtn)
	enabled := g.state == stateIdle && g.deck.Remaining() > 0 && g.drawCandidate() != nil
	label := fmt.Sprintf("Draw [D] (%d)", g.deck.Remaining())
	drawHUDButton(screen, g.drawBtn, label, hover, enabled)

	// End Turn button just below it.
	endHover := image.Point{X: cx, Y: cy}.In(g.endTurnBtn)
	endEnabled := g.state == stateIdle && !g.engine.IsGameOver()
	drawHUDButton(screen, g.endTurnBtn, "End Turn [E]", endHover, endEnabled)

	// bottom hint
	if time.Now().Before(g.hintExpires) && g.hintMsg != "" {
		vector.DrawFilledRect(screen, 0, float32(g.screenH-28), float32(g.screenW), 28, color.RGBA{R: 0, G: 0, B: 0, A: 160}, false)
		ui.DrawAt(screen, g.hintMsg, 8, g.screenH-22)
	}

	// help line
	help := "L-click adj cell: move/draw   D / Draw button: pick a yellow neighbour   T: dice tester   N/I/O: force draw event/item/omen   Drag: pan   Wheel: zoom   Mid-click: reset   In drawing: R rotate, F flip, Esc cancel   In card: A apply, M manual, Esc close"
	vector.DrawFilledRect(screen, 0, 28, float32(g.screenW), 18, color.RGBA{R: 0, G: 0, B: 0, A: 120}, false)
	ui.DrawAt(screen, help, 8, 30)

	// 4-row stat panel for the current player (bottom-left).
	g.drawStatPanel(screen)

	// inventory panel (below stat panel).
	g.drawInventoryPanel(screen)

	// optional dice-tester modal (toggled with T).
	if g.diceShow {
		g.drawDicePanel(screen)
	}

	// optional card dialog (roadmap stage 4). Topmost overlay.
	if g.cardDlg != nil {
		g.drawCardDialog(screen)
	}
}

// drawStatPanel renders the current player's four stat tracks as a row
// of value blocks each, with the active index highlighted in gold and
// the Idx=0 skull cell shown in dark red. Dead players get a [DEAD] tag
// and the skull cell flashes bright red. Sits in the bottom-left corner,
// above the hint strip.
func (g *GameScene) drawStatPanel(dst *ebiten.Image) {
	cur := g.cur()
	if cur == nil {
		return
	}
	const (
		cellW   = 26
		cellH   = 22
		cellGap = 2
		labelW  = 36
		padX    = 8
		padY    = 8
		rowGap  = 4
		titleH  = 18
	)
	// Width fits the longest stat track among the four so the cells align.
	maxTrack := 0
	for k := 0; k < player.StatCount; k++ {
		if n := len(cur.Stats[k].Track); n > maxTrack {
			maxTrack = n
		}
	}
	if maxTrack == 0 {
		maxTrack = 8 // placeholder (Char==nil)
	}
	panelW := padX*2 + labelW + maxTrack*(cellW+cellGap) - cellGap
	panelH := padY*2 + titleH + 4*cellH + 3*rowGap
	px := 12
	py := g.screenH - 28 - panelH - 8 // sit above the bottom hint bar

	// backdrop
	vector.DrawFilledRect(dst, float32(px), float32(py), float32(panelW), float32(panelH),
		color.RGBA{R: 0, G: 0, B: 0, A: 180}, false)
	vector.StrokeRect(dst, float32(px), float32(py), float32(panelW), float32(panelH), 1,
		color.RGBA{R: 220, G: 200, B: 180, A: 220}, false)

	// title row: colour swatch + name (+ [DEAD])
	swatch := float32(12)
	sx := float32(px + padX)
	sy := float32(py+padY) + 2
	vector.DrawFilledRect(dst, sx, sy, swatch, swatch, cur.Color(), false)
	vector.StrokeRect(dst, sx, sy, swatch, swatch, 1,
		color.RGBA{R: 30, G: 30, B: 30, A: 255}, false)
	title := playerLabel(cur)
	if cur.Dead {
		title += "  [DEAD]"
	}
	ui.DrawAt(dst, title, px+padX+int(swatch)+6, py+padY)

	// four stat rows
	rowY := py + padY + titleH + rowGap
	labels := [...]string{"MIG", "SPD", "SAN", "KNW"}
	for k := 0; k < player.StatCount; k++ {
		ui.DrawAt(dst, labels[k], px+padX, rowY+(cellH-12)/2)
		s := cur.Stats[k]
		cx := px + padX + labelW
		for i, v := range s.Track {
			xf := float32(cx + i*(cellW+cellGap))
			yf := float32(rowY)
			active := i == s.Idx
			skull := i == 0
			var fill color.RGBA
			switch {
			case active && skull && cur.Dead:
				fill = color.RGBA{R: 220, G: 50, B: 50, A: 255}
			case active:
				fill = color.RGBA{R: 230, G: 190, B: 60, A: 255}
			case skull:
				fill = color.RGBA{R: 110, G: 30, B: 30, A: 220}
			default:
				fill = color.RGBA{R: 50, G: 48, B: 60, A: 220}
			}
			vector.DrawFilledRect(dst, xf, yf, float32(cellW), float32(cellH), fill, false)
			vector.StrokeRect(dst, xf, yf, float32(cellW), float32(cellH), 1,
				color.RGBA{R: 30, G: 30, B: 30, A: 200}, false)
			txt := fmt.Sprintf("%d", v)
			if skull {
				txt = "X"
			}
			tcol := color.RGBA{R: 240, G: 240, B: 240, A: 255}
			if active && !(skull && cur.Dead) {
				tcol = color.RGBA{R: 30, G: 24, B: 10, A: 255}
			}
			tx := int(xf) + cellW/2 - len(txt)*ui.GlyphWidth/2
			ty := int(yf) + cellH/2 - 8
			ui.DrawColorAt(dst, txt, tx, ty, tcol)
		}
		rowY += cellH + rowGap
	}
}

// layoutDrawButton recomputes the HUD draw-button rectangle. Called every
// frame so window resizes are picked up automatically.
func (g *GameScene) layoutDrawButton() {
	const bw, bh = 180, 32
	x := g.screenW - bw - 12
	y := 4
	g.drawBtn = image.Rect(x, y, x+bw, y+bh)
}

// layoutEndTurnButton positions the "End Turn" button just below the
// Draw button on the right edge of the HUD.
func (g *GameScene) layoutEndTurnButton() {
	const bw, bh = 180, 32
	x := g.screenW - bw - 12
	y := 4 + 32 + 6
	g.endTurnBtn = image.Rect(x, y, x+bw, y+bh)
}

// drawCandidate returns the first cardinal neighbour of the player that has
// a door (from the player's room) AND is currently empty—i.e. exactly the
// cell that would yellow-highlight in Draw(). Returns nil when no candidate
// exists (e.g. all neighbours are walls or already filled).
func (g *GameScene) drawCandidate() *board.Cell {
	curRoom, _ := g.board.At(g.curPos())
	if curRoom == nil {
		return nil
	}
	for i, n := range g.board.Neighbors(g.curPos()) {
		if !curRoom.Tile.HasDoor(i) {
			continue
		}
		if _, occupied := g.board.At(n); occupied {
			continue
		}
		cell := n
		return &cell
	}
	return nil
}

// tryDrawAtCandidate is the shared implementation used by both the D
// keyboard shortcut and the HUD Draw button. It picks the first valid
// adjacent empty cell, draws a tile, and enters stateDrawing.
func (g *GameScene) tryDrawAtCandidate() {
	if g.state != stateIdle {
		return
	}
	if g.engine.StepsLeft() <= 0 {
		g.flash("No moves left this turn. Press E to end turn.")
		return
	}
	cell := g.drawCandidate()
	if cell == nil {
		g.flash("No empty doorway from this room.")
		return
	}
	side, ok := sideFromDelta(g.curPos(), *cell)
	if !ok {
		return
	}
	t := g.deck.Draw()
	if t == nil {
		g.flash("Deck is empty.")
		return
	}
	g.drawn = t
	g.target = *cell
	g.entrySide = tile.OppositeSide(side)
	g.useFront = true
	g.state = stateDrawing
	g.autoOrientDrawn()
	g.flash("R rotate, click to confirm (door must face you), Esc cancel.")
}

// drawHUDButton paints a HUD-style flat button. enabled=false greys it.
func drawHUDButton(dst *ebiten.Image, r image.Rectangle, label string, hover, enabled bool) {
	bg := color.RGBA{R: 60, G: 56, B: 70, A: 220}
	border := color.RGBA{R: 220, G: 200, B: 180, A: 255}
	textCol := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	switch {
	case !enabled:
		bg = color.RGBA{R: 40, G: 40, B: 44, A: 200}
		border = color.RGBA{R: 90, G: 90, B: 90, A: 255}
		textCol = color.RGBA{R: 130, G: 130, B: 130, A: 255}
	case hover:
		bg = color.RGBA{R: 90, G: 78, B: 110, A: 230}
	}
	x, y, w, h := r.Min.X, r.Min.Y, r.Dx(), r.Dy()
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), bg, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 2, border, false)
	tx := x + w/2 - len(label)*ui.GlyphWidth/2
	ty := y + h/2 - 8
	ui.DrawColorAt(dst, label, tx, ty, textCol)
}

// roomLabel formats a tile's names for the HUD. We always include the EN
// name + id (ASCII-renderable today) and append CN when set so the same
// call site automatically gains 中文 once a CJK font face is wired in.
func roomLabel(t *tile.RoomTile) string {
	en := t.NameEN
	if en == "" {
		en = "(unnamed)"
	}
	if t.NameCN != "" {
		return fmt.Sprintf("%s / %s [#%d]", t.NameCN, en, t.ID)
	}
	return fmt.Sprintf("%s [#%d]", en, t.ID)
}

// helpers

func worldToCell(wx, wy float64) board.Cell {
	return board.Cell{
		X: int(math.Floor(wx / TileSize)),
		Y: int(math.Floor(wy / TileSize)),
	}
}

// sideFromDelta returns the side index (matching board.Directions / tile.Side*)
// from a to b when b is one of the four cardinal neighbours of a.
func sideFromDelta(a, b board.Cell) (int, bool) {
	dx, dy := b.X-a.X, b.Y-a.Y
	for i, d := range board.Directions {
		if d.X == dx && d.Y == dy {
			return i, true
		}
	}
	return 0, false
}

// autoOrientDrawn rotates the freshly drawn tile (up to three times) so that
// the entry side has a door, if any rotation can satisfy that.
func (g *GameScene) autoOrientDrawn() {
	if g.drawn == nil {
		return
	}
	for i := 0; i < 4; i++ {
		if g.drawn.HasDoor(g.entrySide) {
			return
		}
		g.drawn.RotateCW()
	}
}

func drawCellRect(dst *ebiten.Image, c board.Cell, cam ebiten.GeoM, col color.Color, stroke float32) {
	wx0 := float64(c.X) * TileSize
	wy0 := float64(c.Y) * TileSize
	sx0, sy0 := cam.Apply(wx0, wy0)
	sx1, sy1 := cam.Apply(wx0+TileSize, wy0+TileSize)
	x := float32(math.Min(sx0, sx1))
	y := float32(math.Min(sy0, sy1))
	w := float32(math.Abs(sx1 - sx0))
	h := float32(math.Abs(sy1 - sy0))
	vector.StrokeRect(dst, x, y, w, h, stroke, col, true)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// drawPawn renders a player's pawn at its current board cell. Living
// players use Char.ColorHex (red fallback when Char is nil); dead players
// render as a desaturated grey disc so they remain visible but read as
// out-of-play. Pawn rendering used to live on (*player.Player).Draw, but
// was inlined here so pkg/player can stay free of ebiten and run in CI
// without an X11 toolchain.
func drawPawn(dst *ebiten.Image, p *player.Player, tileSize float64, cam ebiten.GeoM) {
	if p == nil {
		return
	}
	wx := float64(p.Pos.X)*tileSize + tileSize/2
	wy := float64(p.Pos.Y)*tileSize + tileSize/2
	sx, sy := cam.Apply(wx, wy)
	scale := cam.Element(0, 0)
	if scale <= 0 {
		scale = 1
	}
	r := float32(tileSize * 0.18 * scale)
	outline := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	fill := p.PawnColor()
	vector.DrawFilledCircle(dst, float32(sx), float32(sy), r+2, outline, true)
	vector.DrawFilledCircle(dst, float32(sx), float32(sy), r, fill, true)
}

// rollDice replays the current dice count through the seeded Die and
// caches faces and total for drawDicePanel. Bounded to 1..8 even if
// diceCount somehow strayed out of range.
func (g *GameScene) rollDice() {
	if g.dice == nil {
		return
	}
	if g.diceCount < 1 {
		g.diceCount = 1
	}
	if g.diceCount > 8 {
		g.diceCount = 8
	}
	g.diceFaces = g.dice.RollDetail(g.diceCount)
	g.diceTotal = dice.SumOf(g.diceFaces)
}

// drawDicePanel renders the dice-tester modal: a centred panel with the
// current count, the per-die face values, and the total. Keys 1..8 to
// change the count, R/Space to re-roll, Esc/T to close.
func (g *GameScene) drawDicePanel(dst *ebiten.Image) {
	const (
		dieSize = 44
		dieGap  = 10
		padX    = 20
		padY    = 18
		titleH  = 22
		footerH = 36
	)
	rowW := g.diceCount*dieSize + (g.diceCount-1)*dieGap
	if rowW < 200 {
		rowW = 200
	}
	panelW := padX*2 + rowW
	panelH := padY*2 + titleH + dieSize + footerH
	px := g.screenW/2 - panelW/2
	py := g.screenH/2 - panelH/2

	vector.DrawFilledRect(dst, 0, 0, float32(g.screenW), float32(g.screenH),
		color.RGBA{R: 0, G: 0, B: 0, A: 120}, false)
	vector.DrawFilledRect(dst, float32(px), float32(py), float32(panelW), float32(panelH),
		color.RGBA{R: 32, G: 28, B: 38, A: 240}, false)
	vector.StrokeRect(dst, float32(px), float32(py), float32(panelW), float32(panelH), 2,
		color.RGBA{R: 220, G: 200, B: 180, A: 255}, false)

	title := fmt.Sprintf("Dice tester  (%d \u00d7 d{0,0,1,1,2,2})", g.diceCount)
	ui.DrawAt(dst, title, px+padX, py+padY)

	rowX := px + (panelW-rowW)/2
	rowY := py + padY + titleH
	for i, v := range g.diceFaces {
		x := rowX + i*(dieSize+dieGap)
		drawDieFace(dst, x, rowY, dieSize, v)
	}

	total := fmt.Sprintf("Total: %d", g.diceTotal)
	instr := "1-8 count   R / Space reroll   Esc / T close"
	ui.DrawAt(dst, total, px+padX, py+panelH-footerH+4)
	ui.DrawColorAt(dst, instr, px+panelW-len(instr)*ui.GlyphWidth-padX, py+panelH-footerH+4,
		color.RGBA{R: 200, G: 200, B: 200, A: 255})
}

// drawDieFace draws one die face at (x, y) with side length size showing
// pip count v (0/1/2). 0 is a blank face, 1 is a single centre pip, 2 is
// four corner pips (so 0/1/2 are visually distinguishable at a glance).
func drawDieFace(dst *ebiten.Image, x, y, size, v int) {
	fx, fy, fs := float32(x), float32(y), float32(size)
	vector.DrawFilledRect(dst, fx, fy, fs, fs, color.RGBA{R: 240, G: 230, B: 210, A: 255}, false)
	vector.StrokeRect(dst, fx, fy, fs, fs, 2, color.RGBA{R: 60, G: 40, B: 30, A: 255}, false)
	pip := color.RGBA{R: 30, G: 24, B: 20, A: 255}
	r := fs * 0.09
	cx, cy := fx+fs/2, fy+fs/2
	offset := fs * 0.28
	switch v {
	case 1:
		vector.DrawFilledCircle(dst, cx, cy, r, pip, true)
	case 2:
		vector.DrawFilledCircle(dst, cx-offset, cy-offset, r, pip, true)
		vector.DrawFilledCircle(dst, cx+offset, cy+offset, r, pip, true)
		vector.DrawFilledCircle(dst, cx-offset, cy+offset, r, pip, true)
		vector.DrawFilledCircle(dst, cx+offset, cy-offset, r, pip, true)
	}
}

// cardDialog holds the modal-card state for roadmap stage 4. nil when
// no card is being shown. The card stays in the dialog until the
// player closes it (Apply or Manual); on close it is sent to the
// owning deck's discard pile.
type cardDialog struct {
	card        *cards.Card
	deck        *cards.Deck // owning deck (for discard on close)
	appliedLogs []string    // populated after Apply
	applied     bool
	kept        bool   // true when card moved to inventory (skip discard)
	hauntInfo   string // populated for omen draws
	hauntFire   bool   // true when this draw triggered the haunt
}

// triggerRoomCard inspects rooms_doc.yaml for the freshly-revealed
// room and, if it has an event/item/omen icon, opens a card modal.
// Safe to call with an empty name; missing decks degrade silently.
func (g *GameScene) triggerRoomCard(nameCN string) {
	if nameCN == "" {
		return
	}
	switch assets.RoomKindOf(nameCN) {
	case "event":
		g.openCard(g.eventDeck)
	case "item":
		g.openCard(g.itemDeck)
	case "omen":
		g.openCard(g.omenDeck)
	}
}

// openCard pops the top card off d, runs the per-kind side effects
// (Haunt Roll for omens) and switches the scene into the card-modal
// state. No-op when d is nil or empty.
func (g *GameScene) openCard(d *cards.Deck) {
	if d == nil {
		return
	}
	c := d.Draw()
	if c == nil {
		g.flash(fmt.Sprintf("Deck is empty: %s", d.Kind()))
		return
	}
	dlg := &cardDialog{card: c, deck: d}
	if c.Kind == cards.KindOmen && g.haunt != nil {
		triggered := g.haunt.HauntRoll(g.dice)
		sum, n := g.haunt.LastRoll()
		if triggered {
			dlg.hauntFire = true
			dlg.hauntInfo = fmt.Sprintf("HAUNT TRIGGERED!  Roll %d on %dd  <  %d omens",
				sum, n, g.haunt.OmenCount())
		} else {
			dlg.hauntInfo = fmt.Sprintf("Haunt Roll: %d on %dd   (need < %d) \u2014 safe",
				sum, n, g.haunt.OmenCount())
		}
	}
	g.cardDlg = dlg
}

// applyCard resolves the open card's Effects, captures the result
// strings for the dialog body and leaves the modal open so the player
// can read what happened before pressing M / Esc.
func (g *GameScene) applyCard() {
	if g.cardDlg == nil || g.cardDlg.applied {
		return
	}
	logs := cards.Apply(g.cardDlg.card, g.cur(), g.players)
	g.cardDlg.appliedLogs = logs
	g.cardDlg.applied = true
	if len(logs) > 0 {
		g.flash(strings.Join(logs, "   "))
	}
}

// closeCard discards the open card to its owning pile, prints a haunt
// hint when the draw triggered, and returns to normal play.
// For held cards (items/omens) that were kept via keepCard, the
// discard is skipped since the card already moved to inventory.
func (g *GameScene) closeCard() {
	dlg := g.cardDlg
	if dlg == nil {
		return
	}
	if !dlg.kept && dlg.deck != nil {
		dlg.deck.Discard(dlg.card)
	}
	g.cardDlg = nil
	if dlg.hauntFire {
		// Stage 5 hook: scenarioID lookup + briefing dispatch.
		// For now we just flash and log to console.
		g.flash(fmt.Sprintf("\u26A0  HAUNT TRIGGERED at omen %d \u2014 scenario lookup pending (stage 5)", g.haunt.OmenCount()))
		fmt.Printf("[HAUNT] omen %d triggered haunt for player %s\n",
			g.haunt.OmenCount(), playerLabel(g.cur()))
	}
}

// keepCard moves the open card into the current player's inventory,
// marks it as kept (so closeCard won't discard it), and closes the
// dialog.
func (g *GameScene) keepCard() {
	dlg := g.cardDlg
	if dlg == nil || dlg.kept {
		return
	}
	dlg.kept = true
	if cur := g.cur(); cur != nil {
		g.addToInventory(cur, dlg.card)
	}
	g.closeCard()
}

// drawCardDialog renders the modal: a fullscreen dimmer plus a centred
// panel with the card's title, body text, optional Haunt-Roll line and
// Apply / Manual hints.
func (g *GameScene) drawCardDialog(dst *ebiten.Image) {
	if g.cardDlg == nil || g.cardDlg.card == nil {
		return
	}
	dlg := g.cardDlg
	c := dlg.card

	// Full-screen dim.
	vector.DrawFilledRect(dst, 0, 0, float32(g.screenW), float32(g.screenH),
		color.RGBA{R: 0, G: 0, B: 0, A: 180}, false)

	// Card panel rect (centred, ~480x360).
	panelW, panelH := 480, 360
	px := (g.screenW - panelW) / 2
	py := (g.screenH - panelH) / 2

	// Background tinted by kind.
	var bg color.RGBA
	switch c.Kind {
	case cards.KindEvent:
		bg = color.RGBA{R: 28, G: 22, B: 60, A: 255}
	case cards.KindItem:
		bg = color.RGBA{R: 22, G: 50, B: 28, A: 255}
	case cards.KindOmen:
		bg = color.RGBA{R: 60, G: 18, B: 22, A: 255}
	default:
		bg = color.RGBA{R: 32, G: 32, B: 38, A: 255}
	}
	vector.DrawFilledRect(dst, float32(px), float32(py),
		float32(panelW), float32(panelH), bg, false)
	vector.StrokeRect(dst, float32(px), float32(py),
		float32(panelW), float32(panelH), 2,
		color.RGBA{R: 220, G: 200, B: 140, A: 255}, false)

	// Top: kind badge.
	lh := ui.LineHeight()
	kindLabel := "EVENT"
	switch c.Kind {
	case cards.KindItem:
		kindLabel = "ITEM"
	case cards.KindOmen:
		kindLabel = "OMEN"
	}
	ui.DrawColorAt(dst, kindLabel, px+12, py+10, color.RGBA{R: 255, G: 215, B: 100, A: 255})

	// Title.
	ui.DrawAt(dst, c.Label(), px+12, py+10+lh)

	// Body, wrapped to ~36 runes per line.
	bodyY := py + 14 + lh*2 + 4
	for _, line := range wrapText(c.Text, 36) {
		ui.DrawAt(dst, line, px+12, bodyY)
		bodyY += lh
	}

	// Haunt info, if any (omen draws only).
	if dlg.hauntInfo != "" {
		bodyY += 4
		clr := color.RGBA{R: 240, G: 240, B: 200, A: 255}
		if dlg.hauntFire {
			clr = color.RGBA{R: 255, G: 90, B: 90, A: 255}
		}
		ui.DrawColorAt(dst, dlg.hauntInfo, px+12, bodyY, clr)
		bodyY += lh
	}

	// Applied logs.
	if dlg.applied {
		bodyY += 6
		ui.DrawColorAt(dst, "APPLIED:", px+12, bodyY,
			color.RGBA{R: 180, G: 240, B: 180, A: 255})
		bodyY += lh
		for _, l := range dlg.appliedLogs {
			for _, sub := range wrapText(l, 36) {
				ui.DrawAt(dst, sub, px+24, bodyY)
				bodyY += lh
			}
		}
	}

	// Bottom hint bar.
	var hint string
	switch {
	case c.Kind == cards.KindEvent:
		hint = "[A] Apply   [M] Manual / Close   [Esc] Discard"
		if dlg.applied {
			hint = "[M] / [Esc] Discard"
		}
	case c.Kind == cards.KindItem:
		hint = "[K/Esc] Keep in bag   [U] Use & discard"
	case c.Kind == cards.KindOmen:
		hint = "[A] Apply effects   [K/Esc] Keep (omens always held)"
		if dlg.applied {
			hint = "[K/Esc] Keep"
		}
	default:
		hint = "[Esc] Close"
	}
	ui.DrawColorAt(dst, hint, px+12, py+panelH-lh-8,
		color.RGBA{R: 200, G: 200, B: 200, A: 255})
}

// wrapText breaks s into lines of at most maxRunes runes each. The
// split is rune-aware so CJK characters count correctly.
func wrapText(s string, maxRunes int) []string {
	if maxRunes <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return []string{""}
	}
	out := make([]string, 0, (len(runes)+maxRunes-1)/maxRunes)
	for i := 0; i < len(runes); i += maxRunes {
		end := i + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}

// ---------- inventory helpers (game-level, avoids player→cards cycle) ----------

// playerInventory returns the held cards for p.
func (g *GameScene) playerInventory(p *player.Player) []*cards.Card {
	if p == nil {
		return nil
	}
	return g.inventory[p.ID]
}

// addToInventory appends c to p's held list.
func (g *GameScene) addToInventory(p *player.Player, c *cards.Card) {
	if p == nil || c == nil {
		return
	}
	g.inventory[p.ID] = append(g.inventory[p.ID], c)
}

// removeFromInventory removes the first occurrence of c from p's held list.
func (g *GameScene) removeFromInventory(p *player.Player, c *cards.Card) {
	if p == nil || c == nil {
		return
	}
	inv := g.inventory[p.ID]
	for i, held := range inv {
		if held == c {
			g.inventory[p.ID] = append(inv[:i], inv[i+1:]...)
			return
		}
	}
}

// dropInventory moves all of p's held cards to the floor at p.Pos.
func (g *GameScene) dropInventory(p *player.Player) {
	if p == nil {
		return
	}
	inv := g.inventory[p.ID]
	if len(inv) == 0 {
		return
	}
	g.droppedItems[p.Pos] = append(g.droppedItems[p.Pos], inv...)
	delete(g.inventory, p.ID)
}

// pickupDropped transfers dropped cards at p.Pos into p's inventory.
// Returns true when at least one card was picked up.
func (g *GameScene) pickupDropped(p *player.Player) bool {
	if p == nil {
		return false
	}
	dropped := g.droppedItems[p.Pos]
	if len(dropped) == 0 {
		return false
	}
	g.inventory[p.ID] = append(g.inventory[p.ID], dropped...)
	delete(g.droppedItems, p.Pos)
	return true
}

// onTurnStart runs per-turn effects for all held cards of the current
// player. Call just after BeginTurn so the effects fire before the
// player moves.
func (g *GameScene) onTurnStart(p *player.Player) {
	if p == nil || p.Dead {
		return
	}
	for _, c := range g.playerInventory(p) {
		perTurn := cards.PerTurnEffects(c)
		if len(perTurn) == 0 {
			continue
		}
		var logs []string
		for _, eff := range perTurn {
			effLogs := cards.Apply(&cards.Card{Effects: []cards.Effect{eff}}, p, g.players)
			logs = append(logs, effLogs...)
		}
		if len(logs) > 0 {
			g.flash(fmt.Sprintf("[%s] %s", c.Label(), strings.Join(logs, "; ")))
		}
		if p.Dead {
			g.dropInventory(p)
			g.flash(playerLabel(p) + " has died from held card effects.")
			break
		}
	}
}

// equipBonus returns the total combat/roll bonus from all held cards
// for the given stat. Returns 0 until the combat system is built.
func (g *GameScene) equipBonus(p *player.Player, stat cards.Stat) int {
	total := 0
	for _, c := range g.playerInventory(p) {
		total += cards.EquipBonus(c, stat)
	}
	return total
}

// drawInventoryPanel renders a compact list of held cards below the
// stat panel in the bottom-left corner.
func (g *GameScene) drawInventoryPanel(dst *ebiten.Image) {
	cur := g.cur()
	if cur == nil {
		return
	}
	inv := g.playerInventory(cur)
	if len(inv) == 0 {
		return
	}
	lh := ui.LineHeight()

	// position: below stat panel (approx y = screenH - 28 - 120 - 6 - len*lh)
	panelH := len(inv)*lh + 8 + lh // header + items
	px := 10
	py := g.screenH - 28 - 120 - 6 - panelH
	panelW := 200

	vector.DrawFilledRect(dst, float32(px), float32(py),
		float32(panelW), float32(panelH),
		color.RGBA{R: 10, G: 10, B: 20, A: 180}, false)
	vector.StrokeRect(dst, float32(px), float32(py),
		float32(panelW), float32(panelH), 1,
		color.RGBA{R: 120, G: 120, B: 140, A: 200}, false)

	ui.DrawColorAt(dst, fmt.Sprintf("Bag (%d)", len(inv)), px+6, py+4,
		color.RGBA{R: 200, G: 180, B: 100, A: 255})
	y := py + 4 + lh
	for _, c := range inv {
		var kindTag string
		switch c.Kind {
		case cards.KindItem:
			kindTag = "[I]"
		case cards.KindOmen:
			kindTag = "[O]"
		default:
			kindTag = "[E]"
		}
		label := kindTag + " " + c.Label()
		if len([]rune(label)) > 28 {
			label = string([]rune(label)[:28]) + ".."
		}
		ui.DrawAt(dst, label, px+6, y)
		y += lh
	}
}

// drawCurrentPawnRing draws a thick gold ring around the active player's
// pawn so it stands out from the others on the same cell.
func drawCurrentPawnRing(dst *ebiten.Image, p *player.Player, tileSize float64, cam ebiten.GeoM) {
	if p == nil {
		return
	}
	wx := float64(p.Pos.X)*tileSize + tileSize/2
	wy := float64(p.Pos.Y)*tileSize + tileSize/2
	sx, sy := cam.Apply(wx, wy)
	scale := cam.Element(0, 0)
	if scale <= 0 {
		scale = 1
	}
	r := float32(tileSize * 0.18 * scale)
	gold := color.RGBA{R: 255, G: 210, B: 60, A: 255}
	vector.StrokeCircle(dst, float32(sx), float32(sy), r+5, 2.5, gold, true)
}
