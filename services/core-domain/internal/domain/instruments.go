package domain

import (
	"fmt"
	"slices"
)

// instrumentIDsProblem returns why ids can't be the instruments an item is
// for, or "" if they can. An empty list is valid: it means the item suits
// every instrument. Whether each id references an existing Instrument needs
// a repository round trip, so that stays an application-layer concern.
func instrumentIDsProblem(ids []string) string {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			return "must not contain an empty id"
		}
		if _, dup := seen[id]; dup {
			return fmt.Sprintf("lists instrument %q more than once", id)
		}
		seen[id] = struct{}{}
	}
	return ""
}

// scopeSuits reports whether an item for scope (empty meaning every
// instrument) may serve content for instrumentIDs (likewise): an item for
// every instrument serves any content; an item for specific instruments
// serves content for at least one of them, never content for every
// instrument.
func scopeSuits(scope, instrumentIDs []string) bool {
	if len(scope) == 0 {
		return true
	}
	for _, id := range instrumentIDs {
		if slices.Contains(scope, id) {
			return true
		}
	}
	return false
}
