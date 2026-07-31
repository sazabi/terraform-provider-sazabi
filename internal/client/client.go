// Package client is a minimal HTTP client for the Sazabi public API.
//
// It mirrors the transport behavior of packages/public-sdk in sazabi/monorepo:
// Bearer secret-key auth against {base_url}/v1, an x-sazabi-client-source
// header, and structured {code, message, operationId} error payloads. The
// public API documents no rate limits, so the client applies conservative
// exponential backoff on 429 and 5xx responses by default.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production public API host, excluding the
	// version path segment (the client appends /v1 itself).
	DefaultBaseURL = "https://api.sazabi.com"

	apiVersionPath = "/v1"
	clientSource   = "terraform-provider"

	maxRetries     = 4
	baseRetryDelay = 500 * time.Millisecond
	maxRetryDelay  = 8 * time.Second
)

// Config configures a Client.
type Config struct {
	// BaseURL is the API host without the /v1 path. Defaults to DefaultBaseURL.
	BaseURL string
	// APIKey is a Sazabi secret key (sazabi_secret_...).
	APIKey string
	// UserAgent identifies the provider build, e.g. "terraform-provider-sazabi/0.1.0".
	UserAgent string
	// HTTPClient overrides the underlying HTTP client. Defaults to a
	// 30-second-timeout client.
	HTTPClient *http.Client
}

// Client talks to the Sazabi public API.
type Client struct {
	baseURL    string
	apiKey     string
	userAgent  string
	httpClient *http.Client
}

// APIError is a structured error response from the public API.
type APIError struct {
	StatusCode  int
	Code        string
	Message     string
	OperationID string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("sazabi api error (HTTP %d, %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("sazabi api error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsNotFound reports whether err is an APIError with HTTP 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// New creates a Client. APIKey is required; BaseURL defaults to production.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("api key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base url %q: %w", baseURL, err)
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    baseURL + apiVersionPath,
		apiKey:     cfg.APIKey,
		userAgent:  cfg.UserAgent,
		httpClient: httpClient,
	}, nil
}

// Do issues one API request. path is relative to /v1 (e.g. "/projects").
// body (when non-nil) is JSON-encoded; out (when non-nil) receives the
// decoded JSON response. 429 and 5xx responses are retried with exponential
// backoff before returning an error.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	requestURL := c.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay(attempt)):
			}
		}

		var requestBody io.Reader
		if encoded != nil {
			requestBody = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("x-sazabi-client-source", clientSource)
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}
		if encoded != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		retryable, err := c.handleResponse(resp, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable {
			return err
		}
	}
	return fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// handleResponse decodes one HTTP response. It returns (retryable, error);
// error is nil on success.
func (c *Client) handleResponse(resp *http.Response, out any) (bool, error) {
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return true, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || resp.StatusCode == http.StatusNoContent || len(responseBody) == 0 {
			return false, nil
		}
		if err := json.Unmarshal(responseBody, out); err != nil {
			return false, fmt.Errorf("decoding response body: %w", err)
		}
		return false, nil
	}

	apiErr := &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(responseBody))}
	var payload struct {
		Code        string `json:"code"`
		Message     string `json:"message"`
		OperationID string `json:"operationId"`
	}
	if json.Unmarshal(responseBody, &payload) == nil && payload.Message != "" {
		apiErr.Code = payload.Code
		apiErr.Message = payload.Message
		apiErr.OperationID = payload.OperationID
	}

	retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	return retryable, apiErr
}

// retryDelay returns the exponential backoff delay for the given attempt
// (1-based), with up to 25% jitter, capped at maxRetryDelay.
func retryDelay(attempt int) time.Duration {
	delay := baseRetryDelay << (attempt - 1)
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	jitter := time.Duration(rand.Int63n(int64(delay) / 4))
	return delay + jitter
}
