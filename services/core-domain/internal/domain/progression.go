package domain

// Progression is a common chord progression of the progression catalog, by
// number: one numeral per bar of a key in Mode, such as I, vi or V7, in a
// key the drill asking it picks. Confusable lists the keys of the
// progressions a learner mistakes it for, each listing it back. Reference
// data, installed by migration.
type Progression struct {
	ID         string
	Key        string
	Mode       string
	Bars       []string
	NodeIDs    []string
	Confusable []string
	Names      LocalizedText
}
