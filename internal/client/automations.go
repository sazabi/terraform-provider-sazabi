package client

import (
	"context"
	"net/http"
	"net/url"
)

// Automation mirrors the stable subset of AutomationDetailSchema in
// packages/public-api-contracts/src/automations.ts. Run-health fields
// (health, successRate, lastRun, run counts) are volatile runtime data the
// provider does not track. ScriptID/ScriptName identify the DB-backed
// project script the automation runs.
type Automation struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	ScriptID       *string `json:"scriptId"`
	ScriptName     *string `json:"scriptName"`
	Enabled        bool    `json:"enabled"`
	CronExpression *string `json:"cronExpression"`
	Timezone       string  `json:"timezone"`
	TimeoutSeconds *int64  `json:"timeoutSeconds"`
	CanToggle      bool    `json:"canToggle"`
}

// CreateAutomationInput mirrors CreateAutomationInputSchema. Exactly one of
// ScriptID or Script (script name) must be set. CronExpression, Timezone,
// TimeoutSeconds, and Enabled are optional; the API defaults them to every
// minute, UTC, 60 seconds, and true respectively.
type CreateAutomationInput struct {
	ProjectID      string  `json:"projectId,omitempty"`
	Name           string  `json:"name"`
	Description    *string `json:"description,omitempty"`
	ScriptID       string  `json:"scriptId,omitempty"`
	Script         string  `json:"script,omitempty"`
	CronExpression string  `json:"cronExpression,omitempty"`
	Timezone       string  `json:"timezone,omitempty"`
	TimeoutSeconds *int64  `json:"timeoutSeconds,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

// UpdateAutomationInput mirrors UpdateAutomationInputSchema. It does not
// change which script the automation runs, nor its enabled state (use
// SetAutomationEnabled for that). Omitted fields are left unchanged.
type UpdateAutomationInput struct {
	Name           string  `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	CronExpression string  `json:"cronExpression,omitempty"`
	Timezone       string  `json:"timezone,omitempty"`
	TimeoutSeconds *int64  `json:"timeoutSeconds,omitempty"`
}

type automationEnvelope struct {
	Automation Automation `json:"automation"`
}

func automationQuery(projectID string) url.Values {
	if projectID == "" {
		return nil
	}
	return url.Values{"projectId": []string{projectID}}
}

// CreateAutomation calls automations.create (POST /automations, 201).
func (c *Client) CreateAutomation(ctx context.Context, input CreateAutomationInput) (*Automation, error) {
	var out automationEnvelope
	if err := c.Do(ctx, http.MethodPost, "/automations", nil, input, &out); err != nil {
		return nil, err
	}
	return &out.Automation, nil
}

// GetAutomation calls automations.get (GET /automations/{automationId}).
func (c *Client) GetAutomation(ctx context.Context, automationID, projectID string) (*Automation, error) {
	var out automationEnvelope
	if err := c.Do(ctx, http.MethodGet, "/automations/"+automationID, automationQuery(projectID), nil, &out); err != nil {
		return nil, err
	}
	return &out.Automation, nil
}

// UpdateAutomation calls automations.update (PATCH /automations/{automationId}).
// It updates name/description/schedule only; it cannot change the script the
// automation runs or its enabled state.
func (c *Client) UpdateAutomation(ctx context.Context, automationID, projectID string, input UpdateAutomationInput) (*Automation, error) {
	var out automationEnvelope
	if err := c.Do(ctx, http.MethodPatch, "/automations/"+automationID, automationQuery(projectID), input, &out); err != nil {
		return nil, err
	}
	return &out.Automation, nil
}

// SetAutomationEnabled calls automations.enable or automations.disable
// (POST /automations/{automationId}/enable|disable).
func (c *Client) SetAutomationEnabled(ctx context.Context, automationID, projectID string, enabled bool) (*Automation, error) {
	action := "/disable"
	if enabled {
		action = "/enable"
	}
	body := map[string]string{}
	if projectID != "" {
		body["projectId"] = projectID
	}
	var out automationEnvelope
	if err := c.Do(ctx, http.MethodPost, "/automations/"+automationID+action, nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Automation, nil
}
