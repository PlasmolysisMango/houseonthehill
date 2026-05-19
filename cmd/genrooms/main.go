// genrooms parses the Chinese rulebook PDF and emits a structured yaml of
// every room that has special rules.
//
//   assets/raw/小黑屋/Betrayal at House on the Hill_cn.pdf
//                                            -> assets/datafs/rooms.yaml
//
// The PDF lays each room out as:
//
//     <Chinese name>            <- one line of CJK
//     <English name>            <- 1..N lines of pure ASCII
//     (
//     <Chinese floor>           <- e.g. 地下
//     <English floor>[、...]    <- e.g. basement     OR  basement、ground
//     ...                       <- repeated for multi-floor rooms
//     <floor>)
//     <rule text...>            <- until the next room's Chinese-name line
//
// We anchor on the lone "(" line that introduces every room block, walk
// back to recover the name, walk forward to recover the floor list, then
// capture rule text up to (but not including) the next room's name.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

type room struct {
	NameCN   string
	NameEN   string
	Floors   [4]bool // [roof, upper, ground, basement] -- matches tile.Floor*
	RuleText string
}

var floorClose = regexp.MustCompile(`^(basement|ground|upper|roof)\)$`)

func main() {
	in := flag.String("in", "assets/raw/小黑屋/Betrayal at House on the Hill_cn.pdf", "rulebook PDF")
	outDir := flag.String("out", "assets/datafs", "directory to write yaml into")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		die("mkdir %s: %v", *outDir, err)
	}

	lines, err := pdfLines(*in)
	if err != nil {
		die("read pdf: %v", err)
	}

	rooms := extractRooms(lines)
	if len(rooms) == 0 {
		die("no room blocks recognised — pdf format may have changed")
	}

	if err := os.WriteFile(filepath.Join(*outDir, "rooms.yaml"), []byte(emitRooms(rooms)), 0o644); err != nil {
		die("write rooms.yaml: %v", err)
	}
	fmt.Fprintf(os.Stderr, "rooms.yaml: %d rooms with rules\n", len(rooms))
}

// pdfLines reads every page of the PDF and returns every non-page-marker
// line, trimmed of trailing whitespace. Empty lines are kept so we don't
// accidentally fuse adjacent blocks.
func pdfLines(path string) ([]string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []string
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i, err)
		}
		for _, ln := range strings.Split(text, "\n") {
			out = append(out, strings.TrimRight(ln, " \t\r"))
		}
	}
	return out, nil
}

// extractRooms scans lines for "(" anchor lines that introduce a room block
// and pulls each one out as a room.
func extractRooms(lines []string) []room {
	// Step 1: locate every "(" anchor line that looks like a room-block
	// opener. A real opener is preceded (skipping blanks) by a pure-ASCII
	// line (the English name); parenthetical asides inside rule text are
	// preceded by Chinese and are filtered out here.
	anchors := make([]int, 0, 64)
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "(" {
			continue
		}
		if !isRoomAnchor(lines, i) {
			continue
		}
		anchors = append(anchors, i)
	}

	// Step 2: for each anchor, recover (nameStartLine, closeLine).
	type block struct {
		anchor    int
		nameStart int
		closeLine int
	}
	blocks := make([]block, 0, len(anchors))
	for _, a := range anchors {
		ns := nameStartLine(lines, a)
		if ns < 0 {
			continue
		}
		cl := floorCloseLine(lines, a)
		if cl < 0 {
			continue
		}
		blocks = append(blocks, block{anchor: a, nameStart: ns, closeLine: cl})
	}

	// Step 3: turn each block into a room. Rule text spans from the line
	// after closeLine to the line before the NEXT block's nameStart (or
	// EOF for the last block).
	out := make([]room, 0, len(blocks))
	for i, b := range blocks {
		nameLines := lines[b.nameStart:b.anchor]
		nameCN, nameEN := splitName(nameLines)
		if nameCN == "" {
			continue
		}
		floors := parseFloors(lines[b.anchor : b.closeLine+1])

		ruleEnd := len(lines)
		if i+1 < len(blocks) {
			ruleEnd = blocks[i+1].nameStart
		}
		ruleText := joinRule(lines[b.closeLine+1 : ruleEnd])

		out = append(out, room{
			NameCN:   nameCN,
			NameEN:   nameEN,
			Floors:   floors,
			RuleText: ruleText,
		})
	}

	// Deduplicate by Chinese name in case the rulebook lists a room twice
	// (e.g. cross-reference appendix). Keep the longest rule text.
	seen := map[string]int{}
	dedup := make([]room, 0, len(out))
	for _, r := range out {
		if idx, ok := seen[r.NameCN]; ok {
			if len(r.RuleText) > len(dedup[idx].RuleText) {
				dedup[idx] = r
			}
			continue
		}
		seen[r.NameCN] = len(dedup)
		dedup = append(dedup, r)
	}
	sort.SliceStable(dedup, func(i, j int) bool { return dedup[i].NameCN < dedup[j].NameCN })
	return dedup
}

// isRoomAnchor returns true when the lone "(" line at index i looks like a
// room-block opener: the previous non-blank line must be pure ASCII (the
// English name continuation). This filters out parenthetical asides like
// "(或袭击，长程袭击除外)" embedded in rule text.
func isRoomAnchor(lines []string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		ln := strings.TrimSpace(lines[j])
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, "--- Page ") {
			return false
		}
		return !hasCJK(ln)
	}
	return false
}

// nameStartLine walks back from the "(" anchor and returns the line index
// of the room's Chinese name. Heuristic: skip pure-ASCII lines (the
// English-name continuation), stop at the first non-empty line that
// contains a CJK ideograph.
func nameStartLine(lines []string, anchor int) int {
	i := anchor - 1
	// Step over the English-name lines.
	for i >= 0 {
		ln := strings.TrimSpace(lines[i])
		if ln == "" {
			i--
			continue
		}
		if hasCJK(ln) {
			return i
		}
		i--
	}
	return -1
}

// floorCloseLine walks forward from the "(" anchor and returns the line
// index where the floor list closes (e.g. "basement)").
func floorCloseLine(lines []string, anchor int) int {
	for i := anchor + 1; i < len(lines) && i < anchor+30; i++ {
		ln := strings.TrimSpace(lines[i])
		if floorClose.MatchString(ln) {
			return i
		}
	}
	return -1
}

// splitName splits the lines between nameStart and the "(" anchor into
// (nameCN, nameEN). The first non-empty line is the Chinese name; every
// following non-empty line is concatenated (space-separated) as the
// English name.
func splitName(lines []string) (cn, en string) {
	collected := false
	enParts := make([]string, 0, 3)
	for _, raw := range lines {
		ln := strings.TrimSpace(raw)
		if ln == "" {
			continue
		}
		if !collected {
			cn = ln
			collected = true
			continue
		}
		enParts = append(enParts, ln)
	}
	en = strings.Join(enParts, " ")
	return cn, en
}

// parseFloors returns a [4]bool of the room's allowed floors. The floor
// region is the lines from "(" through "<floor>)". We pick out occurrences
// of the four English floor keywords.
func parseFloors(region []string) [4]bool {
	var f [4]bool
	idx := map[string]int{"roof": 0, "upper": 1, "ground": 2, "basement": 3}
	for _, raw := range region {
		ln := strings.TrimSpace(raw)
		ln = strings.TrimSuffix(ln, ")")
		// Floor keywords may be preceded by Chinese delimiters like 、.
		for kw, i := range idx {
			if ln == kw || strings.HasSuffix(ln, kw) {
				f[i] = true
			}
		}
	}
	return f
}

// joinRule concatenates the rule-text lines after dropping page markers,
// then collapses interior newlines so the yaml stays readable.
//
// The last room with rules in the PDF ("酒窖" / Wine Cellar) would
// otherwise swallow the entire event-card appendix that follows it. We
// truncate at the appendix sentinel "以上房间皆无特别规则", which the
// rulebook prints right after the last room block.
func joinRule(lines []string) string {
	const appendixSentinel = "以上房间皆无特别规则"
	var b strings.Builder
	for _, raw := range lines {
		ln := strings.TrimSpace(raw)
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, "--- Page ") {
			continue
		}
		if idx := strings.Index(ln, appendixSentinel); idx >= 0 {
			b.WriteString(ln[:idx])
			break
		}
		b.WriteString(ln)
	}
	return b.String()
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func emitRooms(rs []room) string {
	var b strings.Builder
	b.WriteString("# Auto-generated by cmd/genrooms from\n")
	b.WriteString("# assets/raw/小黑屋/Betrayal at House on the Hill_cn.pdf.\n")
	b.WriteString("# DO NOT edit by hand; rerun `go run ./cmd/genrooms`.\n")
	b.WriteString("# Only rooms that have special rules in the rulebook show up\n")
	b.WriteString("# here (~45 rooms). The remaining ~20 plain rooms have no\n")
	b.WriteString("# special text and need to be filled into tile_meta.yaml by\n")
	b.WriteString("# hand.\n")
	b.WriteString("# floors order: [roof, upper, ground, basement].\n\n")

	b.WriteString("rooms:\n")
	for _, r := range rs {
		fmt.Fprintf(&b, "  - name_cn: %q\n", r.NameCN)
		fmt.Fprintf(&b, "    name_en: %q\n", r.NameEN)
		fmt.Fprintf(&b, "    floors: [%t, %t, %t, %t]\n",
			r.Floors[0], r.Floors[1], r.Floors[2], r.Floors[3])
		fmt.Fprintf(&b, "    rule_text: %q\n", r.RuleText)
	}
	return b.String()
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "genrooms: "+format+"\n", args...)
	os.Exit(1)
}
