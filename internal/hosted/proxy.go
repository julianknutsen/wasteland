package hosted

import (
	"net/http"
	"strings"
)

// NangoProxyTransport is an http.RoundTripper that rewrites DoltHub API
// requests to go through Nango's proxy. Nango injects the user's stored
// token, so the server never sees or stores the token itself.
type NangoProxyTransport struct {
	Base          string // Nango API base, e.g. "https://api.nango.dev"
	SecretKey     string
	IntegrationID string
	ConnectionID  string
	DoltHubBase   string            // default "https://www.dolthub.com/api/v1alpha1"
	Inner         http.RoundTripper // nil means http.DefaultTransport
}

func (t *NangoProxyTransport) inner() http.RoundTripper {
	if t.Inner != nil {
		return t.Inner
	}
	return http.DefaultTransport
}

func (t *NangoProxyTransport) dolthubBase() string {
	if t.DoltHubBase != "" {
		return t.DoltHubBase
	}
	return "https://www.dolthub.com/api/v1alpha1"
}

// RoundTrip intercepts requests targeting the DoltHub API and rewrites them
// to Nango's proxy endpoint, adding the required Nango auth headers.
// Non-DoltHub requests pass through unchanged.
func (t *NangoProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqURL := req.URL.String()
	base := t.dolthubBase()

	if !strings.HasPrefix(reqURL, base) {
		return t.inner().RoundTrip(req)
	}

	suffix := strings.TrimPrefix(reqURL, base)
	suffix = strings.TrimPrefix(suffix, "/")

	proxyURL := t.Base + "/proxy/" + suffix

	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, proxyURL, req.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers, then replace auth with Nango headers.
	for k, vv := range req.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	proxyReq.Header.Del("Authorization")
	proxyReq.Header.Del("authorization")
	proxyReq.Header.Set("Authorization", "Bearer "+t.SecretKey)
	proxyReq.Header.Set("Connection-Id", t.ConnectionID)
	proxyReq.Header.Set("Provider-Config-Key", t.IntegrationID)
	// Override the provider's default base URL (e.g. apify) with DoltHub's.
	proxyReq.Header.Set("Base-Url-Override", base)

	return t.inner().RoundTrip(proxyReq)
}

// NewNangoProxyClient creates an *http.Client whose transport rewrites
// DoltHub API calls through Nango's proxy.
func NewNangoProxyClient(nangoBaseURL, secretKey, integrationID, connectionID string) *http.Client {
	return &http.Client{
		Transport: &NangoProxyTransport{
			Base:          nangoBaseURL,
			SecretKey:     secretKey,
			IntegrationID: integrationID,
			ConnectionID:  connectionID,
		},
	}
}
