package scene

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/plasmolysismango/houseonthehill/assets"
	"github.com/plasmolysismango/houseonthehill/pkg/board"
	"github.com/plasmolysismango/houseonthehill/pkg/camera"
	"github.com/plasmolysismango/houseonthehill/pkg/component"
	"github.com/plasmolysismango/houseonthehill/pkg/deck"
	"github.com/plasmolysismango/houseonthehill/pkg/player"
	"github.com/plasmolysismango/houseonthehill/pkg/tile"
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
	player *player.Player

	state     gameState
	drawn     *tile.RoomTile
	target    board.Cell
	entrySide int  // world side of the drawn tile that must face the player
	useFront  bool // toggle whether to flip drawn tile preview face up

	// click vs drag detection
	pressX, pressY int
	dragged        bool

	// HUD draw-room button (recomputed every frame in drawHUD).
	drawBtn image.Rectangle

	// transient hint
	hintMsg     string
	hintExpires time.Time
}

// NewGameScene loads assets and lays down the starter rooms.
func NewGameScene(useExtension bool, screenW, screenH int) (*GameScene, error) {
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
		screenW: screenW,
		screenH: screenH,
		mouse:   &utils.Mouse{},
		cam:     camera.New(screenW, screenH),
		board:   board.New[*component.Room](TileSize),
		deck:    deck.New(allTiles, time.Now().UnixNano()),
		state:   stateIdle,
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
	g.player = player.New(spawn)

	// centre camera on player's tile centre
	g.cam.CenterOn(float64(g.player.Pos.X)*TileSize+TileSize/2,
		float64(g.player.Pos.Y)*TileSize+TileSize/2)
	// fit comfortably: scale so a tile is about 200px on screen
	g.cam.Scale = 200.0 / TileSize

	g.flash("Click an adjacent cell to move or draw a room. Or press D / click Draw.")
	return g, nil
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

	switch g.state {
	case stateIdle:
		if clicked && board.IsAdjacent(g.player.Pos, hover) {
			side, ok := sideFromDelta(g.player.Pos, hover)
			if !ok {
				break
			}
			curRoom, _ := g.board.At(g.player.Pos)
			if curRoom == nil || !curRoom.Tile.HasDoor(side) {
				g.flash("No door on that side of this room.")
				break
			}
			if room, ok := g.board.At(hover); ok {
				if !room.Tile.HasDoor(tile.OppositeSide(side)) {
					g.flash("The next room has no door facing here.")
					break
				}
				g.player.Pos = hover
				g.flash("Moved.")
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
				g.player.Pos = g.target
				g.drawn = nil
				g.state = stateIdle
				g.flash("Room placed.")
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
	curRoom, _ := g.board.At(g.player.Pos)
	for i, n := range g.board.Neighbors(g.player.Pos) {
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

	// 5. player pawn
	g.player.Draw(screen, TileSize, camGeo)

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
	hud := fmt.Sprintf("State: %s   Deck: %d left   Player: (%d,%d)   Rooms: %d",
		state, g.deck.Remaining(), g.player.Pos.X, g.player.Pos.Y, len(g.board.All()))
	ebitenutil.DebugPrintAt(screen, hud, 8, 6)

	// hover-room line: shows the names from tile_meta.yaml. CJK glyphs
	// cannot be rendered by the built-in debug font; once a CJK face is
	// wired in (roadmap stage 8) the same string will show 中文.
	cx, cy := ebiten.CursorPosition()
	wx, wy := g.cam.ScreenToWorld(cx, cy)
	if room, ok := g.board.At(worldToCell(wx, wy)); ok && room != nil {
		ebitenutil.DebugPrintAt(screen, "Hover: "+roomLabel(room.Tile), 8, 48)
	}

	// Draw-room button (top-right). Only enabled in stateIdle so the user
	// can't accidentally throw away the in-flight tile while orienting it.
	g.layoutDrawButton()
	hover := image.Point{X: cx, Y: cy}.In(g.drawBtn)
	enabled := g.state == stateIdle && g.deck.Remaining() > 0 && g.drawCandidate() != nil
	label := fmt.Sprintf("Draw [D] (%d)", g.deck.Remaining())
	drawHUDButton(screen, g.drawBtn, label, hover, enabled)

	// bottom hint
	if time.Now().Before(g.hintExpires) && g.hintMsg != "" {
		vector.DrawFilledRect(screen, 0, float32(g.screenH-28), float32(g.screenW), 28, color.RGBA{R: 0, G: 0, B: 0, A: 160}, false)
		ebitenutil.DebugPrintAt(screen, g.hintMsg, 8, g.screenH-22)
	}

	// help line
	help := "L-click adj cell: move/draw   D / Draw button: pick a yellow neighbour   Drag: pan   Wheel: zoom   Mid-click: reset   In drawing: R rotate, F flip, Esc cancel"
	vector.DrawFilledRect(screen, 0, 28, float32(g.screenW), 18, color.RGBA{R: 0, G: 0, B: 0, A: 120}, false)
	ebitenutil.DebugPrintAt(screen, help, 8, 30)
}

// layoutDrawButton recomputes the HUD draw-button rectangle. Called every
// frame so window resizes are picked up automatically.
func (g *GameScene) layoutDrawButton() {
	const bw, bh = 180, 32
	x := g.screenW - bw - 12
	y := 4
	g.drawBtn = image.Rect(x, y, x+bw, y+bh)
}

// drawCandidate returns the first cardinal neighbour of the player that has
// a door (from the player's room) AND is currently empty—i.e. exactly the
// cell that would yellow-highlight in Draw(). Returns nil when no candidate
// exists (e.g. all neighbours are walls or already filled).
func (g *GameScene) drawCandidate() *board.Cell {
	curRoom, _ := g.board.At(g.player.Pos)
	if curRoom == nil {
		return nil
	}
	for i, n := range g.board.Neighbors(g.player.Pos) {
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
	cell := g.drawCandidate()
	if cell == nil {
		g.flash("No empty doorway from this room.")
		return
	}
	side, ok := sideFromDelta(g.player.Pos, *cell)
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
	tx := x + w/2 - len(label)*6/2
	ty := y + h/2 - 8
	_ = textCol // ebitenutil prints in white; kept for future custom font
	ebitenutil.DebugPrintAt(dst, label, tx, ty)
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
