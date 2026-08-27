// Package coredomain adapts the Core Domain Service's HTTP API for use by the
// Event Ingestion Service. It is the identity/authorization capability the
// service consults to establish a caller's MotifPath user id (ADR-014) and
// role (ADR-013), rather than trusting claims in the caller's JWT.
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

// defaultTimeout bounds a single profile lookup. A slow Core Domain Service
// should surface as a prompt "try again" rather than a hanging request.
const defaultTimeout = 3 * time.Second

// Client calls the Core Domain Service's HTTP API with a caller's own
// forwarded bearer token. It implements ports.ProfileResolver.
type Client struct {
	baseURL string
	client  *http.Client
}

var _ ports.ProfileResolver = (*Client)(nil)

// NewClient builds a client targeting baseURL (the Core Domain Service's root,
// e.g. http://core-domain:8080). A nil httpClient gets a default with
// defaultTimeout.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), client: httpClient}
}

type profileResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// ResolveProfile implements ports.ProfileResolver. It maps a 200 to the
// profile, a 404 to ports.ErrIdentityNotRegistered, and every other outcome
// (transport failure, timeout, unexpected status, unreadable or incomplete
// body) to ports.ErrProfileUnavailable so callers fail closed.
func (c *Client) ResolveProfile(ctx context.Context, bearerToken string) (ports.Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/users/me", nil)
	if err != nil {
		return ports.Profile{}, fmt.Errorf("%w: build request: %v", ports.ErrProfileUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return ports.Profile{}, fmt.Errorf("%w: call core domain service: %v", ports.ErrProfileUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body profileResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return ports.Profile{}, fmt.Errorf("%w: decode profile: %v", ports.ErrProfileUnavailable, err)
		}
		if body.UserID == "" || body.Role == "" {
			return ports.Profile{}, fmt.Errorf("%w: profile was missing user_id or role", ports.ErrProfileUnavailable)
		}
		return ports.Profile{UserID: body.UserID, Role: body.Role}, nil
	case http.StatusNotFound:
		return ports.Profile{}, ports.ErrIdentityNotRegistered
	default:
		return ports.Profile{}, fmt.Errorf("%w: core domain service returned status %d", ports.ErrProfileUnavailable, resp.StatusCode)
	}
}
