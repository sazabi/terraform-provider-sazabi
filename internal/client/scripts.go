package client

import (
	"context"
	"net/http"
	"net/url"
)

// Script mirrors ProjectScriptDetailSchema in
// packages/public-api-contracts/src/scripts.ts. The API keys scripts by
// name within a project; get/update/delete address them by name, not by ID.
// ContentHash is a volatile server-derived field surfaced for drift context.
type Script struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"projectId"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	ContentHash string  `json:"contentHash"`
	Content     string  `json:"content"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// CreateScriptInput mirrors CreateProjectScriptInputSchema. Content is
// required; Description is optional and nullable.
type CreateScriptInput struct {
	ProjectID   string  `json:"projectId,omitempty"`
	Name        string  `json:"name"`
	Content     string  `json:"content"`
	Description *string `json:"description,omitempty"`
}

// UpdateScriptInput mirrors UpdateProjectScriptInputSchema. Content and
// Description are optional: omit Content to leave the body unchanged, and
// set Description to a pointer-to-nil-string (via a nulled field) to clear
// it. The provider always sends Content on update since Terraform manages
// the full body.
type UpdateScriptInput struct {
	Content     *string `json:"content,omitempty"`
	Description *string `json:"description,omitempty"`
}

type scriptEnvelope struct {
	Script Script `json:"script"`
}

// CreateScript calls scripts.create (POST /scripts, 201).
func (c *Client) CreateScript(ctx context.Context, input CreateScriptInput) (*Script, error) {
	var out scriptEnvelope
	if err := c.Do(ctx, http.MethodPost, "/scripts", nil, input, &out); err != nil {
		return nil, err
	}
	return &out.Script, nil
}

// GetScript calls scripts.get (GET /scripts/{name}), including the body.
func (c *Client) GetScript(ctx context.Context, name, projectID string) (*Script, error) {
	var out scriptEnvelope
	if err := c.Do(ctx, http.MethodGet, "/scripts/"+url.PathEscape(name), scriptQuery(projectID), nil, &out); err != nil {
		return nil, err
	}
	return &out.Script, nil
}

// UpdateScript calls scripts.update (PATCH /scripts/{name}).
func (c *Client) UpdateScript(ctx context.Context, name, projectID string, input UpdateScriptInput) (*Script, error) {
	var out scriptEnvelope
	if err := c.Do(ctx, http.MethodPatch, "/scripts/"+url.PathEscape(name), scriptQuery(projectID), input, &out); err != nil {
		return nil, err
	}
	return &out.Script, nil
}

// DeleteScript calls scripts.delete (DELETE /scripts/{name}, 204), a soft delete.
func (c *Client) DeleteScript(ctx context.Context, name, projectID string) error {
	return c.Do(ctx, http.MethodDelete, "/scripts/"+url.PathEscape(name), scriptQuery(projectID), nil, nil)
}

// scriptQuery builds the optional projectId query parameter used by the
// name-keyed script endpoints.
func scriptQuery(projectID string) url.Values {
	if projectID == "" {
		return nil
	}
	return url.Values{"projectId": []string{projectID}}
}
