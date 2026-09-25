package repo

import (
	"slices"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
)

// instrumentIDsOf returns the ids of rows, sorted, so an item's instruments
// read back in a stable order. No rows is nil: the item suits every
// instrument.
func instrumentIDsOf(rows []*ent.Instrument) []string {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID.String()
	}
	slices.Sort(ids)
	return ids
}
