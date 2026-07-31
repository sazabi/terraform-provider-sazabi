package client

import (
	"context"
	"net/http"
)

// SecretKey mirrors SecretKeySchema in
// packages/public-api-contracts/src/keys.ts. ProjectID is null for
// organization-wide keys. The plaintext Value is present only in the
// create response; no GET ever returns it again.
type SecretKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ProjectID  *string `json:"projectId"`
	ExpiresAt  *string `json:"expiresAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	CreatedAt  string  `json:"createdAt"`
	Value      string  `json:"value,omitempty"`
}

// CreateSecretKeyInput mirrors CreateSecretKeyInputSchema.
type CreateSecretKeyInput struct {
	ProjectID string `json:"projectId,omitempty"`
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// UpdateSecretKeyInput mirrors UpdateSecretKeyInputSchema. ExpiresAt uses a
// double pointer so the payload can distinguish "leave unchanged" (nil)
// from "clear the expiration" (pointer to nil).
type UpdateSecretKeyInput struct {
	Name      string
	ExpiresAt **string
}

type secretKeyEnvelope struct {
	SecretKey SecretKey `json:"secretKey"`
}

// CreateSecretKey calls secretKeys.create (POST /secret-keys). The returned
// key includes the plaintext Value — the only time the API ever returns it.
func (c *Client) CreateSecretKey(ctx context.Context, input CreateSecretKeyInput) (*SecretKey, error) {
	var out secretKeyEnvelope
	if err := c.Do(ctx, http.MethodPost, "/secret-keys", nil, input, &out); err != nil {
		return nil, err
	}
	return &out.SecretKey, nil
}

// GetSecretKey calls secretKeys.get (GET /secret-keys/{keyId}).
func (c *Client) GetSecretKey(ctx context.Context, keyID string) (*SecretKey, error) {
	var out secretKeyEnvelope
	if err := c.Do(ctx, http.MethodGet, "/secret-keys/"+keyID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.SecretKey, nil
}

// UpdateSecretKey calls secretKeys.update (PATCH /secret-keys/{keyId}).
func (c *Client) UpdateSecretKey(ctx context.Context, keyID string, input UpdateSecretKeyInput) (*SecretKey, error) {
	payload := map[string]any{}
	if input.Name != "" {
		payload["name"] = input.Name
	}
	if input.ExpiresAt != nil {
		if *input.ExpiresAt == nil {
			payload["expiresAt"] = nil
		} else {
			payload["expiresAt"] = **input.ExpiresAt
		}
	}
	var out secretKeyEnvelope
	if err := c.Do(ctx, http.MethodPatch, "/secret-keys/"+keyID, nil, payload, &out); err != nil {
		return nil, err
	}
	return &out.SecretKey, nil
}

// DeleteSecretKey calls secretKeys.delete (DELETE /secret-keys/{keyId}, 204).
func (c *Client) DeleteSecretKey(ctx context.Context, keyID string) error {
	return c.Do(ctx, http.MethodDelete, "/secret-keys/"+keyID, nil, nil, nil)
}
