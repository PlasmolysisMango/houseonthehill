package data

import (
	"testing"

	"github.com/plasmolysismango/houseonthehill/assets/datafs"
)

func TestLoadTiles(t *testing.T) {
	tiles, err := LoadTiles(datafs.FS)
	if err != nil {
		t.Fatalf("LoadTiles: %v", err)
	}
	if len(tiles) < 60 {
		t.Fatalf("expect at least 60 tiles, got %d", len(tiles))
	}

	// Two anchor tiles must be tagged with their respective floors. These
	// are the canary cases for the manual-force overrides in cmd/gendoors.
	var upperOK, basementOK, starterGroundOK bool
	for _, x := range tiles {
		switch x.ID {
		case 0:
			if x.Floors[1] {
				upperOK = true
			}
		case 1:
			if x.Floors[3] {
				basementOK = true
			}
		case 1000, 1001, 1002:
			if x.Floors[2] {
				starterGroundOK = true
			}
		}
	}
	if !upperOK {
		t.Error("tile 0 should allow Upper floor")
	}
	if !basementOK {
		t.Error("tile 1 should allow Basement floor")
	}
	if !starterGroundOK {
		t.Error("starter tiles 1000-1002 should allow Ground floor")
	}

	// start_cell canary: the three starter tiles must each carry a board
	// cell so pkg/scene/game.go can lay them out without hard-coding
	// coordinates. The expected column matches the original game's
	// vertical entrance: staircase (0,-2) above foyer (0,-1) above the
	// entrance hall (0,0). Every other tile must have StartCell == nil.
	want := map[int][2]int{
		1000: {0, -2},
		1001: {0, -1},
		1002: {0, 0},
	}
	seen := map[int]bool{}
	for _, x := range tiles {
		exp, isStarter := want[x.ID]
		if isStarter {
			if x.StartCell == nil {
				t.Errorf("starter tile %d missing start_cell", x.ID)
				continue
			}
			if *x.StartCell != exp {
				t.Errorf("starter tile %d: start_cell = %v, want %v", x.ID, *x.StartCell, exp)
			}
			seen[x.ID] = true
		} else if x.StartCell != nil {
			t.Errorf("non-starter tile %d should not carry start_cell, got %v", x.ID, *x.StartCell)
		}
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("starter tile %d not present in tile_meta.yaml", id)
		}
	}

	// Sanity: every tile must allow at least one floor (anchor tiles are
	// the harshest case and are still set by the force callback).
	for _, x := range tiles {
		any := false
		for _, v := range x.Floors {
			if v {
				any = true
				break
			}
		}
		if !any {
			t.Errorf("tile %d has no floor allowed", x.ID)
		}
	}
}

func TestLoadMonsters(t *testing.T) {
	groups, err := LoadMonsters(datafs.FS)
	if err != nil {
		t.Fatalf("LoadMonsters: %v", err)
	}
	if len(groups) < 5 {
		t.Fatalf("expect at least 5 monster colour groups, got %d", len(groups))
	}
	want := map[string]int{
		"红色": 24, "橙色": 24, "绿色": 16,
		"蓝色": 9, "紫色": 6, "玫红色": 6, "黄色": 6,
	}
	for _, g := range groups {
		if expect, ok := want[g.Color]; ok {
			if g.Count != expect || len(g.Pawns) != expect {
				t.Errorf("colour %s: count=%d pawns=%d, want %d",
					g.Color, g.Count, len(g.Pawns), expect)
			}
		}
	}
}

func TestLoadTTSIndex(t *testing.T) {
	ents, err := LoadTTSIndex(datafs.FS)
	if err != nil {
		t.Fatalf("LoadTTSIndex: %v", err)
	}
	if len(ents) < 200 {
		t.Fatalf("expect at least 200 entities, got %d", len(ents))
	}
	// Spot-check a few well-known haunt NPC names show up.
	required := []string{"幽灵", "德古拉", "女鬼", "蜘蛛", "龙"}
	have := map[string]bool{}
	for _, e := range ents {
		have[e.Nickname] = true
	}
	for _, n := range required {
		if !have[n] {
			t.Errorf("missing required entity: %s", n)
		}
	}
}

func TestLoadOmens(t *testing.T) {
	tbl, err := LoadOmens(datafs.FS)
	if err != nil {
		t.Fatalf("LoadOmens: %v", err)
	}
	if len(tbl.Omens) != 13 {
		t.Errorf("expect 13 omens, got %d", len(tbl.Omens))
	}
	if len(tbl.Rooms) != 13 {
		t.Errorf("expect 13 rooms, got %d", len(tbl.Rooms))
	}

	// Sanity: every omen has a Chinese name and an English name.
	for i, o := range tbl.Omens {
		if o.NameCN == "" || o.NameEN == "" {
			t.Errorf("omen %d empty: %+v", i, o)
		}
	}

	// Spot-check the first cell against the source xls. (荐废的房间 × 噬咬 = 17)
	var abIdx, biteIdx int = -1, -1
	for i, r := range tbl.Rooms {
		if r.NameCN == "荒废的房间" {
			abIdx = i
		}
	}
	for i, o := range tbl.Omens {
		if o.NameCN == "噬咬" {
			biteIdx = i
		}
	}
	if abIdx < 0 || biteIdx < 0 {
		t.Fatalf("could not find canary cells: room=%d omen=%d", abIdx, biteIdx)
	}
	if got := tbl.Matrix[abIdx][biteIdx]; got != 17 {
		t.Errorf("荐废的房间 × 噬咬: got %d, want 17", got)
	}

	// Every cell should map to a valid scenario id 1..50.
	for ri, row := range tbl.Matrix {
		for ci, id := range row {
			if id < 1 || id > 50 {
				t.Errorf("matrix[%d][%d] = %d, out of range 1..50", ri, ci, id)
			}
		}
	}
}

func TestLoadScenarios(t *testing.T) {
	scs, err := LoadScenarios(datafs.FS)
	if err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	if len(scs) != 50 {
		t.Fatalf("expect 50 scenarios, got %d", len(scs))
	}

	// Scenarios must be sorted by id 1..50 with no gaps.
	for i, s := range scs {
		if s.ID != i+1 {
			t.Errorf("scenarios[%d].ID = %d, want %d", i, s.ID, i+1)
		}
		if s.Traitor == "" {
			t.Errorf("scenarios[%d] has empty traitor", i)
		}
	}

	// Cross-reference a few well-known mappings from 奸徒身份表.xls.
	want := map[int]string{
		1:  "揭露者",
		6:  "神志数值最低者",
		42: "力量数值最高者",
	}
	for id, traitor := range want {
		if scs[id-1].Traitor != traitor {
			t.Errorf("scenario %d traitor = %q, want %q", id, scs[id-1].Traitor, traitor)
		}
	}
}

func TestLoadRooms(t *testing.T) {
	rooms, err := LoadRooms(datafs.FS)
	if err != nil {
		t.Fatalf("LoadRooms: %v", err)
	}
	if len(rooms) < 40 {
		t.Fatalf("expect at least 40 rooms with rules, got %d", len(rooms))
	}

	index := map[string]Room{}
	for _, r := range rooms {
		if r.NameCN == "" {
			t.Errorf("room with empty Chinese name: %+v", r)
		}
		if r.NameEN == "" {
			t.Errorf("room %q: empty English name", r.NameCN)
		}
		any := false
		for _, f := range r.Floors {
			if f {
				any = true
				break
			}
		}
		if !any {
			t.Errorf("room %q: no floor set", r.NameCN)
		}
		index[r.NameCN] = r
	}

	// Spot-check well-known rooms with their floors.
	// Floors order: [roof, upper, ground, basement].
	wantFloors := map[string][4]bool{
		"陵墓":   {false, false, false, true},  // basement
		"裂缝":   {false, false, false, true},  // basement
		"塔楼":   {false, true, false, false},  // upper
		"酒窖":   {false, false, false, true},  // basement
		"阁楼":   {false, true, false, false},  // upper
	}
	for name, want := range wantFloors {
		r, ok := index[name]
		if !ok {
			t.Errorf("missing required room: %s", name)
			continue
		}
		if r.Floors != want {
			t.Errorf("room %q floors = %v, want %v", name, r.Floors, want)
		}
		if r.NameEN == "" {
			t.Errorf("room %q: missing English name", name)
		}
	}

	// The Wine Cellar must NOT carry the event-card appendix in its rule
	// text (regression check for the joinRule sentinel truncation).
	if wc, ok := index["酒窖"]; ok {
		if len(wc.RuleText) > 200 {
			t.Errorf("酒窖 rule_text too long (%d bytes); appendix sentinel may have failed", len(wc.RuleText))
		}
	}
}

func TestLoadRoomsDoc(t *testing.T) {
	rows, err := LoadRoomsDoc(datafs.FS)
	if err != nil {
		t.Fatalf("LoadRoomsDoc: %v", err)
	}
	if len(rows) < 40 {
		t.Fatalf("expect at least 40 rows from 房间表.doc, got %d", len(rows))
	}

	index := map[string]RoomDoc{}
	valid := map[string]bool{"event": true, "omen": true, "item": true, "base": true, "unknown": true}
	for _, r := range rows {
		if r.NameCN == "" {
			t.Errorf("row with empty NameCN: %+v", r)
		}
		if !valid[r.Kind] {
			t.Errorf("row %q: unexpected kind %q", r.NameCN, r.Kind)
		}
		index[r.NameCN] = r
	}

	// Spot-check well-known rooms with their kinds.
	wantKind := map[string]string{
		"阁楼":     "event",
		"墓园":     "event",
		"荒废的房间": "omen",
		"健身房":    "omen",
		"五芒星堂":   "omen",
		"保险库":    "item",
		"入口大堂":   "base",
		"升降梯":    "base",
	}
	for name, kind := range wantKind {
		r, ok := index[name]
		if !ok {
			t.Errorf("missing required room: %s", name)
			continue
		}
		if r.Kind != kind {
			t.Errorf("room %q: kind = %q, want %q", name, r.Kind, kind)
		}
	}

	// No row should be classified as 'unknown' after gendoc's mapping.
	for _, r := range rows {
		if r.Kind == "unknown" {
			t.Errorf("row %q has kind=unknown (kind_cn=%q); update canonicalKind in cmd/gendoc", r.NameCN, r.KindCN)
		}
	}
}

func TestLoadCharacters(t *testing.T) {
	chars, err := LoadCharacters(datafs.FS)
	if err != nil {
		t.Fatalf("LoadCharacters: %v", err)
	}
	if len(chars) != 12 {
		t.Fatalf("want 12 characters, got %d", len(chars))
	}

	// IDs must be 1..12 in order.
	for i, c := range chars {
		if c.ID != i+1 {
			t.Errorf("chars[%d].ID = %d, want %d", i, c.ID, i+1)
		}
	}

	// Each character has 4 stat tracks; every track must have at least 6
	// cells (the 8-cell base-game tracks plus a sanity floor) and Start
	// must point at a positive (non-death) cell.
	for _, c := range chars {
		if c.NameCN == "" || c.NameEN == "" {
			t.Errorf("character id=%d has empty name", c.ID)
		}
		if c.ColorHex == "" {
			t.Errorf("character %q has empty color_hex", c.NameEN)
		}
		if c.Age <= 0 {
			t.Errorf("character %q age=%d, want >0", c.NameEN, c.Age)
		}
		tracks := map[string]StatTrack{
			"might":     c.Might,
			"speed":     c.Speed,
			"sanity":    c.Sanity,
			"knowledge": c.Knowledge,
		}
		for name, st := range tracks {
			if len(st.Tracks) < 6 {
				t.Errorf("%s.%s tracks len=%d, want >=6", c.NameEN, name, len(st.Tracks))
			}
			if st.Start <= 0 || st.Start >= len(st.Tracks) {
				t.Errorf("%s.%s start=%d not in [1,%d)", c.NameEN, name, st.Start, len(st.Tracks))
			}
		}
	}

	// Canary entries: confirm we kept the canonical CN/EN pairings from
	// tts_index.yaml.
	index := map[string]Character{}
	for _, c := range chars {
		index[c.NameEN] = c
	}
	wantCN := map[string]string{
		"Madame Zostra":            "左思泽夫人",
		"Vivian Lopez":             "薇薇安·洛佩兹",
		"Ox Bellows":               "“公牛”彼罗斯",
		"Darrin \"Flash\" Williams": "达瑞“闪电”威廉姆斯",
		"Father Rhinehardt":        "莱因哈特神父",
	}
	for en, wantCN := range wantCN {
		c, ok := index[en]
		if !ok {
			t.Errorf("missing character: %s", en)
			continue
		}
		if c.NameCN != wantCN {
			t.Errorf("%s: name_cn=%q, want %q", en, c.NameCN, wantCN)
		}
	}

	// Same-card pairs share ColorHex; pre-haunt selection should treat
	// ColorHex as a mutex key. Verify the 6 expected pairs.
	colorCount := map[string]int{}
	for _, c := range chars {
		colorCount[c.ColorHex]++
	}
	if len(colorCount) != 6 {
		t.Errorf("want 6 distinct colours, got %d (%v)", len(colorCount), colorCount)
	}
	for color, n := range colorCount {
		if n != 2 {
			t.Errorf("colour %s shared by %d characters, want 2", color, n)
		}
	}
}
