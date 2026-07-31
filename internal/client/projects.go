package client

import (
	"context"
	"net/http"
)

// Project mirrors ProjectSchema in packages/public-api-contracts/src/project.ts.
// OrganizationID is a slug-like string, not a UUID.
type Project struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Region         string `json:"region"`
}

// ProjectRegions are the valid values for Project.Region, from
// ProjectRegionSchema in the public API contracts.
var ProjectRegions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	"eu-central-1",
	"eu-central-2",
	"eu-north-1",
	"eu-south-1",
	"eu-south-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
}

// CreateProjectInput mirrors CreateProjectInputSchema. OrganizationID and
// Region are optional; the API defaults region to us-west-2.
type CreateProjectInput struct {
	OrganizationID string `json:"organizationId,omitempty"`
	Name           string `json:"name"`
	Region         string `json:"region,omitempty"`
}

type projectEnvelope struct {
	Project Project `json:"project"`
}

// CreateProject calls projects.create (POST /projects).
func (c *Client) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	var out projectEnvelope
	if err := c.Do(ctx, http.MethodPost, "/projects", nil, input, &out); err != nil {
		return nil, err
	}
	return &out.Project, nil
}

// GetProject calls projects.get (GET /projects/{projectId}).
func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	var out projectEnvelope
	if err := c.Do(ctx, http.MethodGet, "/projects/"+projectID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Project, nil
}

// UpdateProject calls projects.update (PATCH /projects/{projectId}),
// which is rename-only: name is the sole mutable field.
func (c *Client) UpdateProject(ctx context.Context, projectID, name string) (*Project, error) {
	var out projectEnvelope
	body := map[string]string{"name": name}
	if err := c.Do(ctx, http.MethodPatch, "/projects/"+projectID, nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Project, nil
}

// DeleteProject calls projects.delete (DELETE /projects/{projectId}, 204).
// The API rejects deleting an organization's last active project.
func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	return c.Do(ctx, http.MethodDelete, "/projects/"+projectID, nil, nil, nil)
}
