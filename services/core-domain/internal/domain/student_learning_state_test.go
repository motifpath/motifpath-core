package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestStudentLearningState_Invariant(t *testing.T) {
	t.Run("zero value has no current course or path", func(t *testing.T) {
		var state domain.StudentLearningState
		assert.False(t, state.HasCurrent())
		assert.NoError(t, state.Validate())
	})

	t.Run("setting current_course_enrollment_id when current_standalone_path_id is already set is rejected", func(t *testing.T) {
		enrollmentID := "enrollment-1"
		pathID := "path-1"
		state := domain.StudentLearningState{
			CurrentStandalonePathID:   &pathID,
			CurrentCourseEnrollmentID: &enrollmentID,
		}
		err := state.Validate()
		assert.Error(t, err)
		var valErr *domain.ValidationError
		assert.ErrorAs(t, err, &valErr)
	})

	t.Run("WithCurrentStandalonePath clears any current course enrollment", func(t *testing.T) {
		enrollmentID := "enrollment-1"
		state := domain.StudentLearningState{CurrentCourseEnrollmentID: &enrollmentID}

		next := state.WithCurrentStandalonePath("path-1")

		assert.NoError(t, next.Validate())
		assert.Nil(t, next.CurrentCourseEnrollmentID)
		assert.Equal(t, "path-1", *next.CurrentStandalonePathID)
	})

	t.Run("WithCurrentCourseEnrollment clears any current standalone path", func(t *testing.T) {
		pathID := "path-1"
		state := domain.StudentLearningState{CurrentStandalonePathID: &pathID}

		next := state.WithCurrentCourseEnrollment("enrollment-1")

		assert.NoError(t, next.Validate())
		assert.Nil(t, next.CurrentStandalonePathID)
		assert.Equal(t, "enrollment-1", *next.CurrentCourseEnrollmentID)
	})

	t.Run("Cleared unsets both pointers", func(t *testing.T) {
		pathID := "path-1"
		state := domain.StudentLearningState{CurrentStandalonePathID: &pathID}

		next := state.Cleared()

		assert.False(t, next.HasCurrent())
	})
}
