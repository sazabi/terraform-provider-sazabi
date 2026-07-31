package client

import (
	"context"
	"net/http"
)

// DataSourceConnection mirrors DataSourceConnectionSchema in
// packages/public-api-contracts/src/data-sources.ts. Connection metadata
// (credentials) is write-only: no GET ever returns it.
type DataSourceConnection struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	DataSourceType string  `json:"dataSourceType"`
	DisplayName    *string `json:"displayName"`
	CreatedAt      string  `json:"createdAt"`
}

// CreateDataSourceConnectionInput mirrors CreateDataSourceConnectionInputSchema.
type CreateDataSourceConnectionInput struct {
	ProjectID      string            `json:"projectId,omitempty"`
	DataSourceType string            `json:"dataSourceType"`
	Metadata       map[string]string `json:"metadata"`
	DisplayName    string            `json:"displayName,omitempty"`
}

// CreateDataSourceConnectionOutput mirrors the create response: the
// connection ID plus a public key shown exactly once.
type CreateDataSourceConnectionOutput struct {
	ConnectionID string `json:"connectionId"`
	PublicKey    string `json:"publicKey"`
}

// CreateDataSourceConnection calls dataSources.createConnection
// (POST /data-sources/connections). Validates credentials server-side.
func (c *Client) CreateDataSourceConnection(ctx context.Context, input CreateDataSourceConnectionInput) (*CreateDataSourceConnectionOutput, error) {
	var out CreateDataSourceConnectionOutput
	if err := c.Do(ctx, http.MethodPost, "/data-sources/connections", nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDataSourceConnection calls dataSources.getConnection
// (GET /data-sources/connections/{connectionId}).
func (c *Client) GetDataSourceConnection(ctx context.Context, connectionID string) (*DataSourceConnection, error) {
	var out struct {
		Connection DataSourceConnection `json:"connection"`
	}
	if err := c.Do(ctx, http.MethodGet, "/data-sources/connections/"+connectionID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Connection, nil
}

// DisconnectDataSourceConnectionOutput mirrors the disconnect response.
type DisconnectDataSourceConnectionOutput struct {
	Success                 bool    `json:"success"`
	ConnectionTeardownError *string `json:"connectionTeardownError"`
}

// DisconnectDataSourceConnection calls dataSources.disconnectConnection
// (DELETE /data-sources/connections/{connectionId}), soft-deleting the
// connection, its streams, and its API keys.
func (c *Client) DisconnectDataSourceConnection(ctx context.Context, connectionID string) (*DisconnectDataSourceConnectionOutput, error) {
	var out DisconnectDataSourceConnectionOutput
	if err := c.Do(ctx, http.MethodDelete, "/data-sources/connections/"+connectionID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DataSourceStream mirrors DataSourceStreamSchema. Status and errorMessage
// are volatile provisioning fields the provider does not track as state.
type DataSourceStream struct {
	ID           string         `json:"id"`
	ConnectionID *string        `json:"connectionId"`
	DisplayName  string         `json:"displayName"`
	Config       map[string]any `json:"config"`
	Status       string         `json:"status"`
	ErrorMessage *string        `json:"errorMessage"`
	Enabled      bool           `json:"enabled"`
	CreatedAt    string         `json:"createdAt"`
}

// CreateDataSourceStreamInput mirrors CreateDataSourceStreamInputSchema
// minus the path-bound connection ID.
type CreateDataSourceStreamInput struct {
	DisplayName string            `json:"displayName"`
	Config      map[string]string `json:"config"`
}

// CreateDataSourceStreamOutput mirrors the create response. PublicKey is
// present only for sources that mint a per-stream key.
type CreateDataSourceStreamOutput struct {
	StreamID  string `json:"streamId"`
	PublicKey string `json:"publicKey,omitempty"`
}

// CreateDataSourceStream calls dataSources.createStream
// (POST /data-sources/connections/{connectionId}/streams).
func (c *Client) CreateDataSourceStream(ctx context.Context, connectionID string, input CreateDataSourceStreamInput) (*CreateDataSourceStreamOutput, error) {
	var out CreateDataSourceStreamOutput
	if err := c.Do(ctx, http.MethodPost, "/data-sources/connections/"+connectionID+"/streams", nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDataSourceStream calls dataSources.getStream
// (GET /data-sources/streams/{streamId}).
func (c *Client) GetDataSourceStream(ctx context.Context, streamID string) (*DataSourceStream, error) {
	var out struct {
		Stream DataSourceStream `json:"stream"`
	}
	if err := c.Do(ctx, http.MethodGet, "/data-sources/streams/"+streamID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Stream, nil
}

// DeleteDataSourceStream calls dataSources.deleteStream
// (DELETE /data-sources/streams/{streamId}).
func (c *Client) DeleteDataSourceStream(ctx context.Context, streamID string) error {
	return c.Do(ctx, http.MethodDelete, "/data-sources/streams/"+streamID, nil, nil, nil)
}
