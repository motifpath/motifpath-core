package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// SongChartStatus is where a chart stands with learners: never published,
// served at its latest published revision, or taken away from them.
type SongChartStatus string

const (
	SongChartDraftStatus SongChartStatus = "draft"
	SongChartPublished   SongChartStatus = "published"
	SongChartWithdrawn   SongChartStatus = "withdrawn"
)

// StandardGuitarTuningFingerprint is the tuning every song chart's chords
// are written for: standard six-string guitar, lowest string first.
const StandardGuitarTuningFingerprint = "E2-A2-D3-G3-B3-E4"

// The bounds of a song chart's fields.
const (
	MaxSongChartTitleLength   = 200
	MaxSongChartArtistLength  = 200
	MaxSongChartCapoFret      = 12
	MaxWithdrawalReasonLength = 2000
)

var concertKeyPattern = regexp.MustCompile(`^[A-G](b|#)?m?$`)

// SongChartDraft is a chart's working copy. Editing it never changes what
// learners read; publishing turns it into a new revision.
type SongChartDraft struct {
	Title         string
	Artist        string
	Language      string
	ConcertKey    *string
	CapoFret      int
	TempoBPM      *int
	TimeSignature *TimeSignature
	// RightsConfirmation is an admin's statement that the song's rights
	// were checked, outside MotifPath; nil when nobody has confirmed them.
	RightsConfirmation *RightsConfirmation
	Body               SongChartDocument
	// Warnings are the chord anchors that didn't fully resolve when the
	// draft was last saved, in document order.
	Warnings  []AnchorWarning
	UpdatedBy string
	UpdatedAt time.Time
}

// RightsConfirmation records who confirmed a song's rights were checked,
// and when. It says nothing about the rights themselves.
type RightsConfirmation struct {
	ConfirmedBy string
	ConfirmedAt time.Time
}

// Validate checks the draft's own fields. The body was checked when it was
// parsed, and whether its language exists is the caller's to check.
func (d SongChartDraft) Validate() error {
	if err := validateRequiredText("title", d.Title, MaxSongChartTitleLength); err != nil {
		return err
	}
	if err := validateRequiredText("artist", d.Artist, MaxSongChartArtistLength); err != nil {
		return err
	}
	if d.Language == "" || d.Language == LanguageCodeAny {
		return NewValidationError("language", `must be a language code other than "any"`)
	}
	if d.ConcertKey != nil && !concertKeyPattern.MatchString(*d.ConcertKey) {
		return NewValidationError("concert_key", "must be a note, A to G with an optional b or #, then an optional m")
	}
	if d.CapoFret < 0 || d.CapoFret > MaxSongChartCapoFret {
		return NewValidationError("capo_fret", "must be from 0 to 12")
	}
	if d.TempoBPM != nil && (*d.TempoBPM < MinTempoBPM || *d.TempoBPM > MaxTempoBPM) {
		return NewValidationError("tempo_bpm", "must be from 20 to 300")
	}
	if d.TimeSignature != nil && !d.TimeSignature.valid() {
		return NewValidationError("time_signature", "must have 1 to 16 beats of a 1, 2, 4, 8, 16 or 32 note")
	}
	return nil
}

func validateRequiredText(field, value string, maxLength int) error {
	if strings.TrimSpace(value) == "" {
		return NewValidationError(field, "must not be blank")
	}
	if utf8.RuneCountInString(value) > maxLength {
		return NewValidationError(field, fmt.Sprintf("must be at most %d characters", maxLength))
	}
	return nil
}

// SetRightsConfirmed confirms or clears the rights confirmation. Confirming
// an already confirmed draft keeps its first confirmation, so who confirmed
// it doesn't change with every save.
func (d *SongChartDraft) SetRightsConfirmed(confirmed bool, by string, at time.Time) {
	switch {
	case !confirmed:
		d.RightsConfirmation = nil
	case d.RightsConfirmation == nil:
		d.RightsConfirmation = &RightsConfirmation{ConfirmedBy: by, ConfirmedAt: at}
	}
}

// AnchorWarningKind is why a chord anchor didn't fully resolve to a
// playable catalog chord.
type AnchorWarningKind string

const (
	AnchorUnparsedSymbol     AnchorWarningKind = "unparsed_symbol"
	AnchorUnsupportedQuality AnchorWarningKind = "unsupported_quality"
	AnchorChordNotInCatalog  AnchorWarningKind = "chord_not_in_catalog"
	AnchorVoicingUnavailable AnchorWarningKind = "voicing_unavailable"
	AnchorBassNotInCatalog   AnchorWarningKind = "bass_not_in_catalog"
)

// AnchorWarning is one chord anchor that didn't fully resolve.
type AnchorWarning struct {
	AnchorID      string
	Position      AnchorPosition
	WrittenSymbol string
	Kind          AnchorWarningKind
}

// BlocksPublication says whether the warning stops a draft from being
// published. Only a slash chord whose bass the catalog lacks doesn't: the
// learner is shown the chord without its bass.
func (w AnchorWarning) BlocksPublication() bool {
	return w.Kind != AnchorBassNotInCatalog
}

// SongChartRevisionSummary is which revision learners are served.
type SongChartRevisionSummary struct {
	Number      int
	Title       string
	Language    string
	PublishedBy string
	PublishedAt time.Time
}

// SongChartWithdrawal is who took a chart away from learners, when and why.
type SongChartWithdrawal struct {
	WithdrawnBy string
	WithdrawnAt time.Time
	Reason      string
}

// SongChart is lyrics with chords anchored to their words, authored and
// published by admins. PublishedRevision is nil until the first publication.
type SongChart struct {
	ID                string
	Status            SongChartStatus
	Draft             SongChartDraft
	PublishedRevision *SongChartRevisionSummary
	Withdrawal        *SongChartWithdrawal
	CreatedBy         string
	CreatedAt         time.Time
}

// SongChartFilter narrows a list of song charts. Q matches the draft's title
// or artist, ignoring case and accents; Status, when set, is the chart's.
type SongChartFilter struct {
	Q      string
	Status *SongChartStatus
}

// SongChartRevision is a published revision. It never changes: a correction
// is published as a new revision.
type SongChartRevision struct {
	SongChartID        string
	Number             int
	Title              string
	Artist             string
	Language           string
	ConcertKey         *string
	CapoFret           int
	TempoBPM           *int
	TimeSignature      *TimeSignature
	TuningFingerprint  string
	Body               SongChartDocument
	RightsConfirmation RightsConfirmation
	PublishedBy        string
	PublishedAt        time.Time
}

// NotPublishableReason is one thing that stops a draft from being published.
type NotPublishableReason string

const (
	RightsNotConfirmed NotPublishableReason = "rights_not_confirmed"
	UnresolvedChords   NotPublishableReason = "unresolved_chords"
)

// SongChartNotPublishableError lists everything that stops a draft from
// being published. It is an ErrConflict: the request is well formed, but the
// draft isn't ready.
type SongChartNotPublishableError struct {
	Reasons        []NotPublishableReason
	AnchorWarnings []AnchorWarning
}

func (e *SongChartNotPublishableError) Error() string {
	reasons := make([]string, len(e.Reasons))
	for i, r := range e.Reasons {
		reasons[i] = string(r)
	}
	return "song chart is not publishable: " + strings.Join(reasons, ", ")
}

func (e *SongChartNotPublishableError) Unwrap() error {
	return ErrConflict
}

// Publish turns the draft into the next revision and serves it, also when
// the chart was withdrawn. The draft's warnings must be current: a draft
// whose rights aren't confirmed, or with a warning that blocks publication,
// is refused with every reason at once.
func (c SongChart) Publish(by string, at time.Time) (SongChart, SongChartRevision, error) {
	var refusal SongChartNotPublishableError
	if c.Draft.RightsConfirmation == nil {
		refusal.Reasons = append(refusal.Reasons, RightsNotConfirmed)
	}
	for _, w := range c.Draft.Warnings {
		if w.BlocksPublication() {
			refusal.AnchorWarnings = append(refusal.AnchorWarnings, w)
		}
	}
	if len(refusal.AnchorWarnings) > 0 {
		refusal.Reasons = append(refusal.Reasons, UnresolvedChords)
	}
	if len(refusal.Reasons) > 0 {
		return c, SongChartRevision{}, &refusal
	}

	number := 1
	if c.PublishedRevision != nil {
		number = c.PublishedRevision.Number + 1
	}
	d := c.Draft
	rev := SongChartRevision{
		SongChartID: c.ID, Number: number,
		Title: d.Title, Artist: d.Artist, Language: d.Language, ConcertKey: d.ConcertKey,
		CapoFret: d.CapoFret, TempoBPM: d.TempoBPM, TimeSignature: d.TimeSignature,
		TuningFingerprint: StandardGuitarTuningFingerprint, Body: d.Body,
		RightsConfirmation: *d.RightsConfirmation, PublishedBy: by, PublishedAt: at,
	}
	c.Status = SongChartPublished
	c.Withdrawal = nil
	c.PublishedRevision = &SongChartRevisionSummary{Number: number, Title: d.Title, Language: d.Language, PublishedBy: by, PublishedAt: at}
	return c, rev, nil
}

// Withdraw takes a published chart away from learners. Its revisions stay,
// for audit.
func (c SongChart) Withdraw(by string, at time.Time, reason string) (SongChart, error) {
	if c.Status != SongChartPublished {
		return c, fmt.Errorf("%w: the chart is not published", ErrConflict)
	}
	if err := validateRequiredText("reason", reason, MaxWithdrawalReasonLength); err != nil {
		return c, err
	}
	c.Status = SongChartWithdrawn
	c.Withdrawal = &SongChartWithdrawal{WithdrawnBy: by, WithdrawnAt: at, Reason: reason}
	return c, nil
}
