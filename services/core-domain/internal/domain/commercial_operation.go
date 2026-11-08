package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

// AuditActorKind is the trusted category of whoever initiated a commercial
// operation. It is never taken from unverified caller metadata.
type AuditActorKind string

const (
	AuditActorUser     AuditActorKind = "user"
	AuditActorService  AuditActorKind = "service"
	AuditActorProvider AuditActorKind = "provider"
)

func (k AuditActorKind) valid() bool {
	switch k {
	case AuditActorUser, AuditActorService, AuditActorProvider:
		return true
	default:
		return false
	}
}

// ErrInvalidOperationKey is returned for a key missing any part of its scope.
var ErrInvalidOperationKey = errors.New("invalid commercial operation key")

// lowerUUIDPattern matches the canonical lowercase UUID text form.
var lowerUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// OperationKey is the durable identity of one logical commercial operation.
// A caller's key is only unique within its scope: the same UUID sent by
// another actor, for another action or resource, or in another environment is
// a different operation.
type OperationKey struct {
	Environment  Environment
	ActorKind    AuditActorKind
	ActorID      string
	Action       string
	ResourceKind string
	ResourceID   string
	CallerKey    string
}

// Validate checks that every part of the scope is present.
func (k OperationKey) Validate() error {
	switch {
	case k.Environment.String() == "":
		return fmt.Errorf("%w: no environment", ErrInvalidOperationKey)
	case !k.ActorKind.valid():
		return fmt.Errorf("%w: actor kind %q", ErrInvalidOperationKey, k.ActorKind)
	case k.ActorID == "":
		return fmt.Errorf("%w: no actor", ErrInvalidOperationKey)
	case k.Action == "":
		return fmt.Errorf("%w: no action", ErrInvalidOperationKey)
	case k.ResourceKind == "":
		return fmt.Errorf("%w: no resource kind", ErrInvalidOperationKey)
	case k.ResourceID == "":
		return fmt.Errorf("%w: no resource", ErrInvalidOperationKey)
	case !lowerUUIDPattern.MatchString(k.CallerKey):
		return fmt.Errorf("%w: caller key %q is not a lowercase UUID", ErrInvalidOperationKey, k.CallerKey)
	}
	return nil
}

// OperationID is the logical operation UUID reported in evidence and
// receipts. It is the caller's key; the rest of the scope keeps operations
// that reuse it apart.
func (k OperationKey) OperationID() string {
	return k.CallerKey
}

// ErrInvalidIntent is returned for an intent body that has no single
// canonical form.
var ErrInvalidIntent = errors.New("invalid commercial intent")

// FingerprintIntent returns the SHA-256 (lowercase hex) of an operation's
// canonical intent: its action, intent version, target and body. The body is
// canonicalized with object keys sorted recursively, array order kept, number
// literals and string content unchanged, and duplicate keys refused, so a
// missing field and an explicit null stay distinct.
func FingerprintIntent(action string, version int, target string, body json.RawMessage) (string, error) {
	canonicalBody, err := canonicalJSON(body)
	if err != nil {
		return "", err
	}
	var envelope bytes.Buffer
	envelope.WriteString(`{"action":`)
	writeJSONString(&envelope, action)
	envelope.WriteString(`,"body":`)
	envelope.Write(canonicalBody)
	envelope.WriteString(`,"target":`)
	writeJSONString(&envelope, target)
	envelope.WriteString(`,"version":`)
	envelope.WriteString(strconv.Itoa(version))
	envelope.WriteString(`}`)

	sum := sha256.Sum256(envelope.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out bytes.Buffer
	if err := writeCanonicalValue(decoder, &out); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing data after the body", ErrInvalidIntent)
	}
	return out.Bytes(), nil
}

func writeCanonicalValue(decoder *json.Decoder, out *bytes.Buffer) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidIntent, err)
	}
	switch value := token.(type) {
	case json.Delim:
		if value == '{' {
			return writeCanonicalObject(decoder, out)
		}
		return writeCanonicalArray(decoder, out)
	case string:
		writeJSONString(out, value)
	case json.Number:
		out.WriteString(value.String())
	case bool:
		out.WriteString(strconv.FormatBool(value))
	case nil:
		out.WriteString("null")
	default:
		return fmt.Errorf("%w: unexpected token %v", ErrInvalidIntent, value)
	}
	return nil
}

func writeCanonicalObject(decoder *json.Decoder, out *bytes.Buffer) error {
	members := map[string][]byte{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidIntent, err)
		}
		key, isString := keyToken.(string)
		if !isString {
			return fmt.Errorf("%w: object key is not a string", ErrInvalidIntent)
		}
		if _, seen := members[key]; seen {
			return fmt.Errorf("%w: duplicate key %q", ErrInvalidIntent, key)
		}
		var value bytes.Buffer
		if err := writeCanonicalValue(decoder, &value); err != nil {
			return err
		}
		members[key] = value.Bytes()
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidIntent, err)
	}

	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		writeJSONString(out, key)
		out.WriteByte(':')
		out.Write(members[key])
	}
	out.WriteByte('}')
	return nil
}

func writeCanonicalArray(decoder *json.Decoder, out *bytes.Buffer) error {
	out.WriteByte('[')
	for first := true; decoder.More(); first = false {
		if !first {
			out.WriteByte(',')
		}
		if err := writeCanonicalValue(decoder, out); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidIntent, err)
	}
	out.WriteByte(']')
	return nil
}

// writeJSONString writes s as a JSON string. Marshalling a Go string cannot
// fail, so the encoder's error is impossible here.
func writeJSONString(out *bytes.Buffer, s string) {
	encoded, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Sprintf("marshal a string: %v", err))
	}
	out.Write(encoded)
}
