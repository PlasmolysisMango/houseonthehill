package cards

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// LoadAll parses cards.yaml from the given filesystem and returns three
// pre-bucketed slices in (events, items, omens) order. Cards in the
// yaml may declare their kind explicitly; if missing it's inferred
// from the section name. Each card's Kind is set on the returned
// pointers so downstream code can rely on it.
//
// Returns an error if cards.yaml is missing, malformed, or contains
// duplicate IDs within the same deck.
func LoadAll(fsys fs.FS) (events, items, omens []*Card, err error) {
	raw, err := fs.ReadFile(fsys, "cards.yaml")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read cards.yaml: %w", err)
	}
	var doc struct {
		Events []*Card `yaml:"events"`
		Items  []*Card `yaml:"items"`
		Omens  []*Card `yaml:"omens"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, nil, nil, fmt.Errorf("parse cards.yaml: %w", err)
	}
	if err := finalize(doc.Events, KindEvent); err != nil {
		return nil, nil, nil, fmt.Errorf("events: %w", err)
	}
	if err := finalize(doc.Items, KindItem); err != nil {
		return nil, nil, nil, fmt.Errorf("items: %w", err)
	}
	if err := finalize(doc.Omens, KindOmen); err != nil {
		return nil, nil, nil, fmt.Errorf("omens: %w", err)
	}
	return doc.Events, doc.Items, doc.Omens, nil
}

func finalize(cs []*Card, def Kind) error {
	seen := make(map[int]bool, len(cs))
	for i, c := range cs {
		if c == nil {
			return fmt.Errorf("card %d is nil", i)
		}
		if c.Kind == "" {
			c.Kind = def
		}
		if c.ID == 0 {
			return fmt.Errorf("card %d (%q) missing id", i, c.NameEN)
		}
		if seen[c.ID] {
			return fmt.Errorf("duplicate card id %d", c.ID)
		}
		seen[c.ID] = true
	}
	return nil
}
