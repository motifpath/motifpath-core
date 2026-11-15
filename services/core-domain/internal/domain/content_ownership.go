package domain

import "fmt"

// CatalogProfileID is the fixed MotifPath Catalog system profile. Every
// platform-owned item names it as owner, so platform content resolves to one
// rights holder while the admin who wrote it stays its creator.
const CatalogProfileID = "77d0239a-8d28-5c95-bc6e-53d59f7f84a8"

// ContentKind is a kind of authored content that has a rights holder.
// Learner copies are not a kind: they resolve to their template's ownership.
type ContentKind string

const (
	ContentCourse      ContentKind = "course"
	ContentPath        ContentKind = "path"
	ContentContentNode ContentKind = "content_node"
	ContentExercise    ContentKind = "exercise"
)

// ParseContentKind accepts only the four owned content kinds.
func ParseContentKind(name string) (ContentKind, error) {
	switch kind := ContentKind(name); kind {
	case ContentCourse, ContentPath, ContentContentNode, ContentExercise:
		return kind, nil
	default:
		return "", NewValidationError("content_kind", fmt.Sprintf("%q is not an owned content kind", name))
	}
}

// ContentRef names one item of authored content.
type ContentRef struct {
	Kind ContentKind
	ID   string
}

// OwnerKind is the category of an item's commercial rights holder.
type OwnerKind string

const (
	OwnerPlatform OwnerKind = "platform"
	OwnerTeacher  OwnerKind = "teacher"
	// OwnerUnresolved means no reviewed rights holder is known. It never
	// names an owner and cannot establish commercial ownership.
	OwnerUnresolved OwnerKind = "unresolved"
)

// OwnershipSource is where an ownership record's attribution came from.
type OwnershipSource string

const (
	SourceNative         OwnershipSource = "native"
	SourceImported       OwnershipSource = "imported"
	SourceReviewedLegacy OwnershipSource = "reviewed_legacy"
)

// OwnershipCopiedFrom is the item an author copied, with its ownership as it
// stood when the copy was made. It is attribution only and grants no right to
// the source.
type OwnershipCopiedFrom struct {
	Content           ContentRef
	OwnerKind         OwnerKind
	OwnerID           *string
	OwnershipRevision int
}

// ContentOwnership is one revision of an item's authoritative rights holder.
// Revision starts at 1 and only a correction adds one; edits, publishing and
// role changes never do. Earlier revisions stay recorded as history.
type ContentOwnership struct {
	Content         ContentRef
	OwnerKind       OwnerKind
	OwnerID         *string
	CreatorID       *string
	Source          OwnershipSource
	EvidenceEntryID string
	Revision        int
	CopiedFrom      *OwnershipCopiedFrom
}

// NativeOwnership is the ownership of content just created by author: the
// platform's when the author is an admin, otherwise the author's own.
func NativeOwnership(content ContentRef, author User, evidenceEntryID string) ContentOwnership {
	creator := author.ID
	ownership := ContentOwnership{
		Content: content, CreatorID: &creator, Source: SourceNative,
		EvidenceEntryID: evidenceEntryID, Revision: 1,
	}
	if author.Role == RoleAdmin {
		return ownership.ownedBy(OwnerPlatform, CatalogProfileID)
	}
	return ownership.ownedBy(OwnerTeacher, author.ID)
}

// LegacyAuthor is the recorded author of content that existed before
// ownership was recorded, with whether they are an admin when the backfill
// runs.
type LegacyAuthor struct {
	ID      string
	IsAdmin bool
}

// LegacyOwnership attributes existing content to its recorded author. An
// admin or the catalog profile gives platform ownership, any other author
// teacher ownership, and no recorded author leaves it unresolved.
func LegacyOwnership(content ContentRef, author *LegacyAuthor, evidenceEntryID string) ContentOwnership {
	ownership := ContentOwnership{
		Content: content, OwnerKind: OwnerUnresolved, Source: SourceReviewedLegacy,
		EvidenceEntryID: evidenceEntryID, Revision: 1,
	}
	if author == nil {
		return ownership
	}
	creator := author.ID
	ownership.CreatorID = &creator
	if author.IsAdmin || author.ID == CatalogProfileID {
		return ownership.ownedBy(OwnerPlatform, CatalogProfileID)
	}
	return ownership.ownedBy(OwnerTeacher, author.ID)
}

func (o ContentOwnership) ownedBy(kind OwnerKind, ownerID string) ContentOwnership {
	o.OwnerKind = kind
	o.OwnerID = &ownerID
	return o
}

// OwnershipCorrection is an administrator's reviewed change of rights holder,
// stated against the ownership revision they observed.
type OwnershipCorrection struct {
	ExpectedRevision int
	OwnerKind        OwnerKind
	OwnerID          string
}

// Validate checks the target rights holder: the platform is always the
// catalog profile, and a teacher never is.
func (c OwnershipCorrection) Validate() error {
	switch {
	case c.ExpectedRevision < 1:
		return NewValidationError("expected_revision", "must be at least 1")
	case c.OwnerKind == OwnerPlatform && c.OwnerID != CatalogProfileID:
		return NewValidationError("owner_id", "platform ownership names the MotifPath Catalog profile")
	case c.OwnerKind == OwnerTeacher && c.OwnerID == CatalogProfileID:
		return NewValidationError("owner_id", "the MotifPath Catalog profile is not a teacher")
	case c.OwnerKind != OwnerPlatform && c.OwnerKind != OwnerTeacher:
		return NewValidationError("owner_kind", "a correction names the platform or a teacher")
	}
	return nil
}

// Corrected returns the next ownership revision with the corrected rights
// holder. The creator, source and copy attribution are kept; a correction
// stated against an older revision is a conflict.
func (o ContentOwnership) Corrected(correction OwnershipCorrection, evidenceEntryID string) (ContentOwnership, error) {
	if err := correction.Validate(); err != nil {
		return ContentOwnership{}, err
	}
	if correction.ExpectedRevision != o.Revision {
		return ContentOwnership{}, fmt.Errorf("%w: ownership is at revision %d, not %d", ErrConflict, o.Revision, correction.ExpectedRevision)
	}
	next := o.ownedBy(correction.OwnerKind, correction.OwnerID)
	next.Revision = o.Revision + 1
	next.EvidenceEntryID = evidenceEntryID
	return next, nil
}
