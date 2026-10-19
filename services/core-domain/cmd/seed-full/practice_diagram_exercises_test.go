package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The catalog generator derives each basic diagram's id from its key; a seed
// naming one must derive the same id, or it points at nothing.
func TestCatalogDiagramID(t *testing.T) {
	assert.Equal(t, "431e9182-6aeb-5665-83ba-a5bfa4a03dbf", catalogDiagramID(fullNeckMinorPentatonicKey))
}
