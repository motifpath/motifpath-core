package repo

import (
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent/course"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/domain"
)

// courseListPredicates translates filter into ent predicates on the courses
// table. The text, level and classification predicates read either the live
// draft or — when filter.PublishedView is set — the course's latest
// published version, so a student's filtering matches exactly what the
// catalog entry shows them.
func courseListPredicates(filter domain.CourseListFilter) ([]predicate.Course, error) {
	var predicates []predicate.Course

	if filter.Status != nil {
		predicates = append(predicates, course.StatusEQ(course.Status(*filter.Status)))
	}
	if filter.CreatedBy != "" {
		createdBy, err := uuid.Parse(filter.CreatedBy)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, course.CreatedByEQ(createdBy))
	}

	predicates = append(predicates, textAndLevelPredicates(filter)...)

	if filter.InstrumentID != "" {
		instrumentID, err := uuid.Parse(filter.InstrumentID)
		if err != nil {
			return nil, err
		}
		if filter.PublishedView {
			predicates = append(predicates, publishedVersionMatches(publishedInstrumentCondition(instrumentID)))
		} else {
			predicates = append(predicates, course.Or(
				course.HasInstrumentsWith(instrument.ID(instrumentID)),
				course.Not(course.HasInstruments()),
			))
		}
	}

	if len(filter.SkillIDs) > 0 || len(filter.ConceptIDs) > 0 {
		classified, err := classificationPredicate(filter)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, classified)
	}
	return predicates, nil
}

// textAndLevelPredicates matches filter.Query against title or summary, and
// filter.Levels and filter.Language against the level and language, reading the published version's snapshot
// when filter.PublishedView is set and the live draft otherwise.
func textAndLevelPredicates(filter domain.CourseListFilter) []predicate.Course {
	var predicates []predicate.Course

	if filter.PublishedView {
		if filter.Query != "" {
			predicates = append(predicates, publishedVersionMatches(publishedTextCondition(filter.Query)))
		}
		if len(filter.Levels) > 0 {
			predicates = append(predicates, publishedVersionMatches(publishedLevelCondition(filter.Levels)))
		}
		if filter.Language != "" {
			predicates = append(predicates, publishedVersionMatches(publishedLanguageCondition(filter.Language)))
		}
		return predicates
	}

	if filter.Query != "" {
		predicates = append(predicates, course.Or(
			course.TitleContainsFold(filter.Query),
			course.SummaryContainsFold(filter.Query),
		))
	}
	if len(filter.Levels) > 0 {
		levels := make([]course.Level, len(filter.Levels))
		for i, l := range filter.Levels {
			levels[i] = course.Level(l)
		}
		predicates = append(predicates, course.LevelIn(levels...))
	}
	if filter.Language != "" {
		predicates = append(predicates, course.LanguageEQ(filter.Language))
	}
	return predicates
}

func classificationPredicate(filter domain.CourseListFilter) (predicate.Course, error) {
	skillIDs, err := parseUUIDs(filter.SkillIDs)
	if err != nil {
		return nil, err
	}
	conceptIDs, err := parseUUIDs(filter.ConceptIDs)
	if err != nil {
		return nil, err
	}
	return classifiedCheckpointMatches(filter.PublishedView, skillIDs, conceptIDs), nil
}

// courseListOrder orders by title then id — the published version's title
// for a published view, since that is the title the entry displays.
func courseListOrder(filter domain.CourseListFilter) []course.OrderOption {
	if !filter.PublishedView {
		return []course.OrderOption{course.ByTitle(), course.ByID()}
	}
	byPublishedTitle := course.OrderOption(func(s *sql.Selector) {
		s.OrderExprFunc(func(b *sql.Builder) {
			b.WriteString("(SELECT lv.title_snapshot FROM course_versions lv WHERE lv.course_id = " + s.C(course.FieldID) +
				" ORDER BY lv.version_number DESC LIMIT 1)")
		})
	})
	return []course.OrderOption{byPublishedTitle, course.ByID()}
}

// publishedVersionMatches matches a course whose latest published version
// (aliased cv) satisfies condition, which writes a leading " AND ..." clause.
func publishedVersionMatches(condition func(b *sql.Builder)) predicate.Course {
	return predicate.Course(func(s *sql.Selector) {
		s.Where(sql.P(func(b *sql.Builder) {
			b.WriteString("EXISTS (SELECT 1 FROM course_versions cv WHERE cv.course_id = " + s.C(course.FieldID) +
				" AND cv.version_number = (SELECT MAX(lv.version_number) FROM course_versions lv WHERE lv.course_id = cv.course_id)")
			condition(b)
			b.WriteString(")")
		}))
	})
}

func publishedTextCondition(query string) func(b *sql.Builder) {
	pattern := "%" + escapeLike(query) + "%"
	return func(b *sql.Builder) {
		b.WriteString(" AND (cv.title_snapshot ILIKE ").Arg(pattern).
			WriteString(" OR cv.summary_snapshot ILIKE ").Arg(pattern).WriteString(")")
	}
}

// publishedInstrumentCondition matches a version for instrumentID or for
// every instrument: a snapshot that is SQL NULL (published before instruments
// were recorded), JSON null (an empty list written as nil) or an empty array.
func publishedInstrumentCondition(instrumentID uuid.UUID) func(b *sql.Builder) {
	return func(b *sql.Builder) {
		b.WriteString(" AND (cv.instrument_ids_snapshot IS NULL OR jsonb_typeof(cv.instrument_ids_snapshot) <> 'array'" +
			" OR jsonb_array_length(cv.instrument_ids_snapshot) = 0" +
			" OR cv.instrument_ids_snapshot @> jsonb_build_array(").Arg(instrumentID.String()).WriteString("::text))")
	}
}

func publishedLanguageCondition(language string) func(b *sql.Builder) {
	return func(b *sql.Builder) {
		b.WriteString(" AND cv.language_snapshot = ").Arg(language)
	}
}

func publishedLevelCondition(levels []domain.DifficultyLevel) func(b *sql.Builder) {
	return func(b *sql.Builder) {
		b.WriteString(" AND cv.level_snapshot IN (")
		for i, l := range levels {
			if i > 0 {
				b.WriteString(", ")
			}
			b.Arg(string(l))
		}
		b.WriteString(")")
	}
}

// classifiedCheckpointMatches matches a course when some checkpoint's
// learning path template holds a content node linked to any of skillIDs and
// — where both are given — also to any of conceptIDs. The checkpoints read
// are the live draft's, or the latest published version's when
// publishedView is set.
func classifiedCheckpointMatches(publishedView bool, skillIDs, conceptIDs []uuid.UUID) predicate.Course {
	return predicate.Course(func(s *sql.Selector) {
		s.Where(sql.P(func(b *sql.Builder) {
			if publishedView {
				b.WriteString("EXISTS (SELECT 1 FROM course_version_checkpoints cp " +
					"JOIN learning_path_items lpi ON lpi.learning_path_id = cp.learning_path_id " +
					"WHERE cp.course_version_id = (SELECT lv.id FROM course_versions lv WHERE lv.course_id = " + s.C(course.FieldID) +
					" ORDER BY lv.version_number DESC LIMIT 1)")
			} else {
				b.WriteString("EXISTS (SELECT 1 FROM course_checkpoints cp " +
					"JOIN learning_path_items lpi ON lpi.learning_path_id = cp.learning_path_id " +
					"WHERE cp.course_id = " + s.C(course.FieldID))
			}
			writeNodeLinkedToAny(b, "content_node_skills", "skill_id", skillIDs)
			writeNodeLinkedToAny(b, "content_node_concepts", "concept_id", conceptIDs)
			b.WriteString(")")
		}))
	})
}

// writeNodeLinkedToAny appends " AND EXISTS (...)" requiring the item's
// content node (lpi) to be linked through linkTable to any of ids. It writes
// nothing when ids is empty.
func writeNodeLinkedToAny(b *sql.Builder, linkTable, idColumn string, ids []uuid.UUID) {
	if len(ids) == 0 {
		return
	}
	b.WriteString(" AND EXISTS (SELECT 1 FROM " + linkTable + " link WHERE link.content_node_id = lpi.content_node_id AND link." + idColumn + " IN (")
	for i, id := range ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.Arg(id)
	}
	b.WriteString("))")
}

// escapeLike escapes the LIKE wildcards in s so a user's search text is
// matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
