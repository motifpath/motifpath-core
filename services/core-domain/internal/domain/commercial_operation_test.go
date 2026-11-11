package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const callerKey = "5d0c1f7a-2b8e-4c1d-9a3f-6e7b8c9d0a1b"

func validOperationKey(t *testing.T) domain.OperationKey {
	t.Helper()
	env, err := domain.ParseEnvironment("staging")
	require.NoError(t, err)
	return domain.OperationKey{
		Environment:  env,
		ActorKind:    domain.AuditActorUser,
		ActorID:      beaID,
		Action:       "commercial_rollout.flag_changed",
		ResourceKind: "commercial_flag",
		ResourceID:   "content_ownership",
		CallerKey:    callerKey,
	}
}

func TestOperationKey_Validate(t *testing.T) {
	t.Run("a fully qualified key is valid", func(t *testing.T) {
		require.NoError(t, validOperationKey(t).Validate())
	})

	for name, mutate := range map[string]func(*domain.OperationKey){
		"no environment":          func(k *domain.OperationKey) { k.Environment = domain.Environment{} },
		"an unknown actor kind":   func(k *domain.OperationKey) { k.ActorKind = "robot" },
		"no actor":                func(k *domain.OperationKey) { k.ActorID = "" },
		"no action":               func(k *domain.OperationKey) { k.Action = "" },
		"no resource kind":        func(k *domain.OperationKey) { k.ResourceKind = "" },
		"no resource":             func(k *domain.OperationKey) { k.ResourceID = "" },
		"a caller key not a UUID": func(k *domain.OperationKey) { k.CallerKey = "retry-1" },
		"an upper-case caller key": func(k *domain.OperationKey) {
			k.CallerKey = "5D0C1F7A-2B8E-4C1D-9A3F-6E7B8C9D0A1B"
		},
	} {
		t.Run("invalid: "+name, func(t *testing.T) {
			key := validOperationKey(t)
			mutate(&key)

			require.ErrorIs(t, key.Validate(), domain.ErrInvalidOperationKey)
		})
	}

	t.Run("the same caller key under another actor is a different operation", func(t *testing.T) {
		mine := validOperationKey(t)
		theirs := validOperationKey(t)
		theirs.ActorID = cleoID

		assert.NotEqual(t, mine, theirs)
		assert.Equal(t, mine.OperationID(), theirs.OperationID())
	})
}

func TestFingerprintIntent(t *testing.T) {
	fingerprint := func(t *testing.T, body string) string {
		t.Helper()
		got, err := domain.FingerprintIntent("commercial_rollout.flag_changed", 1, "content_ownership", json.RawMessage(body))
		require.NoError(t, err)
		return got
	}

	t.Run("is a lowercase SHA-256 hex digest", func(t *testing.T) {
		assert.Regexp(t, `^[0-9a-f]{64}$`, fingerprint(t, `{"enabled":true}`))
	})

	for _, tc := range []struct {
		name string
		a, b string
	}{
		{"key order does not matter", `{"enabled":true,"all_users":false}`, `{"all_users":false,"enabled":true}`},
		{"nested key order does not matter", `{"x":{"b":1,"a":2}}`, `{"x":{"a":2,"b":1}}`},
		{"insignificant whitespace does not matter", `{"enabled": true}`, "{\n\"enabled\":true}"},
	} {
		t.Run("same intent: "+tc.name, func(t *testing.T) {
			assert.Equal(t, fingerprint(t, tc.a), fingerprint(t, tc.b))
		})
	}

	for _, tc := range []struct {
		name string
		a, b string
	}{
		{"a missing field differs from an explicit null", `{"reason":"x"}`, `{"reason":"x","note":null}`},
		{"array order matters", `{"subject_ids":["a","b"]}`, `{"subject_ids":["b","a"]}`},
		{"a different value differs", `{"enabled":true}`, `{"enabled":false}`},
		{"a number differs from its string", `{"expected_revision":1}`, `{"expected_revision":"1"}`},
		{"string content is kept exactly", `{"reason":"Beta"}`, `{"reason":"beta"}`},
		{"large integers are kept exactly", `{"n":9007199254740993}`, `{"n":9007199254740992}`},
	} {
		t.Run("different intent: "+tc.name, func(t *testing.T) {
			assert.NotEqual(t, fingerprint(t, tc.a), fingerprint(t, tc.b))
		})
	}

	t.Run("action, version and target are part of the intent", func(t *testing.T) {
		body := json.RawMessage(`{"enabled":true}`)
		base, err := domain.FingerprintIntent("a", 1, "t", body)
		require.NoError(t, err)

		for _, other := range []struct {
			action  string
			version int
			target  string
		}{{"b", 1, "t"}, {"a", 2, "t"}, {"a", 1, "u"}} {
			got, err := domain.FingerprintIntent(other.action, other.version, other.target, body)
			require.NoError(t, err)
			assert.NotEqual(t, base, got, "%+v", other)
		}
	})

	for name, body := range map[string]string{
		"a duplicate key": `{"enabled":true,"enabled":false}`,
		"malformed JSON":  `{"enabled":`,
		"trailing data":   `{"enabled":true} {}`,
	} {
		t.Run("refused: "+name, func(t *testing.T) {
			_, err := domain.FingerprintIntent("a", 1, "t", json.RawMessage(body))

			require.ErrorIs(t, err, domain.ErrInvalidIntent)
		})
	}
}
