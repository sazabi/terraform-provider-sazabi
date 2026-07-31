package client

import (
	"context"
	"net/http"
	"net/url"
)

// Automation mirrors the stable subset of AutomationDetailSchema in
// packages/public-api-contracts/src/automations.ts. Run-health fields
// (health, successRate, lastRun, run counts) are volatile runtime data the
// provider does not track.
type Automation struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	CanToggle bool   `json:"canToggle"`
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

// GetAutomation calls automations.get (GET /automations/{automationId}).
func (c *Client) GetAutomation(ctx context.Context, automationID, projectID string) (*Automation, error) {
	var out automationEnvelope
	if err := c.Do(ctx, http.MethodGet, "/automations/"+automationID, automationQuery(projectID), nil, &out); err != nil {
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
