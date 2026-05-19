// Package data loads the yaml-encoded game-rule data shipped under
// assets/data/. Each loader takes an fs.FS so it can be unit-tested against
// a stub filesystem; production callers pass assets.DataFS.
package data

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// TileMeta is the editable description of one room tile. The geometric fields
// (ID, Source, Row, Col, Doors, Floors, Note) are auto-generated from the
// artwork by cmd/gendoors and overwritten on each regeneration. The text
// fields (NameCN, NameEN) and the optional StartCell are preserved across
// regenerations.
//
// StartCell is set only for the three starter tiles (id 1000–1002): it pins
// each tile to a board cell so the runtime can lay out the entrance column
// without hard-coded coordinates. Nil for every base / extension tile.
type TileMeta struct {
	ID        int     `yaml:"id"`
	Source    string  `yaml:"source"`
	Row       int     `yaml:"row"`
	Col       int     `yaml:"col"`
	StartCell *[2]int `yaml:"start_cell,omitempty"`
	Doors     [4]bool `yaml:"doors"`
	Floors    [4]bool `yaml:"floors"`
	NameCN    string  `yaml:"name_cn"`
	NameEN    string  `yaml:"name_en"`
	Note      string  `yaml:"note"`
}

// MonsterPawn is one physical pawn from the TTS mod.
type MonsterPawn struct {
	Number int    `yaml:"number"`
	GUID   string `yaml:"guid"`
}

// MonsterGroup is a set of pawns sharing a colour.
type MonsterGroup struct {
	Color string        `yaml:"color"`
	Count int           `yaml:"count"`
	Pawns []MonsterPawn `yaml:"pawns"`
}

// TTSEntity is one named TTS object: a monster, token, pawn, deck, etc.
// Subsequent stages join on Nickname.
type TTSEntity struct {
	Nickname    string `yaml:"nickname"`
	Kind        string `yaml:"kind"`
	GUID        string `yaml:"guid"`
	Description string `yaml:"description"`
}

// Omen is one of the 13 omen cards. NameCN matches the column header in
// 真相表.xls; NameEN is hard-coded in cmd/genxls.
type Omen struct {
	NameCN string `yaml:"name_cn"`
	NameEN string `yaml:"name_en"`
}

// OmenRoom is one of the 13 rooms that can trigger a haunt. Names come
// directly from the 真相表.xls room column.
type OmenRoom struct {
	NameCN string `yaml:"name_cn"`
	NameEN string `yaml:"name_en"`
}

// OmenTable is the haunt-roll lookup table: given an omen card and the room
// the player is in, Matrix[roomIdx][omenIdx] is the scenario id (1..50).
type OmenTable struct {
	Omens  []Omen     `yaml:"omens"`
	Rooms  []OmenRoom `yaml:"rooms"`
	Matrix [][]int    `yaml:"matrix"`
}

// Scenario is the index entry for one haunt scenario (1..50). Traitor is the
// raw Chinese description from 奸徒身份表.xls (e.g. "揭露者", "力量数值最高者").
type Scenario struct {
	ID      int    `yaml:"id"`
	Traitor string `yaml:"traitor"`
}

// Room is a room with special rules, parsed from the rulebook PDF by
// cmd/genrooms. Floors order is [roof, upper, ground, basement] and matches
// tile.Floor*. Plain rooms with no special text do NOT appear here.
type Room struct {
	NameCN   string  `yaml:"name_cn"`
	NameEN   string  `yaml:"name_en"`
	Floors   [4]bool `yaml:"floors"`
	RuleText string  `yaml:"rule_text"`
}

// RoomDoc is one row from 房间表.doc, decoded by cmd/gendoc. KindCN is the
// raw Chinese tag (事件 / 凶兆 / 物品 / 特殊通道 / 空) and Kind is its
// canonical English form (event / omen / item / base / unknown).
type RoomDoc struct {
	NameCN string `yaml:"name_cn"`
	KindCN string `yaml:"kind_cn"`
	Kind   string `yaml:"kind"`
}

// StatTrack describes one of the four stat tracks (might / speed / sanity /
// knowledge) printed on the back of a character card. Tracks are read left
// to right; index 0 is the death skull and lethal — when a character's index
// is decremented to 0 the character dies. Start is the initial cursor index
// at the beginning of a game and must satisfy 0 < Start < len(Tracks).
type StatTrack struct {
	Tracks []int `yaml:"tracks"`
	Start  int   `yaml:"start"`
}

// Character is one of the 12 explorers from the base game. The four stat
// tracks share an identical schema; ColorHex is the TTS-mod accent colour
// ("#RRGGBB") and is shared by the two characters printed on the same
// double-sided character card — character selection should treat ColorHex
// as a mutual-exclusion key.
type Character struct {
	ID        int       `yaml:"id"`
	NameCN    string    `yaml:"name_cn"`
	NameEN    string    `yaml:"name_en"`
	Age       int       `yaml:"age"`
	ColorHex  string    `yaml:"color_hex"`
	Might     StatTrack `yaml:"might"`
	Speed     StatTrack `yaml:"speed"`
	Sanity    StatTrack `yaml:"sanity"`
	Knowledge StatTrack `yaml:"knowledge"`
}

// LoadTiles parses data/tile_meta.yaml.
func LoadTiles(fsys fs.FS) ([]TileMeta, error) {
	var doc struct {
		Tiles []TileMeta `yaml:"tiles"`
	}
	if err := readYAML(fsys, "tile_meta.yaml", &doc); err != nil {
		return nil, err
	}
	return doc.Tiles, nil
}

// LoadMonsters parses data/monsters.yaml.
func LoadMonsters(fsys fs.FS) ([]MonsterGroup, error) {
	var doc struct {
		Monsters []MonsterGroup `yaml:"monsters"`
	}
	if err := readYAML(fsys, "monsters.yaml", &doc); err != nil {
		return nil, err
	}
	return doc.Monsters, nil
}

// LoadTTSIndex parses data/tts_index.yaml.
func LoadTTSIndex(fsys fs.FS) ([]TTSEntity, error) {
	var doc struct {
		Entities []TTSEntity `yaml:"entities"`
	}
	if err := readYAML(fsys, "tts_index.yaml", &doc); err != nil {
		return nil, err
	}
	return doc.Entities, nil
}

// LoadOmens parses data/omens.yaml. The returned table has
// len(Rooms)==len(Matrix) and each row in Matrix has len(Omens) entries.
func LoadOmens(fsys fs.FS) (*OmenTable, error) {
	var t OmenTable
	if err := readYAML(fsys, "omens.yaml", &t); err != nil {
		return nil, err
	}
	if len(t.Rooms) != len(t.Matrix) {
		return nil, fmt.Errorf("omens.yaml: rooms=%d but matrix rows=%d", len(t.Rooms), len(t.Matrix))
	}
	for i, row := range t.Matrix {
		if len(row) != len(t.Omens) {
			return nil, fmt.Errorf("omens.yaml: row %d has %d cells, expected %d", i, len(row), len(t.Omens))
		}
	}
	return &t, nil
}

// LoadScenarios parses data/scenarios_index.yaml.
func LoadScenarios(fsys fs.FS) ([]Scenario, error) {
	var doc struct {
		Scenarios []Scenario `yaml:"scenarios"`
	}
	if err := readYAML(fsys, "scenarios_index.yaml", &doc); err != nil {
		return nil, err
	}
	return doc.Scenarios, nil
}

// LoadRooms parses data/rooms.yaml.
func LoadRooms(fsys fs.FS) ([]Room, error) {
	var doc struct {
		Rooms []Room `yaml:"rooms"`
	}
	if err := readYAML(fsys, "rooms.yaml", &doc); err != nil {
		return nil, err
	}
	return doc.Rooms, nil
}

// LoadRoomsDoc parses data/rooms_doc.yaml (the canonical Chinese name +
// type list extracted from 房间表.doc by cmd/gendoc).
func LoadRoomsDoc(fsys fs.FS) ([]RoomDoc, error) {
	var doc struct {
		Rooms []RoomDoc `yaml:"rooms_doc"`
	}
	if err := readYAML(fsys, "rooms_doc.yaml", &doc); err != nil {
		return nil, err
	}
	return doc.Rooms, nil
}

// LoadCharacters parses data/characters.yaml and validates each stat track:
// every Tracks slice must be non-empty and Start must be a strictly positive
// index that fits inside the slice (Start == 0 would mean the character
// starts dead).
func LoadCharacters(fsys fs.FS) ([]Character, error) {
	var doc struct {
		Characters []Character `yaml:"characters"`
	}
	if err := readYAML(fsys, "characters.yaml", &doc); err != nil {
		return nil, err
	}
	for i, c := range doc.Characters {
		for _, st := range []struct {
			name  string
			track StatTrack
		}{
			{"might", c.Might},
			{"speed", c.Speed},
			{"sanity", c.Sanity},
			{"knowledge", c.Knowledge},
		} {
			if len(st.track.Tracks) == 0 {
				return nil, fmt.Errorf("characters.yaml[%d] %s: empty tracks", i, st.name)
			}
			if st.track.Start <= 0 || st.track.Start >= len(st.track.Tracks) {
				return nil, fmt.Errorf("characters.yaml[%d] %s: start=%d out of range [1,%d)",
					i, st.name, st.track.Start, len(st.track.Tracks))
			}
		}
	}
	return doc.Characters, nil
}

func readYAML(fsys fs.FS, path string, into any) error {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
