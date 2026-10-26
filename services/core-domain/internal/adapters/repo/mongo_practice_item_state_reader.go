package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/core-domain/internal/domain"
)

// practiceItemStateDocument holds the fields this service reads from a
// `practice_item_state` document, in the shape the Aggregation Worker's
// evidence processor writes it.
type practiceItemStateDocument struct {
	ItemKey      string     `bson:"item_key"`
	RulesVersion int        `bson:"rules_version"`
	Level        string     `bson:"level"`
	Counted      int        `bson:"counted"`
	Box          int        `bson:"box"`
	DueAt        *time.Time `bson:"due_at"`
	LastAt       *time.Time `bson:"last_at"`
	Accuracy     float64    `bson:"accuracy"`
	Fluency      float64    `bson:"fluency"`
	BestCleanBPM *int       `bson:"best_clean_bpm"`
	// BestCleanNoteValue is nil in a state folded before note values: quarters.
	BestCleanNoteValue *string `bson:"best_clean_note_value"`
	// RightByResponse is absent until the item has a right answer.
	RightByResponse map[string]int `bson:"right_by_response"`
	// A chord or root change's best clean count and last minute; absent for
	// any other item.
	BestChangesPerMinute *int   `bson:"best_changes_per_minute"`
	LastChangeTempoBPM   *int   `bson:"last_change_tempo_bpm"`
	LastChangesPerMinute *int   `bson:"last_changes_per_minute"`
	LastRating           string `bson:"last_rating"`
	CleanMinutesInARow   int    `bson:"clean_minutes_in_a_row"`
	// A progression key tour's tempo at effective speed, its keys clean at
	// that tempo and the note value of its last clean take; absent for any
	// other item.
	TourTempoBPM  *int     `bson:"tour_tempo_bpm"`
	TourCleanKeys []string `bson:"tour_clean_keys"`
	TourNoteValue *string  `bson:"tour_note_value"`
	// A guide tone's last key, which its next take is never planned in;
	// absent for any other item.
	LastMusicKey string `bson:"last_music_key"`
}

// keyTour is the document's progression key tour; nil without one.
func (d practiceItemStateDocument) keyTour() *domain.KeyTourState {
	if d.TourTempoBPM == nil {
		return nil
	}
	return &domain.KeyTourState{TempoBPM: *d.TourTempoBPM, CleanKeys: d.TourCleanKeys, NoteValue: tempoNoteValue(d.TourNoteValue)}
}

// lastChangeMinute is the document's last change minute; nil without one.
func (d practiceItemStateDocument) lastChangeMinute() *domain.ChangeMinuteState {
	if d.LastChangeTempoBPM == nil || d.LastChangesPerMinute == nil {
		return nil
	}
	return &domain.ChangeMinuteState{
		TempoBPM: *d.LastChangeTempoBPM, ChangesPerMinute: *d.LastChangesPerMinute,
		Struggled: d.LastRating == "struggled", CleanInARow: d.CleanMinutesInARow,
	}
}

// MongoPracticeItemStateReader reads the `practice_item_state` collection
// the Aggregation Worker owns. It never writes to it.
type MongoPracticeItemStateReader struct {
	collection *mongo.Collection
}

func NewMongoPracticeItemStateReader(db *mongo.Database) *MongoPracticeItemStateReader {
	return &MongoPracticeItemStateReader{collection: db.Collection("practice_item_state")}
}

func (r *MongoPracticeItemStateReader) GetStates(ctx context.Context, studentID string, itemKeys []string) (map[string]domain.PracticeItemState, error) {
	result := map[string]domain.PracticeItemState{}
	if len(itemKeys) == 0 {
		return result, nil
	}

	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "item_key", Value: bson.D{{Key: "$in", Value: itemKeys}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc practiceItemStateDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ItemKey] = domain.PracticeItemState{
			ItemKey:              doc.ItemKey,
			RulesVersion:         doc.RulesVersion,
			Level:                domain.KnowledgeLevel(doc.Level),
			Counted:              doc.Counted,
			Box:                  doc.Box,
			DueAt:                utcTime(doc.DueAt),
			LastAt:               utcTime(doc.LastAt),
			Accuracy:             doc.Accuracy,
			Fluency:              doc.Fluency,
			BestCleanBPM:         doc.BestCleanBPM,
			BestCleanNoteValue:   tempoNoteValue(doc.BestCleanNoteValue),
			RightByResponse:      doc.RightByResponse,
			BestChangesPerMinute: doc.BestChangesPerMinute,
			LastChangeMinute:     doc.lastChangeMinute(),
			KeyTour:              doc.keyTour(),
			LastMusicKey:         doc.LastMusicKey,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func utcTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// tempoNoteValue is a stored note value in the domain, or nil when none was stored.
func tempoNoteValue(s *string) *domain.TempoNoteValue {
	if s == nil {
		return nil
	}
	v := domain.TempoNoteValue(*s)
	return &v
}
