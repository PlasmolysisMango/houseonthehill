package scene

import (
	"fmt"
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

	state    gameState
	drawn    *tile.RoomTile
	target   board.Cell
	useFront bool // toggle whether to flip drawn tile preview face up

	// click vs drag detection
	pressX, pressY int
	dragged        bool

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

	// place starter rooms horizontally at y=0, columns 0..len-1
	for i, t := range starters {
		g.board.Place(board.Cell{X: i, Y: 0}, component.NewRoom(t))
	}
	g.player = player.New(board.Cell{X: 0, Y: 0})

	// centre camera on player's tile centre
	g.cam.CenterOn(float64(g.player.Pos.X)*TileSize+TileSize/2,
		float64(g.player.Pos.Y)*TileSize+TileSize/2)
	// fit comfortably: scale so a tile is about 200px on screen
	g.cam.Scale = 200.0 / TileSize

	g.flash("Click an adjacent cell to move or draw a room.")
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

	switch g.state {
	case stateIdle:
		if clicked && board.IsAdjacent(g.player.Pos, hover) {
			if _, ok := g.board.At(hover); ok {
				g.player.Pos = hover
				g.flash("Moved.")
			} else {
				if t := g.deck.Draw(); t != nil {
					g.drawn = t
					g.target = hover
					g.useFront = true
					g.state = stateDrawing
					g.flash("Press R to rotate, click to confirm, Esc to cancel.")
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
			room := component.NewRoom(g.drawn)
			room.Revealed = true
			g.board.Place(g.target, room)
			g.player.Pos = g.target
			g.drawn = nil
			g.state = stateIdle
			g.flash("Room placed.")
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

	// 2. neighbour highlights from the player's cell
	for _, n := range g.board.Neighbors(g.player.Pos) {
		var col color.Color
		if _, ok := g.board.At(n); ok {
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

	// bottom hint
	if time.Now().Before(g.hintExpires) && g.hintMsg != "" {
		vector.DrawFilledRect(screen, 0, float32(g.screenH-28), float32(g.screenW), 28, color.RGBA{R: 0, G: 0, B: 0, A: 160}, false)
		ebitenutil.DebugPrintAt(screen, g.hintMsg, 8, g.screenH-22)
	}

	// help line
	help := "L-click adj cell: move/draw   Drag: pan   Wheel: zoom   Mid-click: reset   In drawing: R rotate, F flip, Esc cancel"
	vector.DrawFilledRect(screen, 0, 28, float32(g.screenW), 18, color.RGBA{R: 0, G: 0, B: 0, A: 120}, false)
	ebitenutil.DebugPrintAt(screen, help, 8, 30)
}

// helpers

func worldToCell(wx, wy float64) board.Cell {
	return board.Cell{
		X: int(math.Floor(wx / TileSize)),
		Y: int(math.Floor(wy / TileSize)),
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
