package application

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// Creator is a user who created at least one item of a list, with their
// current display name — one option of that list's creator filter.
type Creator struct {
	UserID      string
	DisplayName string
}

// namedCreators turns creator ids into Creators with their current display
// names, keeping only those whose name contains nameQuery when it is
// non-empty. Matching and ordering both ignore case and accents, the way a
// person reads a list of names; equal names fall back to user id. An id
// without a user record is an error, never a nameless entry.
func namedCreators(ctx context.Context, users ports.UserRepository, ids []string, nameQuery string) ([]Creator, error) {
	creators := []Creator{}
	if len(ids) == 0 {
		return creators, nil
	}
	names, err := users.GetDisplayNames(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		name, ok := names[id]
		if !ok {
			return nil, fmt.Errorf("display name for creator %s: %w", id, domain.ErrNotFound)
		}
		if nameQuery != "" && !domain.ContainsLoosely(name, nameQuery) {
			continue
		}
		creators = append(creators, Creator{UserID: id, DisplayName: name})
	}

	collator := collate.New(language.Und)
	sort.Slice(creators, func(i, j int) bool {
		if c := collator.CompareString(creators[i].DisplayName, creators[j].DisplayName); c != 0 {
			return c < 0
		}
		return creators[i].UserID < creators[j].UserID
	})
	return creators, nil
}
