package domain

import (
	"errors"
	"fmt"
)

// Environment is the trusted identity of the deployment the service runs in.
// Commercial operation keys and audit evidence are namespaced by it, so two
// deployments sharing an identifier can never be mistaken for one another.
type Environment struct {
	name string
}

// ErrUnknownEnvironment is returned for a deployment name that is not one of
// the known environments.
var ErrUnknownEnvironment = errors.New("unknown environment")

var knownEnvironments = map[string]bool{
	"local":      true,
	"test":       true,
	"staging":    true,
	"production": true,
}

// ParseEnvironment accepts exactly one of local, test, staging or production.
// Nothing is trimmed or case-folded: the value is an identity, not user input.
func ParseEnvironment(name string) (Environment, error) {
	if !knownEnvironments[name] {
		return Environment{}, fmt.Errorf("%w: %q", ErrUnknownEnvironment, name)
	}
	return Environment{name: name}, nil
}

func (e Environment) String() string {
	return e.name
}
