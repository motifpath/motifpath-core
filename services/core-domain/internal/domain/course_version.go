package domain

import "time"

// CourseVersionCheckpoint is one checkpoint's identity as it existed at the
// moment its CourseVersion was published: position, the LearningPath
// template it pointed at, and the title actually shown then. A checkpoint's
// items are never snapshotted here — resolving a published course's
// outline reads each checkpoint's current items live from the referenced
// LearningPath, the same "resolve live from current state" pattern
// StudentPathItem already uses for its title/content_type.
type CourseVersionCheckpoint struct {
	Position       int
	LearningPathID string
	EffectiveTitle string
}

// CourseVersion is an immutable, permanent snapshot of a course's title,
// summary, level, and checkpoint identities at the moment it was published.
// Students and the catalog only ever read the latest CourseVersion, never
// the live Course/CourseCheckpoint draft.
type CourseVersion struct {
	ID              string
	CourseID        string
	VersionNumber   int
	TitleSnapshot   string
	SummarySnapshot string
	LevelSnapshot   DifficultyLevel
	Checkpoints     []CourseVersionCheckpoint
	PublishedAt     time.Time
	// AvailableForNewEnrollments is true by default on a freshly published
	// version. Nothing in this slice of the feature sets it false — a
	// later course-retirement or superseding-version capability is its
	// actual consumer.
	AvailableForNewEnrollments bool
}

// NewCourseVersionSnapshot builds the next version of course. versionNumber
// must be one greater than the highest version already published for this
// course — the application layer resolves that by asking the repository
// for the course's current latest version before calling this. The
// checkpoints slice is copied field-by-field so a later edit to course's
// draft checkpoints can never retroactively alter an already-built
// snapshot.
func NewCourseVersionSnapshot(id string, course Course, versionNumber int, publishedAt time.Time) CourseVersion {
	checkpoints := make([]CourseVersionCheckpoint, len(course.Checkpoints))
	for i, cp := range course.Checkpoints {
		checkpoints[i] = CourseVersionCheckpoint{
			Position:       cp.Position,
			LearningPathID: cp.LearningPathID,
			EffectiveTitle: cp.EffectiveTitle,
		}
	}

	return CourseVersion{
		ID:                         id,
		CourseID:                   course.ID,
		VersionNumber:              versionNumber,
		TitleSnapshot:              course.Title,
		SummarySnapshot:            course.Summary,
		LevelSnapshot:              course.Level,
		Checkpoints:                checkpoints,
		PublishedAt:                publishedAt,
		AvailableForNewEnrollments: true,
	}
}

// HasUnpublishedChanges reports whether course's live draft differs from
// latest — true when latest is nil (nothing has ever been published for
// this course) or when title, summary, level, or checkpoint identity
// (each checkpoint's learning_path_id, effective_title, and position, in
// order) differs from latest's snapshot. False otherwise.
func HasUnpublishedChanges(course Course, latest *CourseVersion) bool {
	if latest == nil {
		return true
	}

	if course.Title != latest.TitleSnapshot ||
		course.Summary != latest.SummarySnapshot ||
		course.Level != latest.LevelSnapshot {
		return true
	}

	if len(course.Checkpoints) != len(latest.Checkpoints) {
		return true
	}

	for i, cp := range course.Checkpoints {
		snapshot := latest.Checkpoints[i]
		if cp.Position != snapshot.Position ||
			cp.LearningPathID != snapshot.LearningPathID ||
			cp.EffectiveTitle != snapshot.EffectiveTitle {
			return true
		}
	}

	return false
}
