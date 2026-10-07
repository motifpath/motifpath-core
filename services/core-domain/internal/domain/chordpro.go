package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// MaxChordProTextLength is the longest ChordPro text, in characters, an
// import reads.
const MaxChordProTextLength = 100_000

// ChordProWarningKind is why an import skipped part of a ChordPro text.
type ChordProWarningKind string

const (
	ChordProUnsupportedDirective ChordProWarningKind = "unsupported_directive"
	ChordProMalformedDirective   ChordProWarningKind = "malformed_directive"
	ChordProUnclosedChord        ChordProWarningKind = "unclosed_chord"
	ChordProUnbalancedSection    ChordProWarningKind = "unbalanced_section"
	ChordProEmptySection         ChordProWarningKind = "empty_section"
)

// ChordProWarning is a part of a ChordPro text the import skipped: its line,
// numbered from 1, and the text as written.
type ChordProWarning struct {
	Line int
	Kind ChordProWarningKind
	Text string
}

// ChordProMetadata is the draft fields a ChordPro text sets; a field the
// text doesn't set is nil.
type ChordProMetadata struct {
	Title         *string
	Artist        *string
	ConcertKey    *string
	CapoFret      *int
	TempoBPM      *int
	TimeSignature *TimeSignature
}

// ChordProImport is what a ChordPro text reads as. Its anchors carry only
// their written symbols; resolving them to catalog chords is up to the
// caller.
type ChordProImport struct {
	Metadata ChordProMetadata
	Body     SongChartDocument
	Warnings []ChordProWarning
}

// HasLyrics reports whether the document holds at least one lyric line.
func (d SongChartDocument) HasLyrics() bool {
	for _, section := range d.Sections {
		for _, line := range section.Lines {
			if line.Comment == nil {
				return true
			}
		}
	}
	return false
}

// The section environments ChordPro has, by every name that opens or closes
// one. Every other section kind is written bare.
var (
	chordProSectionStarts = map[string]SectionKind{
		"start_of_verse": SectionVerse, "sov": SectionVerse,
		"start_of_chorus": SectionChorus, "soc": SectionChorus,
		"start_of_bridge": SectionBridge, "sob": SectionBridge,
	}
	chordProSectionEnds = map[string]SectionKind{
		"end_of_verse": SectionVerse, "eov": SectionVerse,
		"end_of_chorus": SectionChorus, "eoc": SectionChorus,
		"end_of_bridge": SectionBridge, "eob": SectionBridge,
	}
	chordProTimePattern = regexp.MustCompile(`^([0-9]+)/([0-9]+)$`)
)

// ImportChordPro reads a ChordPro text into a chart's metadata and body.
// Whatever it can't read is skipped and reported as a warning with its
// line, never dropped silently.
func ImportChordPro(text string) ChordProImport {
	r := chordProReader{}
	for i, raw := range strings.Split(text, "\n") {
		r.readLine(i+1, strings.TrimRightFunc(strings.TrimSuffix(raw, "\r"), unicode.IsSpace))
	}
	r.closeSection()
	slices.SortStableFunc(r.out.Warnings, func(a, b ChordProWarning) int { return a.Line - b.Line })
	return r.out
}

type chordProReader struct {
	out     ChordProImport
	open    *chordProSection
	anchors int
}

// chordProSection is the section being read. environment is false for the
// unlabelled section that lines outside any environment fall into.
type chordProSection struct {
	section     SongChartSection
	environment bool
	line        int
	text        string
}

func (r *chordProReader) warn(line int, kind ChordProWarningKind, text string) {
	r.out.Warnings = append(r.out.Warnings, ChordProWarning{Line: line, Kind: kind, Text: text})
}

func (r *chordProReader) readLine(n int, line string) {
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "", strings.HasPrefix(line, "#"):
		return
	case strings.HasPrefix(trimmed, "{"):
		if !strings.HasSuffix(trimmed, "}") {
			r.warn(n, ChordProMalformedDirective, trimmed)
			return
		}
		r.readDirective(n, trimmed)
	default:
		r.addLine(r.lyricLine(n, line))
	}
}

func (r *chordProReader) readDirective(n int, directive string) {
	name, value, _ := strings.Cut(directive[1:len(directive)-1], ":")
	name, value = strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(value)
	if kind, ok := chordProSectionStarts[name]; ok {
		r.startSection(n, directive, kind, value)
		return
	}
	if kind, ok := chordProSectionEnds[name]; ok {
		r.endSection(n, directive, kind)
		return
	}
	if name == "comment" || name == "c" {
		if value == "" {
			r.warn(n, ChordProMalformedDirective, directive)
			return
		}
		r.addLine(SongChartLine{Comment: &value})
		return
	}
	set, supported := r.metadataSetter(name)
	if !supported {
		r.warn(n, ChordProUnsupportedDirective, directive)
		return
	}
	if !set(value) {
		r.warn(n, ChordProMalformedDirective, directive)
	}
}

// metadataSetter returns what sets the metadata field a directive names. It
// reports false when the value doesn't fit the field, which is then left as
// it was.
func (r *chordProReader) metadataSetter(name string) (func(value string) bool, bool) {
	m := &r.out.Metadata
	switch name {
	case "title", "t":
		return func(v string) bool { return setText(&m.Title, v) }, true
	case "artist":
		return func(v string) bool { return setText(&m.Artist, v) }, true
	case "key":
		return func(v string) bool {
			if !concertKeyPattern.MatchString(v) {
				return false
			}
			m.ConcertKey = &v
			return true
		}, true
	case "capo":
		return func(v string) bool { return setInt(&m.CapoFret, v, 0, MaxSongChartCapoFret) }, true
	case "tempo":
		return func(v string) bool { return setInt(&m.TempoBPM, v, MinTempoBPM, MaxTempoBPM) }, true
	case "time":
		return func(v string) bool { return setTimeSignature(&m.TimeSignature, v) }, true
	}
	return nil, false
}

func setText(field **string, value string) bool {
	if value == "" {
		return false
	}
	*field = &value
	return true
}

func setInt(field **int, value string, lowest, highest int) bool {
	n, err := strconv.Atoi(value)
	if err != nil || n < lowest || n > highest {
		return false
	}
	*field = &n
	return true
}

func setTimeSignature(field **TimeSignature, value string) bool {
	match := chordProTimePattern.FindStringSubmatch(value)
	if match == nil {
		return false
	}
	beats, beatsErr := strconv.Atoi(match[1])
	beatValue, valueErr := strconv.Atoi(match[2])
	ts := TimeSignature{Beats: beats, BeatValue: beatValue}
	if beatsErr != nil || valueErr != nil || !ts.valid() {
		return false
	}
	*field = &ts
	return true
}

func (r *chordProReader) startSection(n int, directive string, kind SectionKind, label string) {
	if r.open != nil && r.open.environment {
		r.warn(n, ChordProUnbalancedSection, directive)
	}
	r.closeSection()
	section := SongChartSection{Kind: kind}
	if label != "" {
		section.Label = &label
	}
	r.open = &chordProSection{section: section, environment: true, line: n, text: directive}
}

func (r *chordProReader) endSection(n int, directive string, kind SectionKind) {
	if r.open == nil || !r.open.environment {
		r.warn(n, ChordProUnbalancedSection, directive)
		return
	}
	if r.open.section.Kind != kind {
		r.warn(n, ChordProUnbalancedSection, directive)
	}
	r.closeSection()
}

// closeSection ends the section being read. An environment with no lines is
// dropped, with a warning on the line that opened it.
func (r *chordProReader) closeSection() {
	open := r.open
	r.open = nil
	if open == nil {
		return
	}
	if len(open.section.Lines) == 0 {
		r.warn(open.line, ChordProEmptySection, open.text)
		return
	}
	r.out.Body.Sections = append(r.out.Body.Sections, open.section)
}

// addLine adds a line to the section being read, or to a new unlabelled
// section when none is open.
func (r *chordProReader) addLine(line SongChartLine) {
	if r.open == nil {
		r.open = &chordProSection{section: SongChartSection{Kind: SectionOther}}
	}
	r.open.section.Lines = append(r.open.section.Lines, line)
}

// lyricLine reads a line of lyrics with its inline [chords]. Each chord is
// played at the start of the text after it, up to the next chord; with no
// text there, it anchors to a single space. An empty [] is plain text.
func (r *chordProReader) lyricLine(n int, line string) SongChartLine {
	if strings.LastIndex(line, "[") > strings.LastIndex(line, "]") {
		r.warn(n, ChordProUnclosedChord, line)
		return SongChartLine{Runs: []SongChartRun{{Text: line}}}
	}
	var runs []SongChartRun
	current := SongChartRun{}
	flush := func() {
		if current.Anchor != nil && current.Text == "" {
			current.Text = " "
		}
		if current.Text != "" {
			runs = append(runs, current)
		}
	}
	rest := line
	for {
		open := strings.Index(rest, "[")
		if open < 0 {
			current.Text += rest
			break
		}
		closeAt := open + strings.Index(rest[open:], "]")
		symbol := rest[open+1 : closeAt]
		current.Text += rest[:open]
		rest = rest[closeAt+1:]
		if symbol == "" {
			current.Text += "[]"
			continue
		}
		flush()
		r.anchors++
		current = SongChartRun{Anchor: &ChordAnchor{ID: fmt.Sprintf("a%d", r.anchors), WrittenSymbol: symbol}}
	}
	flush()
	return SongChartLine{Runs: runs}
}

// ExportChordPro writes a draft as ChordPro text: its metadata, then each
// section after a blank line. Verses, choruses and bridges are written in
// their environments; ChordPro has none for the other kinds, so their lines
// are written bare, after their label as a comment.
func ExportChordPro(draft SongChartDraft) string {
	lines := []string{"{title: " + draft.Title + "}", "{artist: " + draft.Artist + "}"}
	if draft.ConcertKey != nil {
		lines = append(lines, "{key: "+*draft.ConcertKey+"}")
	}
	if draft.CapoFret != 0 {
		lines = append(lines, fmt.Sprintf("{capo: %d}", draft.CapoFret))
	}
	if draft.TempoBPM != nil {
		lines = append(lines, fmt.Sprintf("{tempo: %d}", *draft.TempoBPM))
	}
	if ts := draft.TimeSignature; ts != nil {
		lines = append(lines, fmt.Sprintf("{time: %d/%d}", ts.Beats, ts.BeatValue))
	}
	for _, section := range draft.Body.Sections {
		lines = append(lines, "")
		lines = append(lines, sectionChordPro(section)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func sectionChordPro(section SongChartSection) []string {
	var lines []string
	environment := slices.Contains([]SectionKind{SectionVerse, SectionChorus, SectionBridge}, section.Kind)
	switch {
	case environment && section.Label != nil:
		lines = append(lines, fmt.Sprintf("{start_of_%s: %s}", section.Kind, *section.Label))
	case environment:
		lines = append(lines, fmt.Sprintf("{start_of_%s}", section.Kind))
	case section.Label != nil:
		lines = append(lines, "{comment: "+*section.Label+"}")
	}
	for _, line := range section.Lines {
		lines = append(lines, lineChordPro(line))
	}
	if environment {
		lines = append(lines, fmt.Sprintf("{end_of_%s}", section.Kind))
	}
	return lines
}

func lineChordPro(line SongChartLine) string {
	if line.Comment != nil {
		return "{comment: " + *line.Comment + "}"
	}
	var b strings.Builder
	for _, run := range line.Runs {
		if run.Anchor != nil {
			b.WriteString("[" + run.Anchor.WrittenSymbol + "]")
		}
		b.WriteString(run.Text)
	}
	return strings.TrimRightFunc(b.String(), unicode.IsSpace)
}
