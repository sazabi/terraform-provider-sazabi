package client

import (
	"context"
	"net/http"
	"net/url"
)

// IntegrationConnection mirrors the stable, non-secret subset of
// IntegrationConnectionSchema in packages/public-api-contracts/src/integrations.ts.
// Integrations connect through interactive OAuth/app-installation flows, so
// the public API exposes them read-only.
type IntegrationConnection struct {
	ID             string  `json:"id"`
	Provider       string  `json:"provider"`
	DisplayName    *string `json:"displayName"`
	Status         string  `json:"status"`
	IsActive       bool    `json:"isActive"`
	NeedsAttention bool    `json:"needsAttention"`
	HealthStatus   string  `json:"healthStatus"`
	ConnectedBy    *string `json:"connectedBy"`
	CreatedAt      string  `json:"createdAt"`
}

// GetIntegrationConnection calls integrations.getConnection
// (GET /integrations/connections/{connectionId}).
func (c *Client) GetIntegrationConnection(ctx context.Context, connectionID, organizationID string) (*IntegrationConnection, error) {
	var query url.Values
	if organizationID != "" {
		query = url.Values{"organizationId": []string{organizationID}}
	}
	var out struct {
		Connection IntegrationConnection `json:"connection"`
	}
	if err := c.Do(ctx, http.MethodGet, "/integrations/connections/"+connectionID, query, nil, &out); err != nil {
		return nil, err
	}
	return &out.Connection, nil
}

// McpConnector mirrors the stable subset of McpConnectorSchema in
// packages/public-api-contracts/src/mcp-connectors.ts. The endpoints are
// documented read-only in the contract source.
type McpConnector struct {
	ConnectionID     string `json:"connectionId"`
	ConnectionKey    string `json:"connectionKey"`
	ProviderID       string `json:"providerId"`
	DisplayName      string `json:"displayName"`
	Source           string `json:"source"`
	InstallStatus    string `json:"installStatus"`
	AuthMode         string `json:"authMode"`
	Transport        string `json:"transport"`
	ServerURL        string `json:"serverUrl"`
	ReadOnly         bool   `json:"readOnly"`
	EnabledToolCount int    `json:"enabledToolCount"`
	CreatedAt        string `json:"createdAt"`
}

// GetMcpConnector calls mcpConnectors.get (GET /mcp-connectors/{connectionId}).
func (c *Client) GetMcpConnector(ctx context.Context, connectionID, projectID string) (*McpConnector, error) {
	var query url.Values
	if projectID != "" {
		query = url.Values{"projectId": []string{projectID}}
	}
	var out struct {
		Connector McpConnector `json:"connector"`
	}
	if err := c.Do(ctx, http.MethodGet, "/mcp-connectors/"+connectionID, query, nil, &out); err != nil {
		return nil, err
	}
	return &out.Connector, nil
}
