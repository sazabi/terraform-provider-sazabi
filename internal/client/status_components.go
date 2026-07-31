package client

import (
	"context"
	"net/http"
)

// StatusComponent mirrors StatusComponentSchema in
// packages/public-api-contracts/src/status-components.ts. FirstSeenAt,
// LastSeenAt, and CurrentStatus are volatile runtime fields; the provider
// only exposes the stable subset in resource state.
type StatusComponent struct {
	ID            string  `json:"id"`
	ProjectID     string  `json:"projectId"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	CurrentStatus string  `json:"currentStatus"`
	FirstSeenAt   string  `json:"firstSeenAt"`
	LastSeenAt    string  `json:"lastSeenAt"`
	DeletedAt     *string `json:"deletedAt"`
}

// RegisterStatusComponentInput mirrors RegisterStatusComponentInputSchema.
// Registering an existing active component with the same name refreshes it
// (updating description when provided) instead of creating a duplicate.
type RegisterStatusComponentInput struct {
	ProjectID   string  `json:"projectId,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type statusComponentEnvelope struct {
	StatusComponent StatusComponent `json:"statusComponent"`
}

// RegisterStatusComponent calls statusComponents.register (POST /status-components).
func (c *Client) RegisterStatusComponent(ctx context.Context, input RegisterStatusComponentInput) (*StatusComponent, error) {
	var out statusComponentEnvelope
	if err := c.Do(ctx, http.MethodPost, "/status-components", nil, input, &out); err != nil {
		return nil, err
	}
	return &out.StatusComponent, nil
}

// GetStatusComponent calls statusComponents.get (GET /status-components/{componentId}).
func (c *Client) GetStatusComponent(ctx context.Context, componentID string) (*StatusComponent, error) {
	var out statusComponentEnvelope
	if err := c.Do(ctx, http.MethodGet, "/status-components/"+componentID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.StatusComponent, nil
}

// DeregisterStatusComponent calls statusComponents.deregister
// (POST /status-components/{componentId}/deregister), a soft delete.
func (c *Client) DeregisterStatusComponent(ctx context.Context, componentID string, reason string) (*StatusComponent, error) {
	body := map[string]string{}
	if reason != "" {
		body["reason"] = reason
	}
	var out statusComponentEnvelope
	if err := c.Do(ctx, http.MethodPost, "/status-components/"+componentID+"/deregister", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.StatusComponent, nil
}
