package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const maxGroupNameLength = 120

// ErrInvalidGroupChange is returned for a group slug or name that cannot be saved.
var ErrInvalidGroupChange = errors.New("invalid commercial group change")

// ErrInvalidMembershipChange is returned for a membership change that is
// empty, repeats a user, or both adds and removes the same user.
var ErrInvalidMembershipChange = errors.New("invalid commercial group membership change")

// CommercialGroup is a named set of users that a flag can allow as a single
// allowlist entry. Groups hold users only; they do not nest.
type CommercialGroup struct {
	Slug      string
	Name      string
	MemberIDs []string
	UpdatedAt time.Time
	UpdatedBy string
}

// ValidateGroupChange checks a group's slug and display name.
func ValidateGroupChange(slug, name string) error {
	if !rolloutSlugPattern.MatchString(slug) {
		return fmt.Errorf("%w: slug %q", ErrInvalidGroupChange, slug)
	}
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > maxGroupNameLength {
		return fmt.Errorf("%w: name must be 1 to %d characters", ErrInvalidGroupChange, maxGroupNameLength)
	}
	return nil
}

// ApplyMembershipChange returns the sorted membership after adding and
// removing users. Adding a current member or removing a non-member is not an
// error: the result is simply the requested membership.
func ApplyMembershipChange(current, add, remove []string) ([]string, error) {
	if len(add) == 0 && len(remove) == 0 {
		return nil, fmt.Errorf("%w: nothing to add or remove", ErrInvalidMembershipChange)
	}
	if !distinctUUIDs(add) || !distinctUUIDs(remove) {
		return nil, fmt.Errorf("%w: users must be distinct UUIDs", ErrInvalidMembershipChange)
	}
	removing := make(map[string]bool, len(remove))
	for _, userID := range remove {
		removing[userID] = true
	}
	members := make(map[string]bool, len(current)+len(add))
	for _, userID := range current {
		members[userID] = true
	}
	for _, userID := range add {
		if removing[userID] {
			return nil, fmt.Errorf("%w: user %s is both added and removed", ErrInvalidMembershipChange, userID)
		}
		members[userID] = true
	}
	for userID := range removing {
		delete(members, userID)
	}

	result := make([]string, 0, len(members))
	for userID := range members {
		result = append(result, userID)
	}
	sort.Strings(result)
	return result, nil
}
