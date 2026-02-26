package hosted

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNangoProxyTransport_RewritesGET(t *testing.T) {
	var gotReq *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	transport := &NangoProxyTransport{
		Base:          backend.URL,
		SecretKey:     "nango-secret",
		IntegrationID: "dolthub",
		ConnectionID:  "conn-42",
		DoltHubBase:   "https://www.dolthub.com/api/v1alpha1",
	}
	client := &http.Client{Transport: transport}

	req, _ := http.NewRequest("GET", "https://www.dolthub.com/api/v1alpha1/org/db/main?q=SELECT+1", nil)
	req.Header.Set("authorization", "token old-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Verify URL was rewritten.
	if !strings.HasPrefix(gotReq.URL.Path, "/proxy/") {
		t.Errorf("expected /proxy/ prefix, got %s", gotReq.URL.Path)
	}
	expectedPath := "/proxy/org/db/main"
	if gotReq.URL.Path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, gotReq.URL.Path)
	}
	if gotReq.URL.RawQuery != "q=SELECT+1" {
		t.Errorf("expected query q=SELECT+1, got %s", gotReq.URL.RawQuery)
	}

	// Verify Nango headers.
	if got := gotReq.Header.Get("Authorization"); got != "Bearer nango-secret" {
		t.Errorf("expected Bearer nango-secret, got %s", got)
	}
	if got := gotReq.Header.Get("Connection-Id"); got != "conn-42" {
		t.Errorf("expected conn-42, got %s", got)
	}
	if got := gotReq.Header.Get("Provider-Config-Key"); got != "dolthub" {
		t.Errorf("expected dolthub, got %s", got)
	}

	// Old auth header must be stripped.
	if gotReq.Header.Get("authorization") == "token old-token" {
		t.Error("old authorization header was not stripped")
	}
}

func TestNangoProxyTransport_RewritesPOST(t *testing.T) {
	var gotMethod string
	var gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	transport := &NangoProxyTransport{
		Base:          backend.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
		ConnectionID:  "conn-1",
		DoltHubBase:   "https://www.dolthub.com/api/v1alpha1",
	}
	client := &http.Client{Transport: transport}

	body := `{"query":"INSERT INTO t VALUES (1)"}`
	req, _ := http.NewRequest("POST", "https://www.dolthub.com/api/v1alpha1/org/db/write/main/branch?q=INSERT", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotBody != body {
		t.Errorf("expected body %q, got %q", body, gotBody)
	}
}

func TestNangoProxyTransport_RewritesPATCH(t *testing.T) {
	var gotMethod string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	transport := &NangoProxyTransport{
		Base:          backend.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
		ConnectionID:  "conn-1",
		DoltHubBase:   "https://www.dolthub.com/api/v1alpha1",
	}
	client := &http.Client{Transport: transport}

	req, _ := http.NewRequest("PATCH", "https://www.dolthub.com/api/v1alpha1/org/db/pulls/123", strings.NewReader(`{"state":"closed"}`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if gotMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", gotMethod)
	}
}

func TestNangoProxyTransport_NonDoltHubPassesThrough(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	transport := &NangoProxyTransport{
		Base:          "https://api.nango.dev",
		SecretKey:     "nango-secret",
		IntegrationID: "dolthub",
		ConnectionID:  "conn-1",
		DoltHubBase:   "https://www.dolthub.com/api/v1alpha1",
	}

	// Use the backend server (not a DoltHub URL), so it should pass through.
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("GET", backend.URL+"/other/endpoint", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Original auth header should be preserved (not rewritten to Nango).
	if gotAuth != "Bearer my-token" {
		t.Errorf("expected original auth preserved, got %s", gotAuth)
	}
}

func TestNangoProxyTransport_QueryParamsPreserved(t *testing.T) {
	var gotQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	transport := &NangoProxyTransport{
		Base:          backend.URL,
		SecretKey:     "secret",
		IntegrationID: "dolthub",
		ConnectionID:  "conn-1",
		DoltHubBase:   "https://www.dolthub.com/api/v1alpha1",
	}
	client := &http.Client{Transport: transport}

	req, _ := http.NewRequest("GET", "https://www.dolthub.com/api/v1alpha1/org/db/main?q=SELECT+1&foo=bar", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if gotQuery != "q=SELECT+1&foo=bar" {
		t.Errorf("expected query params preserved, got %s", gotQuery)
	}
}
