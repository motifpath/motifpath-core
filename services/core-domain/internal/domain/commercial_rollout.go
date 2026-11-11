package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// FlagName is a registered commercial activation flag, without the
// "monetization." prefix.
type FlagName string

// ErrUnknownFlag is returned for a flag name that is not registered.
var ErrUnknownFlag = errors.New("unknown commercial flag")

// CommercialRegistryRevision identifies the approved flag registry this code
// implements: names, prerequisites and OFF behaviour. Rollout evidence records
// it as the policy revision.
const CommercialRegistryRevision = "commercial-rollout-r5"

// commercialRegistry lists every flag in registry order with the flags that
// must also allow a subject before it does. Enabling a flag never enables its
// prerequisites.
var commercialRegistry = []struct {
	name          FlagName
	prerequisites []FlagName
}{
	{"content_ownership", nil},
	{"content_versions", []FlagName{"content_ownership"}},
	{"strapi_rights", []FlagName{"content_ownership"}},
	{"commercial_validation", []FlagName{"content_versions"}},
	{"entitlement_resolution", []FlagName{"content_ownership", "content_versions"}},
	{"access_grants", []FlagName{"entitlement_resolution"}},
	{"progress_retention", []FlagName{"access_grants"}},
	{"authorization_enforcement", []FlagName{"entitlement_resolution", "access_grants", "progress_retention"}},
	{"plus_catalog", []FlagName{"authorization_enforcement"}},
	{"plus_webhooks", []FlagName{"access_grants", "authorization_enforcement"}},
	{"plus_billing", []FlagName{"plus_webhooks"}},
	{"plus_checkout", []FlagName{"plus_catalog", "plus_webhooks", "plus_billing"}},
	{"teacher_paid_paths", []FlagName{"commercial_validation", "authorization_enforcement"}},
	{"path_purchase", []FlagName{"teacher_paid_paths", "plus_webhooks", "teacher_settlement"}},
	{"motifpath_purchase", []FlagName{"commercial_validation", "authorization_enforcement", "plus_webhooks"}},
	{"teacher_settlement", []FlagName{"plus_webhooks"}},
	{"private_beta", nil},
}

// CommercialFlags returns every registered flag in registry order.
func CommercialFlags() []FlagName {
	flags := make([]FlagName, len(commercialRegistry))
	for i, entry := range commercialRegistry {
		flags[i] = entry.name
	}
	return flags
}

// ParseFlagName accepts only a registered flag.
func ParseFlagName(name string) (FlagName, error) {
	for _, entry := range commercialRegistry {
		if string(entry.name) == name {
			return entry.name, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownFlag, name)
}

// Prerequisites returns the flags that must also allow a subject before this
// one does; nil for an unregistered flag.
func (f FlagName) Prerequisites() []FlagName {
	for _, entry := range commercialRegistry {
		if entry.name == f {
			return entry.prerequisites
		}
	}
	return nil
}

func (f FlagName) isRegistered() bool {
	_, err := ParseFlagName(string(f))
	return err == nil
}

// RolloutSubjectKind is the category of a rollout allowlist entry.
type RolloutSubjectKind string

const (
	RolloutSubjectUser    RolloutSubjectKind = "user"
	RolloutSubjectGroup   RolloutSubjectKind = "group"
	RolloutSubjectService RolloutSubjectKind = "service"
)

// ErrInvalidRolloutSubject is returned for an allowlist entry that is not
// user:<uuid>, group:<slug> or service:<name>.
var ErrInvalidRolloutSubject = errors.New("invalid rollout subject")

// RolloutSubject is a user, a group of users or a service. Operations are
// evaluated for a user or a service; a group only appears in allowlists.
type RolloutSubject struct {
	Kind RolloutSubjectKind
	ID   string
}

var (
	rolloutUserIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	rolloutSlugPattern   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
)

// ParseRolloutSubject parses an allowlist entry. There is no wildcard entry:
// opening a flag to everyone is the separate all-users switch.
func ParseRolloutSubject(value string) (RolloutSubject, error) {
	kind, id, found := strings.Cut(value, ":")
	if !found {
		return RolloutSubject{}, fmt.Errorf("%w: %q", ErrInvalidRolloutSubject, value)
	}
	var valid bool
	switch RolloutSubjectKind(kind) {
	case RolloutSubjectUser:
		valid = rolloutUserIDPattern.MatchString(id)
	case RolloutSubjectGroup, RolloutSubjectService:
		valid = rolloutSlugPattern.MatchString(id)
	}
	if !valid {
		return RolloutSubject{}, fmt.Errorf("%w: %q", ErrInvalidRolloutSubject, value)
	}
	return RolloutSubject{Kind: RolloutSubjectKind(kind), ID: id}, nil
}

func (s RolloutSubject) String() string {
	return string(s.Kind) + ":" + s.ID
}

// RolloutAudience is who an enabled flag allows. With AllUsers off, an empty
// list allows no one. AllUsers never covers a service.
type RolloutAudience struct {
	AllUsers bool
	Entries  []RolloutSubject
}

// FlagSetting is one flag's configured switch and audience. A disabled flag
// keeps its audience, so turning it back on restores who it allowed.
type FlagSetting struct {
	Enabled  bool
	Audience RolloutAudience
}

// CommercialActivation is the environment's activation at one revision: every
// flag's setting and every rollout group's members. A flag without a setting
// is OFF with an empty audience.
type CommercialActivation struct {
	revision int64
	flags    map[FlagName]FlagSetting
	members  map[string]map[string]bool
}

// NewCommercialActivation builds the activation at a revision from flag
// settings and group memberships (group slug → member user ids).
func NewCommercialActivation(revision int64, flags map[FlagName]FlagSetting, groups map[string][]string) CommercialActivation {
	members := make(map[string]map[string]bool, len(groups))
	for slug, userIDs := range groups {
		set := make(map[string]bool, len(userIDs))
		for _, userID := range userIDs {
			set[userID] = true
		}
		members[slug] = set
	}
	copied := make(map[FlagName]FlagSetting, len(flags))
	for name, setting := range flags {
		copied[name] = setting
	}
	return CommercialActivation{revision: revision, flags: copied, members: members}
}

// Revision is the activation revision this state was read at.
func (a CommercialActivation) Revision() int64 {
	return a.revision
}

// Setting returns a flag's configured setting; OFF with an empty audience
// when it has never been changed.
func (a CommercialActivation) Setting(flag FlagName) FlagSetting {
	return a.flags[flag]
}

// Effective reports whether a registered flag is enabled and every
// prerequisite is effective. The audience still decides who it allows.
func (a CommercialActivation) Effective(flag FlagName) bool {
	if !flag.isRegistered() || !a.flags[flag].Enabled {
		return false
	}
	for _, prerequisite := range flag.Prerequisites() {
		if !a.Effective(prerequisite) {
			return false
		}
	}
	return true
}

// Allows reports whether a flag allows a user or service: the flag must be
// enabled and list the subject, and every prerequisite must allow the same
// subject. Anything unregistered, unlisted or disabled is denied.
func (a CommercialActivation) Allows(flag FlagName, subject RolloutSubject) bool {
	if !flag.isRegistered() || !a.flags[flag].Enabled {
		return false
	}
	if !a.audienceCovers(a.flags[flag].Audience, subject) {
		return false
	}
	for _, prerequisite := range flag.Prerequisites() {
		if !a.Allows(prerequisite, subject) {
			return false
		}
	}
	return true
}

func (a CommercialActivation) audienceCovers(audience RolloutAudience, subject RolloutSubject) bool {
	switch subject.Kind {
	case RolloutSubjectUser:
		return audience.AllUsers || a.listsUser(audience, subject.ID)
	case RolloutSubjectService:
		return listsEntry(audience, subject)
	case RolloutSubjectGroup:
		// A group is a set of users in an allowlist, never the subject an
		// operation is evaluated for.
		return false
	default:
		return false
	}
}

func (a CommercialActivation) listsUser(audience RolloutAudience, userID string) bool {
	for _, entry := range audience.Entries {
		switch entry.Kind {
		case RolloutSubjectUser:
			if entry.ID == userID {
				return true
			}
		case RolloutSubjectGroup:
			if a.members[entry.ID][userID] {
				return true
			}
		case RolloutSubjectService:
			continue
		}
	}
	return false
}

func listsEntry(audience RolloutAudience, subject RolloutSubject) bool {
	for _, entry := range audience.Entries {
		if entry == subject {
			return true
		}
	}
	return false
}
