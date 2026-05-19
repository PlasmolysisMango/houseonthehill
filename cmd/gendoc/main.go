// gendoc parses 小黑屋/房间表.doc (an OLE2 binary Word document) and
// extracts the canonical Chinese room-name list with each room's type tag
// (事件 / 凶兆 / 物品 / etc.) into:
//
//	assets/datafs/rooms_doc.yaml
//
// We do NOT depend on any third-party OLE/.doc parser: the WordDocument
// stream stores body text as runs of UTF-16LE code units, so a brute-force
// utf16 decode of the whole file followed by a Han-character regex sweep
// recovers every printable run with negligible noise. Each room entry in
// the source table looks like one of:
//
//	阁楼（事件）<\r><rule body...>
//	荒废的房间（凶兆）
//	保险库（物品、物品）<\r><rule body...>
//
// We split each run on the first '（', take the prefix as name_cn, the
// content of the parens as kind_cn, then map kind_cn to a canonical kind
// keyword (event / omen / item / base / unknown). Rule bodies are
// dropped — the canonical rule text already lives in rooms.yaml from the
// PDF, and aligning two different translations would only invite drift.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

type roomDoc struct {
	NameCN string
	KindCN string
	Kind   string // canonical: event/omen/item/base/unknown
}

func main() {
	in := flag.String("in", "assets/raw/小黑屋/房间表.doc", "OLE2 .doc input")
	outDir := flag.String("out", "assets/datafs", "directory to write yaml into")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		die("mkdir %s: %v", *outDir, err)
	}

	raw, err := os.ReadFile(*in)
	if err != nil {
		die("read %s: %v", *in, err)
	}

	text := decodeUTF16LE(raw)
	rooms := extractRooms(text)
	if len(rooms) < 30 {
		die("only %d rooms recovered — %s format may have changed", len(rooms), *in)
	}

	out := emit(rooms, *in)
	if err := os.WriteFile(filepath.Join(*outDir, "rooms_doc.yaml"), []byte(out), 0o644); err != nil {
		die("write rooms_doc.yaml: %v", err)
	}
	fmt.Fprintf(os.Stderr, "rooms_doc.yaml: %d named rooms\n", len(rooms))
}

// decodeUTF16LE reinterprets every byte pair as a little-endian uint16
// and runs them through utf16.Decode. The high noise from non-text
// streams in the OLE container produces unprintable code points that we
// filter out downstream by Han-character regex.
func decodeUTF16LE(raw []byte) string {
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	u := make([]uint16, len(raw)/2)
	for i := 0; i < len(u); i++ {
		u[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

// runRe matches a Han-led run that starts with at least one CJK ideograph
// followed by ideograph/ASCII/punctuation. We require the leading rune
// to be Han so we don't snag random ASCII runs from style tables.
var runRe = regexp.MustCompile(`[\p{Han}][\p{Han}A-Za-z0-9（）()、，。·~!\- ]{1,40}`)

// fullParen extracts the parenthetical type tag, e.g. （事件）.
var fullParen = regexp.MustCompile(`（([^）]+)）`)

// blockedNames are 4 entries from the doc table that are mansion fixtures
// (entrance / staircase / landings / chute) carrying explanatory text,
// not actual room tiles. They are kept in our output but flagged
// kind=base so callers can filter them.
var fixtureNames = map[string]bool{
	"入口大堂": true, "门厅": true, "楼梯间": true,
	"上台阶": true, "下台阶": true, "煤导槽": true,
	"地下楼梯": true, "升降梯": true,
}

func extractRooms(text string) []roomDoc {
	matches := runRe.FindAllString(text, -1)

	seen := map[string]roomDoc{}
	for _, m := range matches {
		s := strings.TrimSpace(m)
		// Header lines we do not care about ("门厅密道为特殊通道" etc.).
		if !strings.ContainsAny(s, "（(") {
			// Run carries no type tag; treat as fixture if the
			// run is exactly a known fixture name. Otherwise skip.
			if fixtureNames[s] {
				keep(seen, roomDoc{NameCN: s, KindCN: "", Kind: "base"})
			}
			continue
		}
		// Split on the first '（'.
		i := strings.IndexAny(s, "（(")
		name := strings.TrimSpace(s[:i])
		if name == "" {
			continue
		}
		// Extract the parenthetical content.
		mp := fullParen.FindStringSubmatch(s)
		if mp == nil {
			continue
		}
		kindCN := strings.TrimSpace(mp[1])
		// Sometimes the kind tag is "物品、物品" or similar — first token.
		if idx := strings.IndexAny(kindCN, "、,，"); idx >= 0 {
			kindCN = strings.TrimSpace(kindCN[:idx])
		}
		kind := canonicalKind(kindCN, fixtureNames[name])
		// Reject obvious garbage: name must contain a Han ideograph and
		// be no longer than 12 runes (longest legit name "暗夜的房间" is 5).
		if runeLen(name) > 12 {
			continue
		}
		// Reject descriptive lead-ins from the doc that are not
		// real room names, e.g. "连接上台阶（特殊通道）" is a
		// caption attached to 楼梯间, not a tile of its own.
		if strings.HasPrefix(name, "连接") || strings.HasPrefix(name, "永远连接") {
			continue
		}
		keep(seen, roomDoc{NameCN: name, KindCN: kindCN, Kind: kind})
	}

	out := make([]roomDoc, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].NameCN < out[j].NameCN })
	return out
}

// keep prefers entries with a non-empty Kind over entries without one,
// so fixtures encountered both as a bare run and a tagged run end up
// classified correctly.
func keep(into map[string]roomDoc, r roomDoc) {
	prev, ok := into[r.NameCN]
	if !ok {
		into[r.NameCN] = r
		return
	}
	if prev.Kind == "" || prev.Kind == "unknown" {
		into[r.NameCN] = r
	}
}

func canonicalKind(cn string, isFixture bool) string {
	if isFixture {
		return "base"
	}
	switch cn {
	case "事件":
		return "event"
	case "凶兆":
		return "omen"
	case "物品":
		return "item"
	case "特殊通道":
		return "base"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func emit(rs []roomDoc, in string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Auto-generated by cmd/gendoc from %s.\n", in)
	b.WriteString("# DO NOT edit by hand; rerun `go run ./cmd/gendoc`.\n")
	b.WriteString("# kind: event | omen | item | base | unknown.\n")
	b.WriteString("#   event = 事件房间, omen = 凶兆房间, item = 物品房间.\n")
	b.WriteString("#   base  = 入口/门厅/楼梯/升降梯等大宅固有结构.\n\n")
	b.WriteString("rooms_doc:\n")
	for _, r := range rs {
		fmt.Fprintf(&b, "  - name_cn: %q\n", r.NameCN)
		fmt.Fprintf(&b, "    kind_cn: %q\n", r.KindCN)
		fmt.Fprintf(&b, "    kind:    %q\n", r.Kind)
	}
	return b.String()
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gendoc: "+format+"\n", args...)
	os.Exit(1)
}
