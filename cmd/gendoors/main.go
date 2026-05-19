// Command gendoors scans the assets/image/* sheets, detects the yellow door
// brackets on each tile, and writes assets/datafs/tile_meta.yaml so the game
// can consume door / floor / name metadata at runtime without rescanning
// pixels.
//
// Run from the project root:
//
//	go run ./cmd/gendoors
//
// The yaml is the single source of truth: hand-edited fields name_cn,
// name_en and start_cell are preserved across regenerations; geometric
// fields (doors, floors, row, col, source) are always overwritten.
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// --- detection constants (must match what the game gameplay was tuned with) ---

const (
	doorBandRatio   = 0.25
	doorYellowRatio = 0.004

	// Floor labels sit in the central column between the two window
	// stacks. The band height is intentionally generous so anti-aliased
	// edges of the label box still contribute enough yellow pixels.
	floorBandHRatio  = 0.06
	floorYellowRatio = 0.10
)

// floorCenters lists the vertical centers of each floor label as fractions
// of the cell height. Index order matches tile.Floor* constants:
// [0] Roof, [1] Upper, [2] Ground, [3] Basement. The values were calibrated
// against main_map_back.jpg / extension_map_back.jpg by sampling the yellow
// ratio along the central column of each cell.
var floorCenters = [4]float64{0.25, 0.40, 0.55, 0.70}

// isDoorYellow matches the door bracket color used in the artwork. The brackets
// are not pure (255,255,0) yellow; many pixels (especially anti-aliased edges
// and JPEG-compressed regions) sit around (213,166,0) — a saturated
// orange-yellow. We accept any pixel that is bright in red+green and very low
// in blue, while still rejecting the brown wood floor whose red channel is
// well below 200.
func isDoorYellow(r8, g8, b8 int) bool {
	return r8 > 200 && g8 > 150 && b8 < 80 && r8-b8 > 100
}

func bandYellowRatio(img image.Image, x0, y0, x1, y1 int) float64 {
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	yellow, total := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if isDoorYellow(int(r>>8), int(g>>8), int(b>>8)) {
				yellow++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(yellow) / float64(total)
}

// detectFloors examines a back-image cell and returns which floor markers
// are highlighted in yellow. Index order matches tile.Floor* constants:
// [0] Roof, [1] Upper, [2] Ground, [3] Basement.
func detectFloors(img image.Image, rect image.Rectangle) [4]bool {
	w := rect.Dx()
	h := rect.Dy()
	cx0 := rect.Min.X + w*30/100
	cx1 := rect.Min.X + w*70/100
	bandH := int(float64(h) * floorBandHRatio)
	if bandH < 4 {
		bandH = 4
	}
	var floors [4]bool
	for i, c := range floorCenters {
		cy := rect.Min.Y + int(float64(h)*c)
		y0 := cy - bandH/2
		y1 := cy + bandH/2
		if y0 < rect.Min.Y {
			y0 = rect.Min.Y
		}
		if y1 > rect.Max.Y {
			y1 = rect.Max.Y
		}
		if bandYellowRatio(img, cx0, y0, cx1, y1) > floorYellowRatio {
			floors[i] = true
		}
	}
	return floors
}

func anyFloor(f [4]bool) bool {
	for _, v := range f {
		if v {
			return true
		}
	}
	return false
}

func detectDoors(img image.Image, rect image.Rectangle) [4]bool {
	w := rect.Dx()
	h := rect.Dy()
	bandX := int(float64(w) * doorBandRatio)
	if bandX < 4 {
		bandX = 4
	}
	bandY := int(float64(h) * doorBandRatio)
	if bandY < 4 {
		bandY = 4
	}
	var doors [4]bool
	doors[0] = bandYellowRatio(img, rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+bandY) > doorYellowRatio
	doors[1] = bandYellowRatio(img, rect.Max.X-bandX, rect.Min.Y, rect.Max.X, rect.Max.Y) > doorYellowRatio
	doors[2] = bandYellowRatio(img, rect.Min.X, rect.Max.Y-bandY, rect.Max.X, rect.Max.Y) > doorYellowRatio
	doors[3] = bandYellowRatio(img, rect.Min.X, rect.Min.Y, rect.Min.X+bandX, rect.Max.Y) > doorYellowRatio
	return doors
}

func isBlank(img image.Image, rect image.Rectangle) bool {
	step := rect.Dx() / 16
	if step < 1 {
		step = 1
	}
	nonBlack := 0
	for y := rect.Min.Y; y < rect.Max.Y; y += step {
		for x := rect.Min.X; x < rect.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			if int(r>>8)+int(g>>8)+int(bl>>8) > 30 {
				nonBlack++
				if nonBlack > 4 {
					return false
				}
			}
		}
	}
	return true
}

func anyDoor(d [4]bool) bool {
	for _, v := range d {
		if v {
			return true
		}
	}
	return false
}

// --- sheet processing ---

type entry struct {
	ID     int
	Source string // "starter" / "base" / "extension"
	Row    int
	Col    int
	Note   string
	Doors  [4]bool
	Floors [4]bool
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode %s: %v\n", path, err)
		os.Exit(1)
	}
	return img
}

func cellRect(img image.Image, cols, rows, c, r int) image.Rectangle {
	b := img.Bounds()
	tw := b.Dx() / cols
	th := b.Dy() / rows
	x0 := b.Min.X + c*tw
	y0 := b.Min.Y + r*th
	return image.Rect(x0, y0, x0+tw, y0+th)
}

// forceFn lets a sheet-specific tweak adjust both doors and floors after
// auto-detection (e.g. the staircase tile that has no painted door bracket).
type forceFn func(idx int, doors *[4]bool, floors *[4]bool, note *string)

func processSheet(frontPath, backPath string, cols, rows, idBase int, kind string, out *[]entry, defaultFloors [4]bool, force forceFn) {
	img := loadImage(frontPath)
	var back image.Image
	if backPath != "" {
		back = loadImage(backPath)
	}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			rect := cellRect(img, cols, rows, c, r)
			if isBlank(img, rect) {
				continue
			}
			d := detectDoors(img, rect)
			f := defaultFloors
			if back != nil {
				bRect := cellRect(back, cols, rows, c, r)
				if !isBlank(back, bRect) {
					f = detectFloors(back, bRect)
				}
			}
			note := fmt.Sprintf("%s r%d c%d", kind, r, c)
			if force != nil {
				force(idx, &d, &f, &note)
			}
			if !anyDoor(d) {
				continue
			}
			*out = append(*out, entry{
				ID: idBase + idx, Source: kind, Row: r, Col: c,
				Doors: d, Floors: f, Note: note,
			})
		}
	}
}

func main() {
	yamlPath := flag.String("yaml", "assets/datafs/tile_meta.yaml",
		"path of the canonical yaml dump; pass empty string to skip")
	flag.Parse()

	var out []entry

	// The 3 starter tiles are always placed on the ground floor.
	starterFloors := [4]bool{false, false, true, false}
	processSheet("assets/image/entry_hall.png", "", 3, 1, 1000, "starter", &out, starterFloors,
		func(idx int, d *[4]bool, _ *[4]bool, note *string) {
			// The Grand Staircase tile (idx 0) has no yellow brackets in
			// the artwork; its visible staircase implies a south door.
			if idx == 0 && !anyDoor(*d) {
				d[2] = true
				*note += " (manual: down door from staircase)"
			}
		})

	processSheet("assets/image/main_map.jpg", "assets/image/main_map_back.jpg",
		10, 5, 0, "base", &out, [4]bool{},
		func(idx int, _ *[4]bool, f *[4]bool, note *string) {
			// The two anchor rooms ("上层" and "地下室") have a blank
			// back artwork because their floor is implied by the room
			// identity itself; pin them manually.
			switch idx {
			case 0: // r0 c0 = "上层" (Upper Floor) anchor tile
				f[1] = true
				*note += " (manual: upper-floor anchor)"
			case 1: // r0 c1 = "地下室" (Basement) anchor tile
				f[3] = true
				*note += " (manual: basement anchor)"
			}
		})
	processSheet("assets/image/extension_map.jpg", "assets/image/extension_map_back.jpg",
		10, 2, 500, "extension", &out, [4]bool{}, nil)

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	if *yamlPath == "" {
		fmt.Fprintln(os.Stderr, "gendoors: -yaml=\"\" passed; nothing to write")
		return
	}
	if err := writeTileMetaYAML(*yamlPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "write yaml %s: %v\n", *yamlPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "gendoors: wrote %d entries to %s\n", len(out), *yamlPath)
}

// writeTileMetaYAML serialises the gen results to a hand-editable yaml file.
// The format is intentionally small and stable so reviewers can spot drift in
// version control diffs; it is hand-written (no yaml.v3 dependency in this
// command) so cmd/gendoors stays free of non-stdlib deps.
func writeTileMetaYAML(path string, entries []entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Auto-generated by cmd/gendoors. The geometric fields\n")
	b.WriteString("# (id/source/row/col/doors/floors/note) are overwritten on every\n")
	b.WriteString("# regeneration. The text fields (name_cn/name_en) are preserved\n")
	b.WriteString("# across regenerations and are intended for human editing.\n")
	b.WriteString("# DO NOT manually edit the geometric fields — change the source\n")
	b.WriteString("# images instead.\n")
	b.WriteString("tiles:\n")

	existing := loadExistingTileMetaText(path)

	for _, e := range entries {
		prev := existing[e.ID]
		nameCN, nameEN := prev.nameCN, prev.nameEN
		fmt.Fprintf(&b, "  - id: %d\n", e.ID)
		fmt.Fprintf(&b, "    source: %s\n", e.Source)
		fmt.Fprintf(&b, "    row: %d\n", e.Row)
		fmt.Fprintf(&b, "    col: %d\n", e.Col)
		if prev.hasCell {
			fmt.Fprintf(&b, "    start_cell: [%d, %d]\n", prev.cellX, prev.cellY)
		}
		fmt.Fprintf(&b, "    doors:  [%t, %t, %t, %t]\n",
			e.Doors[0], e.Doors[1], e.Doors[2], e.Doors[3])
		fmt.Fprintf(&b, "    floors: [%t, %t, %t, %t]\n",
			e.Floors[0], e.Floors[1], e.Floors[2], e.Floors[3])
		fmt.Fprintf(&b, "    name_cn: %s\n", yamlQuote(nameCN))
		fmt.Fprintf(&b, "    name_en: %s\n", yamlQuote(nameEN))
		if e.Note != "" {
			fmt.Fprintf(&b, "    note: %s\n", yamlQuote(e.Note))
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// yamlQuote emits a value as a YAML double-quoted string. Empty strings render
// as `""` instead of bare so loaders never see them as nil.
func yamlQuote(s string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// loadExistingTileMetaText reads the previously generated yaml file (if any)
// and returns id -> {name_cn, name_en, start_cell} so human edits survive
// regeneration. Parsing is deliberately tiny and forgiving: only the lines
// we care about are matched; everything else is ignored.
func loadExistingTileMetaText(path string) map[int]preservedTileMeta {
	out := map[int]preservedTileMeta{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	lines := strings.Split(string(data), "\n")
	curID := -1
	var cur preservedTileMeta
	flush := func() {
		if curID >= 0 {
			out[curID] = cur
		}
		curID = -1
		cur = preservedTileMeta{}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "- id:"):
			flush()
			var id int
			if _, err := fmt.Sscanf(line, "- id: %d", &id); err == nil {
				curID = id
			}
		case strings.HasPrefix(line, "name_cn:"):
			cur.nameCN = parseYAMLQuoted(strings.TrimSpace(strings.TrimPrefix(line, "name_cn:")))
		case strings.HasPrefix(line, "name_en:"):
			cur.nameEN = parseYAMLQuoted(strings.TrimSpace(strings.TrimPrefix(line, "name_en:")))
		case strings.HasPrefix(line, "start_cell:"):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "start_cell:"))
			if x, y, ok := parseFlowIntPair(rest); ok {
				cur.cellX, cur.cellY, cur.hasCell = x, y, true
			}
		}
	}
	flush()
	return out
}

// preservedTileMeta carries the human-edited fields that must survive a
// regeneration of tile_meta.yaml.
type preservedTileMeta struct {
	nameCN  string
	nameEN  string
	cellX   int
	cellY   int
	hasCell bool
}

// parseFlowIntPair extracts (x, y) from a YAML flow-style int pair such as
// "[0, -2]".
func parseFlowIntPair(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return 0, 0, false
	}
	inner := s[1 : len(s)-1]
	parts := strings.Split(inner, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	y, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	return x, y, true
}

func parseYAMLQuoted(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		inner = strings.ReplaceAll(inner, `\n`, "\n")
		inner = strings.ReplaceAll(inner, `\t`, "\t")
		inner = strings.ReplaceAll(inner, `\r`, "\r")
		return inner
	}
	return s
}
