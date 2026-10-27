package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRunFlags(t *testing.T) {
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.FixedZone("-03", -3*3600))
	tests := []struct {
		name    string
		args    []string
		want    runFlags
		wantErr string
	}{
		{name: "no flags: run on the clock, forever", args: nil, want: runFlags{}},
		{name: "one pass now", args: []string{"--once"}, want: runFlags{once: true}},
		{name: "one pass as if at a moment", args: []string{"--once", "--at", "2026-11-02T09:00:00-03:00"}, want: runFlags{once: true, at: at}},
		{name: "one pass for one address", args: []string{"--once", "--to", "alice@example.com"}, want: runFlags{once: true, to: "alice@example.com"}},
		{name: "a moment needs a single pass", args: []string{"--at", "2026-11-02T09:00:00-03:00"}, wantErr: "--at needs --once"},
		{name: "a moment must name its offset", args: []string{"--once", "--at", "2026-11-02T09:00"}, wantErr: "--at must be"},
		{name: "an unknown flag", args: []string{"--now"}, wantErr: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRunFlags(tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.once, got.once)
			assert.Equal(t, tt.want.to, got.to)
			assert.True(t, tt.want.at.Equal(got.at), "at: want %v, got %v", tt.want.at, got.at)
		})
	}
}
