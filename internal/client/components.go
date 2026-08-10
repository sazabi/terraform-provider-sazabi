package client

import (
	"context"
	"net/http"
)

// Component mirrors ComponentSchema in
// packages/public-api-contracts/src/components.ts. FirstSeenAt,
// LastSeenAt, and CurrentStatus are volatile runtime fields; the provider
// only exposes the stable subset in resource state.
type Component struct {
	ID            string  `json:"id"`
	ProjectID     string  `json:"projectId"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	CurrentStatus string  `json:"currentStatus"`
	FirstSeenAt   string  `json:"firstSeenAt"`
	LastSeenAt    string  `json:"lastSeenAt"`
	DeletedAt     *string `json:"deletedAt"`
}

// RegisterComponentInput mirrors RegisterComponentInputSchema.
// Registering an existing active component with the same name refreshes it
// (updating description when provided) instead of creating a duplicate.
type RegisterComponentInput struct {
	ProjectID   string  `json:"projectId,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type componentEnvelope struct {
	Component Component `json:"component"`
}

// RegisterComponent calls components.register (POST /components).
func (c *Client) RegisterComponent(ctx context.Context, input RegisterComponentInput) (*Component, error) {
	var out componentEnvelope
	if err := c.Do(ctx, http.MethodPost, "/components", nil, input, &out); err != nil {
		return nil, err
	}
	return &out.Component, nil
}

// GetComponent calls components.get (GET /components/{componentId}).
func (c *Client) GetComponent(ctx context.Context, componentID string) (*Component, error) {
	var out componentEnvelope
	if err := c.Do(ctx, http.MethodGet, "/components/"+componentID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Component, nil
}

// DeregisterComponent calls components.deregister
// (POST /components/{componentId}/deregister), a soft delete.
func (c *Client) DeregisterComponent(ctx context.Context, componentID string, reason string) (*Component, error) {
	body := map[string]string{}
	if reason != "" {
		body["reason"] = reason
	}
	var out componentEnvelope
	if err := c.Do(ctx, http.MethodPost, "/components/"+componentID+"/deregister", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Component, nil
}
