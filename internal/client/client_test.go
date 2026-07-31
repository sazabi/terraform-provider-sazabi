package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDoSetsAuthAndClientSourceHeaders(t *testing.T) {
	var gotAuth, gotSource, gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotSource = r.Header.Get("x-sazabi-client-source")
		gotUserAgent = r.Header.Get("User-Agent")
		if r.URL.Path != "/v1/projects" {
			t.Errorf("expected path /v1/projects, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"projects": []any{}})
	}))
	defer server.Close()

	c, err := New(Config{BaseURL: server.URL, APIKey: "sazabi_secret_test", UserAgent: "terraform-provider-sazabi/test"})
	if err != nil {
		t.Fatal(err)
	}

	var out map[string]any
	if err := c.Do(context.Background(), http.MethodGet, "/projects", nil, nil, &out); err != nil {
		t.Fatal(err)
	}

	if gotAuth != "Bearer sazabi_secret_test" {
		t.Errorf("expected bearer auth header, got %q", gotAuth)
	}
	if gotSource != "terraform-provider" {
		t.Errorf("expected client source header, got %q", gotSource)
	}
	if gotUserAgent != "terraform-provider-sazabi/test" {
		t.Errorf("expected user agent header, got %q", gotUserAgent)
	}
}

func TestDoDecodesStructuredAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":        "NOT_FOUND",
			"message":     "Project not found.",
			"operationId": "projects.get",
		})
	}))
	defer server.Close()

	c, err := New(Config{BaseURL: server.URL, APIKey: "sazabi_secret_test"})
	if err != nil {
		t.Fatal(err)
	}

	err = c.Do(context.Background(), http.MethodGet, "/projects/missing", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound to be true, got %v", err)
	}
}

func TestDoRetriesOn5xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	c, err := New(Config{BaseURL: server.URL, APIKey: "sazabi_secret_test"})
	if err != nil {
		t.Fatal(err)
	}

	var out map[string]any
	if err := c.Do(context.Background(), http.MethodGet, "/projects", nil, nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestDoDoesNotRetryOn4xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":        "UNAUTHORIZED",
			"message":     "Invalid API key.",
			"operationId": "projects.list",
		})
	}))
	defer server.Close()

	c, err := New(Config{BaseURL: server.URL, APIKey: "sazabi_secret_bad"})
	if err != nil {
		t.Fatal(err)
	}

	err = c.Do(context.Background(), http.MethodGet, "/projects", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call (no retries on 4xx), got %d", calls.Load())
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for missing api key")
	}
}
