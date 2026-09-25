package domain

import "time"

// ContentNodeVersion is an immutable, permanent snapshot of a content
// node's title, content type, media, and rich content at the moment it was
// published. A StudentPathItem pins to a specific ContentNodeVersion at
// copy time and is never retargeted by a later publish of the same node —
// editing or republishing a node afterward never alters what a student who
// already copied it sees.
type ContentNodeVersion struct {
	ID            string
	ContentNodeID string
	VersionNumber int
	Title         string
	ContentType   ContentType
	MediaURL      *string
	RichContent   *PromptDocument
	// Classification and Languages carry the node's classification and
	// language tags at the moment of publishing, for the publish response
	// and for a future read of this specific version. They are not
	// currently persisted by ContentNodeVersionRepository — only Title,
	// ContentType, MediaURL, and RichContent survive a later
	// GetLatestByContentNodeID call. A historical re-fetch of an older
	// version's classification/languages is a known gap.
	Classification Classification
	Languages      []Language
	// InstrumentIDsSnapshot is the node's InstrumentIDs when this version
	// was published; empty means every instrument.
	InstrumentIDsSnapshot []string
	// ThumbnailURLSnapshot is the node's ThumbnailURL when this version was
	// published; nil means none.
	ThumbnailURLSnapshot *string
	PublishedBy          string
	PublishedAt          time.Time
}

// NewContentNodeVersionSnapshot builds the next version of node.
// versionNumber must be one greater than the highest version already
// published for this node — the application layer resolves that by asking
// the repository for the node's current latest version before calling
// this.
func NewContentNodeVersionSnapshot(id string, node ContentNode, versionNumber int, publishedBy string, publishedAt time.Time) ContentNodeVersion {
	return ContentNodeVersion{
		ID:                    id,
		ContentNodeID:         node.ID,
		VersionNumber:         versionNumber,
		Title:                 node.Title,
		ContentType:           node.ContentType,
		MediaURL:              node.MediaURL,
		RichContent:           node.RichContent,
		Classification:        node.Classification,
		Languages:             node.Languages,
		InstrumentIDsSnapshot: append([]string(nil), node.InstrumentIDs...),
		ThumbnailURLSnapshot:  node.ThumbnailURL,
		PublishedBy:           publishedBy,
		PublishedAt:           publishedAt,
	}
}
