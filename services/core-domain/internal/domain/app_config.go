package domain

import (
	"encoding/json"
	"time"
)

// AppConfigEntry is one of the app's settings as stored: an editorial or operational value the
// team changes at runtime. Value is the JSON of the key's own type — a key's value is only ever
// written after that type's validation accepted it.
type AppConfigEntry struct {
	Key       string
	Value     json.RawMessage
	UpdatedAt time.Time
	// UpdatedBy is the id of the user who last changed the setting.
	UpdatedBy string
}
