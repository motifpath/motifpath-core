package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// SongChartService authors, publishes and serves song charts: lyrics with
// chords anchored to the words they fall on. Only admins author and publish
// charts; any signed-in user reads a published one.
type SongChartService struct {
	charts    ports.SongChartRepository
	chords    ports.ChordCatalogRepository
	diagrams  ports.DiagramRepository
	languages ports.LanguageRepository
	newID     func() string
	now       func() time.Time
}

func NewSongChartService(
	charts ports.SongChartRepository,
	chords ports.ChordCatalogRepository,
	diagrams ports.DiagramRepository,
	languages ports.LanguageRepository,
	newID func() string,
	now func() time.Time,
) *SongChartService {
	return &SongChartService{charts: charts, chords: chords, diagrams: diagrams, languages: languages, newID: newID, now: now}
}

// SongChartInput is what an author writes in a chart's draft. Anchors'
// chord ids are ignored: the server resolves every written symbol itself.
type SongChartInput struct {
	Title           string
	Artist          string
	Language        string
	ConcertKey      *string
	CapoFret        int
	TempoBPM        *int
	TimeSignature   *domain.TimeSignature
	RightsConfirmed bool
	Body            domain.SongChartDocument
}

// LearnerSongChart is a chart as a learner reads it: the lyrics with their
// chords, and every chord used with its voicings and their diagrams.
// RevisionNumber is nil in a preview of the draft.
type LearnerSongChart struct {
	SongChartID       string
	RevisionNumber    *int
	Title             string
	Artist            string
	Language          string
	ConcertKey        *string
	CapoFret          int
	TempoBPM          *int
	TimeSignature     *domain.TimeSignature
	TuningFingerprint string
	Body              domain.SongChartDocument
	Chords            []domain.ChordDefinition
	Diagrams          []domain.Diagram
}

func requireAdmin(caller domain.User) error {
	if caller.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return nil
}

// Create starts a chart with its first draft. It is never published yet.
func (s *SongChartService) Create(ctx context.Context, caller domain.User, in SongChartInput) (domain.SongChart, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.SongChart{}, err
	}
	now := s.now()
	draft, err := s.buildDraft(ctx, caller, nil, in, now)
	if err != nil {
		return domain.SongChart{}, err
	}
	chart := domain.SongChart{ID: s.newID(), Status: domain.SongChartDraftStatus, Draft: draft, CreatedBy: caller.ID, CreatedAt: now}
	if err := s.charts.Create(ctx, chart); err != nil {
		return domain.SongChart{}, err
	}
	return chart, nil
}

// UpdateDraft replaces a chart's draft. What learners read doesn't change
// until the draft is published again.
func (s *SongChartService) UpdateDraft(ctx context.Context, caller domain.User, id string, in SongChartInput) (domain.SongChart, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.SongChart{}, err
	}
	chart, err := s.charts.GetByID(ctx, id)
	if err != nil {
		return domain.SongChart{}, err
	}
	draft, err := s.buildDraft(ctx, caller, chart.Draft.RightsConfirmation, in, s.now())
	if err != nil {
		return domain.SongChart{}, err
	}
	chart.Draft = draft
	if err := s.charts.Save(ctx, chart); err != nil {
		return domain.SongChart{}, err
	}
	return chart, nil
}

// buildDraft validates in, resolves its anchors and carries the previous
// rights confirmation over, so confirming again doesn't change who did.
func (s *SongChartService) buildDraft(ctx context.Context, caller domain.User, confirmation *domain.RightsConfirmation, in SongChartInput, now time.Time) (domain.SongChartDraft, error) {
	draft := domain.SongChartDraft{
		Title: in.Title, Artist: in.Artist, Language: in.Language, ConcertKey: in.ConcertKey,
		CapoFret: in.CapoFret, TempoBPM: in.TempoBPM, TimeSignature: in.TimeSignature,
		RightsConfirmation: confirmation, UpdatedBy: caller.ID, UpdatedAt: now,
	}
	if err := draft.Validate(); err != nil {
		return domain.SongChartDraft{}, err
	}
	if _, err := s.languages.GetByCode(ctx, in.Language); errors.Is(err, domain.ErrNotFound) {
		return domain.SongChartDraft{}, domain.NewValidationError("language", "must be an existing language code")
	} else if err != nil {
		return domain.SongChartDraft{}, err
	}
	body, warnings, err := s.resolveAnchors(ctx, in.Body)
	if err != nil {
		return domain.SongChartDraft{}, err
	}
	draft.Body, draft.Warnings = body, warnings
	draft.SetRightsConfirmed(in.RightsConfirmed, caller.ID, now)
	return draft, nil
}

// Get returns a chart as its authors see it.
func (s *SongChartService) Get(ctx context.Context, caller domain.User, id string) (domain.SongChart, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.SongChart{}, err
	}
	return s.charts.GetByID(ctx, id)
}

// List returns a page of charts. Admins list every chart, to author them;
// teachers list only the published ones, to embed them in lesson content,
// and must ask for that status. Students list none.
func (s *SongChartService) List(ctx context.Context, caller domain.User, filter domain.SongChartFilter, page domain.PageRequest) (domain.Page[domain.SongChart], error) {
	if err := canList(caller, filter); err != nil {
		return domain.Page[domain.SongChart]{}, err
	}
	// Nothing checks an enum query parameter before it gets here, so an
	// unknown status would otherwise match no chart and read as an empty list.
	if filter.Status != nil && !filter.Status.Valid() {
		return domain.Page[domain.SongChart]{}, domain.NewValidationError("status", "must be draft, published or withdrawn")
	}
	return s.charts.List(ctx, filter, page)
}

func canList(caller domain.User, filter domain.SongChartFilter) error {
	switch caller.Role {
	case domain.RoleAdmin:
		return nil
	case domain.RoleTeacher:
		if filter.Status != nil && *filter.Status == domain.SongChartPublished {
			return nil
		}
	case domain.RoleStudent:
	}
	return domain.ErrForbidden
}

// Publish publishes the draft as the chart's next revision. Its chords are
// resolved again first, since the catalog may have changed since the draft
// was saved.
func (s *SongChartService) Publish(ctx context.Context, caller domain.User, id string) (domain.SongChartRevision, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.SongChartRevision{}, err
	}
	chart, err := s.charts.GetByID(ctx, id)
	if err != nil {
		return domain.SongChartRevision{}, err
	}
	body, warnings, err := s.resolveAnchors(ctx, chart.Draft.Body)
	if err != nil {
		return domain.SongChartRevision{}, err
	}
	chart.Draft.Body, chart.Draft.Warnings = body, warnings
	published, rev, err := chart.Publish(caller.ID, s.now())
	if err != nil {
		return domain.SongChartRevision{}, err
	}
	if err := s.charts.Publish(ctx, published, rev); err != nil {
		return domain.SongChartRevision{}, err
	}
	return rev, nil
}

// Withdraw takes a published chart away from learners.
func (s *SongChartService) Withdraw(ctx context.Context, caller domain.User, id, reason string) (domain.SongChart, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.SongChart{}, err
	}
	chart, err := s.charts.GetByID(ctx, id)
	if err != nil {
		return domain.SongChart{}, err
	}
	withdrawn, err := chart.Withdraw(caller.ID, s.now(), reason)
	if err != nil {
		return domain.SongChart{}, err
	}
	if err := s.charts.Save(ctx, withdrawn); err != nil {
		return domain.SongChart{}, err
	}
	return withdrawn, nil
}

// ListRevisions returns a chart's published revisions, newest first.
func (s *SongChartService) ListRevisions(ctx context.Context, caller domain.User, id string) ([]domain.SongChartRevision, error) {
	if err := requireAdmin(caller); err != nil {
		return nil, err
	}
	if _, err := s.charts.GetByID(ctx, id); err != nil {
		return nil, err
	}
	return s.charts.ListRevisions(ctx, id)
}

// GetPublished returns the chart's latest published revision for reading.
// A chart that isn't published (never, or withdrawn) is not found.
func (s *SongChartService) GetPublished(ctx context.Context, _ domain.User, id string) (LearnerSongChart, error) {
	chart, err := s.charts.GetByID(ctx, id)
	if err != nil {
		return LearnerSongChart{}, err
	}
	if chart.Status != domain.SongChartPublished || chart.PublishedRevision == nil {
		return LearnerSongChart{}, domain.ErrNotFound
	}
	rev, err := s.charts.GetRevision(ctx, id, chart.PublishedRevision.Number)
	if err != nil {
		return LearnerSongChart{}, err
	}
	number := rev.Number
	learner := LearnerSongChart{
		SongChartID: id, RevisionNumber: &number, Title: rev.Title, Artist: rev.Artist, Language: rev.Language,
		ConcertKey: rev.ConcertKey, CapoFret: rev.CapoFret, TempoBPM: rev.TempoBPM, TimeSignature: rev.TimeSignature,
		TuningFingerprint: rev.TuningFingerprint, Body: rev.Body,
	}
	return s.withChords(ctx, learner)
}

// Preview returns the draft as a learner would read it.
func (s *SongChartService) Preview(ctx context.Context, caller domain.User, id string) (LearnerSongChart, error) {
	if err := requireAdmin(caller); err != nil {
		return LearnerSongChart{}, err
	}
	chart, err := s.charts.GetByID(ctx, id)
	if err != nil {
		return LearnerSongChart{}, err
	}
	d := chart.Draft
	learner := LearnerSongChart{
		SongChartID: id, Title: d.Title, Artist: d.Artist, Language: d.Language,
		ConcertKey: d.ConcertKey, CapoFret: d.CapoFret, TempoBPM: d.TempoBPM, TimeSignature: d.TimeSignature,
		TuningFingerprint: domain.StandardGuitarTuningFingerprint, Body: d.Body,
	}
	return s.withChords(ctx, learner)
}

// withChords adds every chord the body's anchors resolve to, once each in
// document order, with its active voicings best first and then any picked
// voicing that has since been withdrawn; and the diagram of every voicing.
func (s *SongChartService) withChords(ctx context.Context, learner LearnerSongChart) (LearnerSongChart, error) {
	refs := chordRefsOf(learner.Body)
	chords, err := s.chords.GetChords(ctx, refs.chordIDs)
	if err != nil {
		return LearnerSongChart{}, err
	}
	picked, err := s.chords.GetVoicings(ctx, refs.pickedIDs)
	if err != nil {
		return LearnerSongChart{}, err
	}
	for _, id := range refs.chordIDs {
		if chord, ok := chords[id]; ok {
			learner.Chords = append(learner.Chords, withPickedVoicings(chord, refs.pickedByChord[id], picked))
		}
	}
	learner.Diagrams, err = s.voicingDiagrams(ctx, learner.Chords)
	return learner, err
}

// chordRefs is which chords a document's anchors resolve to, in document
// order, and which voicings they picked.
type chordRefs struct {
	chordIDs      []string
	pickedIDs     []string
	pickedByChord map[string][]string
}

func chordRefsOf(body domain.SongChartDocument) chordRefs {
	refs := chordRefs{pickedByChord: map[string][]string{}}
	for _, a := range body.Anchors() {
		chordID := a.Anchor.ChordDefinitionID
		if chordID == nil {
			continue
		}
		if _, seen := refs.pickedByChord[*chordID]; !seen {
			refs.chordIDs = append(refs.chordIDs, *chordID)
			refs.pickedByChord[*chordID] = nil
		}
		if v := a.Anchor.ChordVoicingID; v != nil {
			refs.pickedByChord[*chordID] = append(refs.pickedByChord[*chordID], *v)
			refs.pickedIDs = append(refs.pickedIDs, *v)
		}
	}
	return refs
}

// withPickedVoicings appends the picked voicings the chord no longer offers,
// after its active ones.
func withPickedVoicings(chord domain.ChordDefinition, pickedIDs []string, picked map[string]domain.ChordVoicing) domain.ChordDefinition {
	offered := map[string]bool{}
	for _, v := range chord.Voicings {
		offered[v.ID] = true
	}
	for _, id := range pickedIDs {
		if v, ok := picked[id]; ok && !offered[id] {
			chord.Voicings = append(chord.Voicings, v)
			offered[id] = true
		}
	}
	return chord
}

// voicingDiagrams returns the diagram of every voicing of chords, in order.
func (s *SongChartService) voicingDiagrams(ctx context.Context, chords []domain.ChordDefinition) ([]domain.Diagram, error) {
	var ids []string
	for _, c := range chords {
		for _, v := range c.Voicings {
			ids = append(ids, v.DiagramID)
		}
	}
	found, err := s.diagrams.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	var diagrams []domain.Diagram
	for _, id := range ids {
		if d, ok := found[id]; ok {
			diagrams = append(diagrams, d)
		}
	}
	return diagrams, nil
}

// ChordProImportResult is a chart after a ChordPro import, with what the
// import skipped.
type ChordProImportResult struct {
	Chart    domain.SongChart
	Warnings []domain.ChordProWarning
}

// ImportChordPro replaces a chart's draft from ChordPro text: its body, and
// every metadata field the text sets. Fields the text doesn't set keep their
// draft values, as does the rights confirmation. Anchors are resolved as on
// any other edit of the draft.
func (s *SongChartService) ImportChordPro(ctx context.Context, caller domain.User, id, text string) (ChordProImportResult, error) {
	if err := requireAdmin(caller); err != nil {
		return ChordProImportResult{}, err
	}
	imported, err := readChordPro(text)
	if err != nil {
		return ChordProImportResult{}, err
	}
	chart, err := s.charts.GetByID(ctx, id)
	if err != nil {
		return ChordProImportResult{}, err
	}
	in := importedInput(chart.Draft, imported.Metadata, imported.Body)
	draft, err := s.buildDraft(ctx, caller, chart.Draft.RightsConfirmation, in, s.now())
	if err != nil {
		return ChordProImportResult{}, err
	}
	chart.Draft = draft
	if err := s.charts.Save(ctx, chart); err != nil {
		return ChordProImportResult{}, err
	}
	return ChordProImportResult{Chart: chart, Warnings: imported.Warnings}, nil
}

// ReadChordPro reads ChordPro text as a song chart and saves nothing: the
// details the text sets, the lyrics with their chords, and what was skipped.
// Anchors carry only their written symbols; they are resolved when the chart
// is saved.
func (s *SongChartService) ReadChordPro(_ context.Context, caller domain.User, text string) (domain.ChordProImport, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.ChordProImport{}, err
	}
	return readChordPro(text)
}

// readChordPro reads text within an import's bounds, with a body that holds
// a lyric line and fits a chart's document.
func readChordPro(text string) (domain.ChordProImport, error) {
	if utf8.RuneCountInString(text) > domain.MaxChordProTextLength {
		return domain.ChordProImport{}, domain.NewValidationError("body", "must be at most 100000 characters")
	}
	imported := domain.ImportChordPro(text)
	body, err := checkedImportBody(imported.Body)
	if err != nil {
		return domain.ChordProImport{}, err
	}
	imported.Body = body
	return imported, nil
}

// checkedImportBody checks an imported body against the bounds of the
// chart's own document, the same way a body sent as JSON is checked, so a
// label or chord symbol too long for a chart is refused, never stored.
func checkedImportBody(body domain.SongChartDocument) (domain.SongChartDocument, error) {
	if !body.HasLyrics() {
		return domain.SongChartDocument{}, domain.NewValidationError("body", "must hold at least one lyric line")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.SongChartDocument{}, err
	}
	return domain.ParseSongChartDocument(raw)
}

// importedInput is the draft with the imported body and every metadata
// field the import set.
func importedInput(draft domain.SongChartDraft, m domain.ChordProMetadata, body domain.SongChartDocument) SongChartInput {
	in := SongChartInput{
		Title: draft.Title, Artist: draft.Artist, Language: draft.Language, ConcertKey: draft.ConcertKey,
		CapoFret: draft.CapoFret, TempoBPM: draft.TempoBPM, TimeSignature: draft.TimeSignature,
		RightsConfirmed: draft.RightsConfirmation != nil, Body: body,
	}
	if m.Title != nil {
		in.Title = *m.Title
	}
	if m.Artist != nil {
		in.Artist = *m.Artist
	}
	if m.ConcertKey != nil {
		in.ConcertKey = m.ConcertKey
	}
	if m.CapoFret != nil {
		in.CapoFret = *m.CapoFret
	}
	if m.TempoBPM != nil {
		in.TempoBPM = m.TempoBPM
	}
	if m.TimeSignature != nil {
		in.TimeSignature = m.TimeSignature
	}
	return in
}

// ExportChordPro writes a chart's draft as ChordPro text.
func (s *SongChartService) ExportChordPro(ctx context.Context, caller domain.User, id string) (string, error) {
	chart, err := s.Get(ctx, caller, id)
	if err != nil {
		return "", err
	}
	return domain.ExportChordPro(chart.Draft), nil
}
