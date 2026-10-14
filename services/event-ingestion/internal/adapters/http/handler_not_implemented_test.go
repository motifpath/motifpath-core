package http

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
)

// An operation whose contract is published ahead of its implementation fails
// as not implemented, and never answers as if it had worked.
func TestListLoggedEvents_notImplementedYet(t *testing.T) {
	resp, err := (&Handler{}).ListLoggedEvents(context.Background(), generated.ListLoggedEventsRequestObject{})
	require.ErrorIs(t, err, ErrNotImplemented)
	assert.Nil(t, resp)
}
