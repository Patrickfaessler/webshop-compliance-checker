package rules

import (
	"fmt"
	"sort"
)

var registry = map[string]Rule{}

// Register adds a rule. It panics on a duplicate ID so mistakes show up at start-up.
func Register(r Rule) {
	id := r.Meta().ID
	if _, exists := registry[id]; exists {
		panic(fmt.Sprintf("rules: duplicate rule id %q", id))
	}
	registry[id] = r
}

// All returns the registered rules sorted by ID.
func All() []Rule {
	out := make([]Rule, 0, len(registry))
	for _, r := range registry {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta().ID < out[j].Meta().ID })
	return out
}
