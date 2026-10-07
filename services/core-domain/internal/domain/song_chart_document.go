package domain

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// SectionKind is what part of a song a section is.
type SectionKind string

const (
	SectionVerse        SectionKind = "verse"
	SectionChorus       SectionKind = "chorus"
	SectionBridge       SectionKind = "bridge"
	SectionIntro        SectionKind = "intro"
	SectionOutro        SectionKind = "outro"
	SectionInstrumental SectionKind = "instrumental"
	SectionOther        SectionKind = "other"
)

var sectionKinds = map[SectionKind]bool{
	SectionVerse: true, SectionChorus: true, SectionBridge: true, SectionIntro: true,
	SectionOutro: true, SectionInstrumental: true, SectionOther: true,
}

// The bounds of a song chart document's parts.
const (
	MaxSectionLabelLength = 100
	MaxAnchorIDLength     = 64
	MaxChordSymbolLength  = 32
)

// SongChartDocument is a chart's lyrics with chords anchored to the words
// they fall on: sections of lyric lines and comments. It is read from and
// written as the song chart editor's ProseMirror JSON.
type SongChartDocument struct {
	Sections []SongChartSection
}

// SongChartSection is a part of the song with its lines. Label is the
// heading as written; nil shows none.
type SongChartSection struct {
	Kind  SectionKind
	Label *string
	Lines []SongChartLine
}

// SongChartLine is either a comment (Comment set) or a lyric line (Runs).
type SongChartLine struct {
	Comment *string
	Runs    []SongChartRun
}

// SongChartRun is a run of lyric text. With an Anchor, the chord is played
// at the start of the run.
type SongChartRun struct {
	Text   string
	Anchor *ChordAnchor
}

// ChordAnchor is a chord on a run of lyrics: the symbol as written, the
// catalog chord the server resolved it to, and the voicing the author
// picked. Both ids are nil when unset.
type ChordAnchor struct {
	ID                string
	WrittenSymbol     string
	ChordDefinitionID *string
	ChordVoicingID    *string
}

// AnchorPosition is where an anchor sits: its section and its line within
// the section, comments included, both numbered from 0.
type AnchorPosition struct {
	SectionIndex int
	LineIndex    int
}

// PositionedAnchor is an anchor with where it sits.
type PositionedAnchor struct {
	Anchor   ChordAnchor
	Position AnchorPosition
}

// Anchors lists the document's chord anchors in document order.
func (d SongChartDocument) Anchors() []PositionedAnchor {
	var anchors []PositionedAnchor
	for si, section := range d.Sections {
		for li, line := range section.Lines {
			for _, run := range line.Runs {
				if run.Anchor != nil {
					anchors = append(anchors, PositionedAnchor{Anchor: *run.Anchor, Position: AnchorPosition{SectionIndex: si, LineIndex: li}})
				}
			}
		}
	}
	return anchors
}

// WithAnchors returns a copy of the document with every anchor replaced by
// change's result. The document itself is left as it was.
func (d SongChartDocument) WithAnchors(change func(ChordAnchor) ChordAnchor) SongChartDocument {
	out := SongChartDocument{Sections: make([]SongChartSection, len(d.Sections))}
	for si, section := range d.Sections {
		section.Lines = append([]SongChartLine(nil), section.Lines...)
		for li, line := range section.Lines {
			line.Runs = append([]SongChartRun(nil), line.Runs...)
			for ri, run := range line.Runs {
				if run.Anchor != nil {
					changed := change(*run.Anchor)
					line.Runs[ri].Anchor = &changed
				}
			}
			section.Lines[li] = line
		}
		out.Sections[si] = section
	}
	return out
}

// pmNode is any node or mark of the ProseMirror JSON, as read. attrs is
// read by the node type that owns it.
type pmNode struct {
	Type    string          `json:"type"`
	Attrs   json.RawMessage `json:"attrs"`
	Content []pmNode        `json:"content"`
	Text    *string         `json:"text"`
	Marks   []pmNode        `json:"marks"`
}

type pmSectionAttrs struct {
	Kind  SectionKind `json:"kind"`
	Label *string     `json:"label"`
}

type pmAnchorAttrs struct {
	AnchorID          string  `json:"anchorId"`
	WrittenSymbol     string  `json:"writtenSymbol"`
	ChordDefinitionID *string `json:"chordDefinitionId"`
	ChordVoicingID    *string `json:"chordVoicingId"`
}

// ParseSongChartDocument reads a document from the song chart editor's
// ProseMirror JSON. Anything outside the chart's own schema (another node
// type, another mark, an empty section, line or run, a duplicate anchor id)
// is a ValidationError whose field is a path into the request body, e.g.
// "body/content/0/content/1/type".
func ParseSongChartDocument(raw []byte) (SongChartDocument, error) {
	var root pmNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return SongChartDocument{}, NewValidationError("body", "must be a song chart document")
	}
	p := documentParser{anchorIDs: map[string]bool{}}
	return p.document(root)
}

type documentParser struct {
	anchorIDs map[string]bool
}

func (p documentParser) document(root pmNode) (SongChartDocument, error) {
	if root.Type != "doc" {
		return SongChartDocument{}, NewValidationError("body/type", `must be "doc"`)
	}
	if len(root.Content) == 0 {
		return SongChartDocument{}, NewValidationError("body/content", "must hold at least one section")
	}
	doc := SongChartDocument{Sections: make([]SongChartSection, len(root.Content))}
	for i, node := range root.Content {
		section, err := p.section(node, fmt.Sprintf("body/content/%d", i))
		if err != nil {
			return SongChartDocument{}, err
		}
		doc.Sections[i] = section
	}
	return doc, nil
}

func (p documentParser) section(node pmNode, path string) (SongChartSection, error) {
	if node.Type != "section" {
		return SongChartSection{}, NewValidationError(path+"/type", `must be "section"`)
	}
	var attrs pmSectionAttrs
	if len(node.Attrs) == 0 || json.Unmarshal(node.Attrs, &attrs) != nil {
		return SongChartSection{}, NewValidationError(path+"/attrs", "must give the section's kind and label")
	}
	if !sectionKinds[attrs.Kind] {
		return SongChartSection{}, NewValidationError(path+"/attrs/kind", "must be verse, chorus, bridge, intro, outro, instrumental or other")
	}
	if attrs.Label != nil && utf8.RuneCountInString(*attrs.Label) > MaxSectionLabelLength {
		return SongChartSection{}, NewValidationError(path+"/attrs/label", "must be at most 100 characters")
	}
	if len(node.Content) == 0 {
		return SongChartSection{}, NewValidationError(path+"/content", "must hold at least one line")
	}
	section := SongChartSection{Kind: attrs.Kind, Label: attrs.Label, Lines: make([]SongChartLine, len(node.Content))}
	for i, child := range node.Content {
		line, err := p.line(child, fmt.Sprintf("%s/content/%d", path, i))
		if err != nil {
			return SongChartSection{}, err
		}
		section.Lines[i] = line
	}
	return section, nil
}

func (p documentParser) line(node pmNode, path string) (SongChartLine, error) {
	switch node.Type {
	case "comment":
		text, err := p.commentText(node, path)
		return SongChartLine{Comment: &text}, err
	case "lyricLine":
		if len(node.Content) == 0 {
			return SongChartLine{}, NewValidationError(path+"/content", "must hold at least one text run")
		}
		runs := make([]SongChartRun, len(node.Content))
		for i, child := range node.Content {
			run, err := p.run(child, fmt.Sprintf("%s/content/%d", path, i))
			if err != nil {
				return SongChartLine{}, err
			}
			runs[i] = run
		}
		return SongChartLine{Runs: runs}, nil
	default:
		return SongChartLine{}, NewValidationError(path+"/type", `must be "lyricLine" or "comment"`)
	}
}

func (p documentParser) commentText(node pmNode, path string) (string, error) {
	if len(node.Content) == 0 {
		return "", NewValidationError(path+"/content", "must hold the comment's text")
	}
	var text string
	for i, child := range node.Content {
		childPath := fmt.Sprintf("%s/content/%d", path, i)
		if len(child.Marks) > 0 {
			return "", NewValidationError(childPath+"/marks", "a comment carries no chords")
		}
		part, err := runText(child, childPath)
		if err != nil {
			return "", err
		}
		text += part
	}
	return text, nil
}

func runText(node pmNode, path string) (string, error) {
	if node.Type != "text" {
		return "", NewValidationError(path+"/type", `must be "text"`)
	}
	if node.Text == nil || *node.Text == "" {
		return "", NewValidationError(path+"/text", "must not be empty")
	}
	return *node.Text, nil
}

func (p documentParser) run(node pmNode, path string) (SongChartRun, error) {
	text, err := runText(node, path)
	if err != nil {
		return SongChartRun{}, err
	}
	run := SongChartRun{Text: text}
	switch len(node.Marks) {
	case 0:
		return run, nil
	case 1:
		anchor, err := p.anchor(node.Marks[0], path+"/marks/0")
		run.Anchor = &anchor
		return run, err
	default:
		return SongChartRun{}, NewValidationError(path+"/marks", "must hold at most one chord anchor")
	}
}

func (p documentParser) anchor(mark pmNode, path string) (ChordAnchor, error) {
	if mark.Type != "chordAnchor" {
		return ChordAnchor{}, NewValidationError(path+"/type", `must be "chordAnchor"`)
	}
	var attrs pmAnchorAttrs
	if len(mark.Attrs) == 0 || json.Unmarshal(mark.Attrs, &attrs) != nil {
		return ChordAnchor{}, NewValidationError(path+"/attrs", "must give the anchor's id and written symbol")
	}
	if n := utf8.RuneCountInString(attrs.AnchorID); n == 0 || n > MaxAnchorIDLength {
		return ChordAnchor{}, NewValidationError(path+"/attrs/anchorId", "must be 1 to 64 characters")
	}
	if p.anchorIDs[attrs.AnchorID] {
		return ChordAnchor{}, NewValidationError(path+"/attrs/anchorId", "must be unique within the document")
	}
	p.anchorIDs[attrs.AnchorID] = true
	if n := utf8.RuneCountInString(attrs.WrittenSymbol); n == 0 || n > MaxChordSymbolLength {
		return ChordAnchor{}, NewValidationError(path+"/attrs/writtenSymbol", "must be 1 to 32 characters")
	}
	return ChordAnchor{
		ID: attrs.AnchorID, WrittenSymbol: attrs.WrittenSymbol,
		ChordDefinitionID: attrs.ChordDefinitionID, ChordVoicingID: attrs.ChordVoicingID,
	}, nil
}

// The ProseMirror JSON as written: every attribute is present, null when
// unset, and a run without an anchor has no marks.
type pmDocOut struct {
	Type    string         `json:"type"`
	Content []pmSectionOut `json:"content"`
}

type pmSectionOut struct {
	Type    string         `json:"type"`
	Attrs   pmSectionAttrs `json:"attrs"`
	Content []pmLineOut    `json:"content"`
}

type pmLineOut struct {
	Type    string      `json:"type"`
	Content []pmTextOut `json:"content"`
}

type pmTextOut struct {
	Type  string        `json:"type"`
	Text  string        `json:"text"`
	Marks []pmAnchorOut `json:"marks,omitempty"`
}

type pmAnchorOut struct {
	Type  string        `json:"type"`
	Attrs pmAnchorAttrs `json:"attrs"`
}

// MarshalJSON writes the document as the song chart editor's ProseMirror
// JSON, the same shape ParseSongChartDocument reads.
func (d SongChartDocument) MarshalJSON() ([]byte, error) {
	out := pmDocOut{Type: "doc", Content: make([]pmSectionOut, len(d.Sections))}
	for si, section := range d.Sections {
		lines := make([]pmLineOut, len(section.Lines))
		for li, line := range section.Lines {
			lines[li] = lineOut(line)
		}
		out.Content[si] = pmSectionOut{Type: "section", Attrs: pmSectionAttrs{Kind: section.Kind, Label: section.Label}, Content: lines}
	}
	return json.Marshal(out)
}

func lineOut(line SongChartLine) pmLineOut {
	if line.Comment != nil {
		return pmLineOut{Type: "comment", Content: []pmTextOut{{Type: "text", Text: *line.Comment}}}
	}
	runs := make([]pmTextOut, len(line.Runs))
	for i, run := range line.Runs {
		runs[i] = pmTextOut{Type: "text", Text: run.Text}
		if a := run.Anchor; a != nil {
			runs[i].Marks = []pmAnchorOut{{Type: "chordAnchor", Attrs: pmAnchorAttrs{
				AnchorID: a.ID, WrittenSymbol: a.WrittenSymbol, ChordDefinitionID: a.ChordDefinitionID, ChordVoicingID: a.ChordVoicingID,
			}}}
		}
	}
	return pmLineOut{Type: "lyricLine", Content: runs}
}
