// Package coredomain adapts the Core Domain Service's HTTP API for use by the
// Event Ingestion Service. Per ADR-013 it is the identity/authorization
// capability the outbox admin endpoints consult to establish a caller's role.
package coredomain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/motifpath/event-ingestion/internal/ports"
)

// defaultTimeout bounds a single role lookup. The caller is a human operator
// remediating a dead-lettered entry, so a slow identity service should surface
// as a prompt "try again" rather than a hanging request (ADR-013).
const defaultTimeout = 3 * time.Second

// RoleResolver resolves a caller's role by calling GET /users/me on the Core
// Domain Service with the caller's own forwarded bearer token. It implements
// ports.RoleResolver.
type RoleResolver struct {
	baseURL string
	client  *http.Client
}

var _ ports.RoleResolver = (*RoleResolver)(nil)

// NewRoleResolver builds a resolver targeting baseURL (the Core Domain
// Service's root, e.g. http://core-domain:8080). A nil client gets a default
// with defaultTimeout.
func NewRoleResolver(baseURL string, client *http.Client) *RoleResolver {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &RoleResolver{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

type profileResponse struct {
	Role string `json:"role"`
}

// ResolveRole implements ports.RoleResolver. It maps a 200 to the profile's
// role, a 404 to ports.ErrIdentityNotRegistered, and every other outcome
// (transport failure, timeout, unexpected status, unreadable body) to
// ports.ErrRoleUnavailable so the caller fails closed.
func (r *RoleResolver) ResolveRole(ctx context.Context, bearerToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/users/me", nil)
	if err != nil {
		return "", fmt.Errorf("%w: build request: %v", ports.ErrRoleUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: call core domain service: %v", ports.ErrRoleUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body profileResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return "", fmt.Errorf("%w: decode profile: %v", ports.ErrRoleUnavailable, err)
		}
		if body.Role == "" {
			return "", fmt.Errorf("%w: profile carried no role", ports.ErrRoleUnavailable)
		}
		return body.Role, nil
	case http.StatusNotFound:
		return "", ports.ErrIdentityNotRegistered
	default:
		return "", fmt.Errorf("%w: core domain service returned status %d", ports.ErrRoleUnavailable, resp.StatusCode)
	}
}
