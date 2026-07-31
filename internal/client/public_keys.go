package client

import (
	"context"
	"net/http"
	"net/url"
)

// PublicKey mirrors PublicKeySchema in
// packages/public-api-contracts/src/keys.ts. Value is present only in
// responses from operations that mint or re-encrypt the key (the
// log-forwarding ensure operation returns it on every call).
type PublicKey struct {
	ID                     string  `json:"id"`
	ProjectID              string  `json:"projectId"`
	Name                   string  `json:"name"`
	DataSourceConnectionID *string `json:"dataSourceConnectionId"`
	DeactivatedAt          *string `json:"deactivatedAt"`
	ExpiresAt              *string `json:"expiresAt"`
	LastUsedAt             *string `json:"lastUsedAt"`
	CreatedAt              string  `json:"createdAt"`
	Value                  string  `json:"value,omitempty"`
}

type publicKeyEnvelope struct {
	PublicKey PublicKey `json:"publicKey"`
}

// EnsureLogForwardingPublicKey calls publicKeys.ensureLogForwarding
// (POST /public-keys/log-forwarding/ensure). The operation is an upsert:
// it adopts the project's existing active forwarding key when one exists
// and returns the plaintext value on every call.
func (c *Client) EnsureLogForwardingPublicKey(ctx context.Context, projectID string) (*PublicKey, error) {
	body := map[string]string{}
	if projectID != "" {
		body["projectId"] = projectID
	}
	var out publicKeyEnvelope
	if err := c.Do(ctx, http.MethodPost, "/public-keys/log-forwarding/ensure", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.PublicKey, nil
}

// GetPublicKey calls publicKeys.get (GET /public-keys/{keyId}). The
// response never includes the plaintext value.
func (c *Client) GetPublicKey(ctx context.Context, keyID, projectID string) (*PublicKey, error) {
	var query url.Values
	if projectID != "" {
		query = url.Values{"projectId": []string{projectID}}
	}
	var out publicKeyEnvelope
	if err := c.Do(ctx, http.MethodGet, "/public-keys/"+keyID, query, nil, &out); err != nil {
		return nil, err
	}
	return &out.PublicKey, nil
}

// DeactivatePublicKey calls publicKeys.deactivate
// (POST /public-keys/{keyId}/deactivate), a soft-disable.
func (c *Client) DeactivatePublicKey(ctx context.Context, keyID, projectID string) (*PublicKey, error) {
	var query url.Values
	if projectID != "" {
		query = url.Values{"projectId": []string{projectID}}
	}
	var out publicKeyEnvelope
	if err := c.Do(ctx, http.MethodPost, "/public-keys/"+keyID+"/deactivate", query, nil, &out); err != nil {
		return nil, err
	}
	return &out.PublicKey, nil
}
