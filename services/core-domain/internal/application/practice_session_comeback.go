package application

import (
	"slices"

	"github.com/motifpath/core-domain/internal/domain"
)

// comebackGap is the fewest other items between an item and its comeback,
// so the second time tests memory, not the echo of the first.
const comebackGap = 3

// comesBack reports whether item is asked a way that brings it back later
// in the same session.
func (c *composer) comesBack(item domain.PracticeSessionItem) bool {
	return c.templates[item.DrillTemplateKey()].ComesBack
}

// cost is the seconds item takes in the session: twice its own when it
// comes back.
func (c *composer) cost(item domain.PracticeSessionItem) int {
	if c.comesBack(item) {
		return 2 * item.EstimatedSeconds
	}
	return item.EstimatedSeconds
}

// withComebacks adds to items the comeback of each item comebacks holds, by
// item key. A comeback waits until comebackGap more of items have been
// asked, and never comes between two confusable items; those still waiting
// at the end come last, in order, and an item that would then have fewer
// than comebackGap items before its comeback is moved earlier.
func withComebacks(items []domain.PracticeSessionItem, comebacks map[string]domain.PracticeSessionItem, confusable map[string][]string) []domain.PracticeSessionItem {
	type waiting struct {
		item domain.PracticeSessionItem
		left int
	}
	var out []domain.PracticeSessionItem
	var queue []waiting
	for i, item := range items {
		out = append(out, item)
		splitsPair := i+1 < len(items) && slices.Contains(confusable[item.ItemKey], items[i+1].ItemKey)
		var still []waiting
		for _, w := range queue {
			if w.left--; w.left > 0 || splitsPair {
				still = append(still, w)
				continue
			}
			out = append(out, w.item)
		}
		queue = still
		if again, ok := comebacks[item.ItemKey]; ok {
			queue = append(queue, waiting{item: again, left: comebackGap})
		}
	}
	for _, w := range queue {
		out = append(out, w.item)
	}
	return spaceComebacks(out, comebacks)
}

// spaceComebacks moves each item with fewer than comebackGap items before
// its comeback earlier, until it has them, where the session allows.
func spaceComebacks(items []domain.PracticeSessionItem, comebacks map[string]domain.PracticeSessionItem) []domain.PracticeSessionItem {
	out := slices.Clone(items)
	for _, original := range items {
		key := original.ItemKey
		if _, ok := comebacks[key]; !ok {
			continue
		}
		first := slices.IndexFunc(out, func(item domain.PracticeSessionItem) bool { return item.ItemKey == key })
		second := first + 1 + slices.IndexFunc(out[first+1:], func(item domain.PracticeSessionItem) bool { return item.ItemKey == key })
		if second <= first || second-first-1 >= comebackGap {
			continue
		}
		to := max(second-1-comebackGap, 0)
		item := out[first]
		out = slices.Insert(slices.Delete(out, first, first+1), to, item)
	}
	return out
}
