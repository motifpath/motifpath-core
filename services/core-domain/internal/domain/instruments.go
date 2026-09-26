package domain

import "fmt"

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
