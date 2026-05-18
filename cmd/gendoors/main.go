// Command gendoors scans the assets/image/* sheets, detects the yellow door
// brackets on each tile, and emits assets/doors_gen.go holding the result so
// the game does not need to rescan pixels at runtime.
//
// Run from the project root:
//
//	go run ./cmd/gendoors > assets/doors_gen.go
package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sort"
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
			*out = append(*out, entry{ID: idBase + idx, Doors: d, Floors: f, Note: note})
		}
	}
}

func main() {
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

	fmt.Println("// Code generated by cmd/gendoors. DO NOT EDIT.")
	fmt.Println("// Run `go run ./cmd/gendoors > assets/doors_gen.go` to regenerate.")
	fmt.Println()
	fmt.Println("package assets")
	fmt.Println()
	fmt.Println("// tileMeta maps each valid tile ID to its door layout and allowed")
	fmt.Println("// floors at rotation 0.")
	fmt.Println("//   Doors  side order: [up, right, down, left].")
	fmt.Println("//   Floors order:      [roof, upper, ground, basement].")
	fmt.Println("// IDs not present in this map are blank artwork cells or had no")
	fmt.Println("// detectable doors and are skipped.")
	fmt.Println("var tileMeta = map[int]TileMeta{")
	for _, e := range out {
		fmt.Printf("\t%-5d: {Doors: [4]bool{%-5v, %-5v, %-5v, %-5v}, Floors: [4]bool{%-5v, %-5v, %-5v, %-5v}}, // %s\n",
			e.ID,
			e.Doors[0], e.Doors[1], e.Doors[2], e.Doors[3],
			e.Floors[0], e.Floors[1], e.Floors[2], e.Floors[3],
			e.Note)
	}
	fmt.Println("}")
}
