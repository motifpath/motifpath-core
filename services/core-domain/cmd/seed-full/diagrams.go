package main

import (
	"context"
	"fmt"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// seededDiagrams is every Instrument and Diagram seedInstrumentsAndDiagrams
// creates, named for the scenario each one exists to show.
type seededDiagrams struct {
	guitar domain.Instrument
	bass   domain.Instrument
	piano  domain.Instrument

	// Basic templates, owned by the template curator and written in every
	// offered language, text annotations included.
	pentatonicPos1   domain.Diagram // regions, a note on the root, star roots in their own colour
	pentatonicPos2   domain.Diagram // overlaps position 1 on six cells: overlay and merge it onto position 1
	pentatonicJoined domain.Diagram // two shapes joined into one, each shown as its own captioned region
	cMajorOpen       domain.Diagram // note-name labels, square markers, open strings (fret 0)
	eMajorChord      domain.Diagram // custom marker labels (finger numbers), a string-bounded region
	bassEMajor       domain.Diagram // a template for another fretted instrument

	// Custom diagrams, each owned by the teacher who made it.
	teacherLick     domain.Diagram // English only, labels hidden, per-marker colours, a playback order
	otherTeacherArp domain.Diagram // another teacher's: read-only for the main seed teacher
}

// fretted is one fretted position in a seeded diagram. customLabel and note,
// when set, are written in every language the diagram is written in.
type fretted struct {
	str, fret          int
	interval, noteName string
	customLabel        domain.LocalizedText
	note               domain.LocalizedText
}

// frettedPositions turns specs into diagram positions, drawing roots as red
// stars so they stand out, and every other marker in shape (the diagram's
// general colour unless the marker has its own).
func frettedPositions(specs []fretted, shape domain.PositionShape) []domain.Position {
	positions := make([]domain.Position, len(specs))
	for i, spec := range specs {
		str, fret := spec.str, spec.fret
		position := domain.Position{Interval: spec.interval, NoteName: spec.noteName, Shape: shape, String: &str, Fret: &fret,
			CustomLabel: spec.customLabel, Note: spec.note}
		if spec.interval == "R" {
			position.Shape = domain.PositionShapeStar
			position.Color = stringPtr("#EF4444")
		}
		positions[i] = position
	}
	return positions
}

// fretBand is a highlighted region over frets from..to, on every string when
// the string bounds are zero.
func fretBand(from, to int, en, ptBR string, color *string, strings ...int) domain.Region {
	region := domain.Region{FretStart: &from, FretEnd: &to, Description: domain.LocalizedText{"en": en, "pt_BR": ptBR}, Color: color}
	if len(strings) == 2 {
		region.StringStart, region.StringEnd = &strings[0], &strings[1]
	}
	return region
}

func both(en, ptBR string) domain.LocalizedText {
	return domain.LocalizedText{"en": en, "pt_BR": ptBR}
}

// A minor pentatonic, position 1 (frets 5-8) and position 2 (frets 7-10).
var (
	pentatonicPos1Cells = []fretted{
		{6, 5, "R", "A", nil, both("Start here: the root on the 6th string.", "Comece aqui: a fundamental na 6ª corda.")},
		{6, 8, "b3", "C", nil, nil}, {5, 5, "4", "D", nil, nil}, {5, 7, "5", "E", nil, nil},
		{4, 5, "b7", "G", nil, nil}, {4, 7, "R", "A", nil, nil}, {3, 5, "b3", "C", nil, nil},
		{3, 7, "4", "D", nil, nil}, {2, 5, "5", "E", nil, nil}, {2, 8, "b7", "G", nil, nil},
		{1, 5, "R", "A", nil, nil}, {1, 8, "b3", "C", nil, nil},
	}
	pentatonicPos2Cells = []fretted{
		{6, 8, "b3", "C", nil, nil}, {6, 10, "4", "D", nil, nil}, {5, 7, "5", "E", nil, nil},
		{5, 10, "b7", "G", nil, nil}, {4, 7, "R", "A", nil, nil}, {4, 10, "b3", "C", nil, nil},
		{3, 7, "4", "D", nil, nil}, {3, 9, "5", "E", nil, nil}, {2, 8, "b7", "G", nil, nil},
		{2, 10, "R", "A", nil, nil}, {1, 8, "b3", "C", nil, nil}, {1, 10, "4", "D", nil, nil},
	}
)

// joinedCells is position 1 followed by the cells position 2 adds, each
// physical cell once.
func joinedCells() []fretted {
	seen := map[[2]int]bool{}
	var cells []fretted
	for _, cell := range append(append([]fretted{}, pentatonicPos1Cells...), pentatonicPos2Cells...) {
		key := [2]int{cell.str, cell.fret}
		if seen[key] {
			continue
		}
		seen[key] = true
		cell.note = nil
		cells = append(cells, cell)
	}
	return cells
}

// seedInstruments creates the guitar, bass and piano Instruments.
func seedInstruments(ctx context.Context, teacher domain.User, instrumentSvc *application.InstrumentService) (guitar, bass, piano domain.Instrument, err error) {
	six, four := 6, 4
	if guitar, err = instrumentSvc.CreateInstrument(ctx, teacher, map[string]string{"en": "Guitar", "pt_BR": "Violão"}, domain.InstrumentFamilyFretted,
		&six, []string{"E", "A", "D", "G", "B", "E"}, nil); err != nil {
		return guitar, bass, piano, fmt.Errorf("create guitar: %w", err)
	}
	if bass, err = instrumentSvc.CreateInstrument(ctx, teacher, map[string]string{"en": "Bass", "pt_BR": "Contrabaixo"}, domain.InstrumentFamilyFretted,
		&four, []string{"E", "A", "D", "G"}, nil); err != nil {
		return guitar, bass, piano, fmt.Errorf("create bass: %w", err)
	}
	if piano, err = instrumentSvc.CreateInstrument(ctx, teacher, map[string]string{"en": "Piano", "pt_BR": "Piano"}, domain.InstrumentFamilyKeyboard,
		nil, nil, &domain.KeyRange{Lowest: "A0", Highest: "C8"}); err != nil {
		return guitar, bass, piano, fmt.Errorf("create piano: %w", err)
	}
	return guitar, bass, piano, nil
}

// diagramSpec is one diagram to seed, and where to keep it once created.
type diagramSpec struct {
	into       *domain.Diagram
	owner      domain.User
	instrument domain.Instrument
	names      map[string]string
	positions  []domain.Position
	skill      string
	concept    string
	opts       domain.DiagramOptions
}

func basicOptions(root string, labels domain.LabelDisplay, color *string, regions ...domain.Region) domain.DiagramOptions {
	return domain.DiagramOptions{RootNote: &root, LabelDisplay: labels, Color: color, Kind: domain.DiagramKindBasic, Regions: regions}
}

// bluesLick is a lick played in order: each marker has its own colour and a
// place in the playback sequence, and the diagram hides its labels so the
// shape alone shows. Its one note is in English only, like the diagram.
func bluesLick() []domain.Position {
	lick := frettedPositions([]fretted{
		{2, 8, "b7", "G", nil, domain.LocalizedText{"en": "Bend this one up a whole step."}},
		{2, 5, "5", "E", nil, nil}, {3, 7, "4", "D", nil, nil}, {3, 5, "b3", "C", nil, nil}, {4, 7, "R", "A", nil, nil},
	}, domain.PositionShapeDot)
	for i, color := range []string{"#EC4899", "#F59E0B", "#22C55E", "#06B6D4", "#EF4444"} {
		index := i
		lick[i].SequenceIndex = &index
		lick[i].Color = stringPtr(color)
	}
	return lick
}

// seedInstrumentsAndDiagrams creates the guitar, bass and piano Instruments
// and a library of diagrams covering what the diagram editor and viewer
// support: basic templates (owned by curator, in every language) and custom
// diagrams (owned by the teacher who made them), with colours, every marker
// shape and label display, custom labels, notes, regions and a playback
// order. otherTeacher owns one custom diagram so a teacher can open another
// teacher's diagram and see it read-only. The piano gets no diagram: only
// fretted diagrams can be drawn yet.
func seedInstrumentsAndDiagrams(ctx context.Context, teacher, otherTeacher, curator domain.User, instrumentSvc *application.InstrumentService, diagramSvc *application.DiagramService, classifier *classificationSeeder) (seededDiagrams, error) {
	var seeded seededDiagrams
	var err error
	if seeded.guitar, seeded.bass, seeded.piano, err = seedInstruments(ctx, teacher, instrumentSvc); err != nil {
		return seededDiagrams{}, err
	}

	blue, green, amber, violet := stringPtr("#3B82F6"), stringPtr("#22C55E"), stringPtr("#F59E0B"), stringPtr("#8B5CF6")
	lickRoot, arpeggioRoot := "A", "G"
	specs := []diagramSpec{
		{&seeded.pentatonicPos1, curator, seeded.guitar,
			map[string]string{"en": "A Minor Pentatonic — Position 1", "pt_BR": "Pentatônica menor de Lá — Posição 1"},
			frettedPositions(pentatonicPos1Cells, domain.PositionShapeDot), "Scales", "Pentatonic scale shapes",
			basicOptions("A", domain.LabelDisplayInterval, blue, fretBand(5, 8, "Position 1", "Posição 1", nil))},
		{&seeded.pentatonicPos2, curator, seeded.guitar,
			map[string]string{"en": "A Minor Pentatonic — Position 2", "pt_BR": "Pentatônica menor de Lá — Posição 2"},
			frettedPositions(pentatonicPos2Cells, domain.PositionShapeDot), "Scales", "Pentatonic scale shapes",
			basicOptions("A", domain.LabelDisplayInterval, green)},
		{&seeded.pentatonicJoined, curator, seeded.guitar,
			map[string]string{"en": "A Minor Pentatonic — Positions 1 and 2", "pt_BR": "Pentatônica menor de Lá — Posições 1 e 2"},
			frettedPositions(joinedCells(), domain.PositionShapeDot), "Scales", "Pentatonic scale shapes",
			basicOptions("A", domain.LabelDisplayInterval, blue,
				fretBand(5, 8, "Shape 1", "Desenho 1", blue),
				fretBand(7, 10, "Shape 2", "Desenho 2", green))},
		{&seeded.cMajorOpen, curator, seeded.guitar,
			map[string]string{"en": "C Major Scale — Open Position", "pt_BR": "Escala de Dó maior — Posição aberta"},
			frettedPositions([]fretted{
				{5, 3, "R", "C", nil, nil}, {4, 0, "2", "D", nil, nil}, {4, 2, "3", "E", nil, nil},
				{4, 3, "4", "F", nil, nil}, {3, 0, "5", "G", nil, nil}, {3, 2, "6", "A", nil, nil},
				{2, 0, "7", "B", nil, nil}, {2, 1, "R", "C", nil, both("The octave: same note, one octave up.", "A oitava: a mesma nota, uma oitava acima.")},
			}, domain.PositionShapeSquare), "Scales", "Major scale fingerings",
			basicOptions("C", domain.LabelDisplayNote, amber)},
		{&seeded.eMajorChord, curator, seeded.guitar,
			map[string]string{"en": "E Major Chord — Open Shape", "pt_BR": "Acorde de Mi maior — Forma aberta"},
			frettedPositions([]fretted{
				{6, 0, "R", "E", nil, nil},
				{5, 2, "5", "B", both("2", "2"), nil},
				{4, 2, "R", "E", both("3", "3"), nil},
				{3, 1, "3", "G#", both("1", "1"), both("The major third: it makes the chord major.", "A terça maior: é ela que torna o acorde maior.")},
				{2, 0, "5", "B", nil, nil},
				{1, 0, "R", "E", nil, nil},
			}, domain.PositionShapeDot), "Chords", "Open chord shapes",
			basicOptions("E", domain.LabelDisplayInterval, violet, fretBand(0, 2, "Fretted notes", "Notas presas", violet, 3, 5))},
		{&seeded.bassEMajor, curator, seeded.bass,
			map[string]string{"en": "E Major Scale — Bass, Open Position", "pt_BR": "Escala de Mi maior — Baixo, posição aberta"},
			frettedPositions([]fretted{
				{4, 0, "R", "E", nil, nil}, {4, 2, "2", "F#", nil, nil}, {4, 4, "3", "G#", nil, nil},
				{3, 0, "4", "A", nil, nil}, {3, 2, "5", "B", nil, nil}, {3, 4, "6", "C#", nil, nil},
				{2, 1, "7", "D#", nil, nil}, {2, 2, "R", "E", nil, nil},
			}, domain.PositionShapeDot), "Scales", "Major scale fingerings",
			basicOptions("E", domain.LabelDisplayInterval, blue)},
		{&seeded.teacherLick, teacher, seeded.guitar, map[string]string{"en": "Blues Lick in A"}, bluesLick(), "Improvisation", "Phrasing",
			domain.DiagramOptions{RootNote: &lickRoot, LabelDisplay: domain.LabelDisplayHidden, Kind: domain.DiagramKindCustom}},
		{&seeded.otherTeacherArp, otherTeacher, seeded.guitar,
			map[string]string{"en": "G Major Arpeggio — Open", "pt_BR": "Arpejo de Sol maior — Aberto"},
			frettedPositions([]fretted{
				{6, 3, "R", "G", nil, nil}, {5, 2, "3", "B", nil, nil}, {4, 0, "5", "D", nil, nil},
				{3, 0, "R", "G", nil, nil}, {2, 0, "3", "B", nil, nil}, {1, 3, "R", "G", nil, nil},
			}, domain.PositionShapeDot), "Arpeggios", "Major triads",
			domain.DiagramOptions{RootNote: &arpeggioRoot, LabelDisplay: domain.LabelDisplayInterval, Color: green, Kind: domain.DiagramKindCustom}},
	}
	for _, spec := range specs {
		if *spec.into, err = seedDiagram(ctx, diagramSvc, classifier, spec); err != nil {
			return seededDiagrams{}, err
		}
	}
	return seeded, nil
}

func seedDiagram(ctx context.Context, diagramSvc *application.DiagramService, classifier *classificationSeeder, spec diagramSpec) (domain.Diagram, error) {
	skillID, err := classifier.skillID(ctx, spec.skill)
	if err != nil {
		return domain.Diagram{}, err
	}
	conceptID, err := classifier.conceptID(ctx, spec.concept)
	if err != nil {
		return domain.Diagram{}, err
	}
	diagram, err := diagramSvc.CreateDiagram(ctx, spec.owner, spec.instrument.ID, spec.names, spec.positions, []string{skillID}, []string{conceptID}, spec.opts)
	if err != nil {
		return domain.Diagram{}, fmt.Errorf("create diagram %q: %w", spec.names["en"], err)
	}
	return diagram, nil
}
