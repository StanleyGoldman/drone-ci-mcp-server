package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret"
	testAccessToken  = "test-access-token"
)

func newOAuthMux() *http.ServeMux {
	mux := http.NewServeMux()
	registerOAuth(mux, testClientID, testClientSecret, testAccessToken)
	return mux
}

func doGet(t *testing.T, rawURL string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func doPostForm(t *testing.T, rawURL string, form url.Values) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestOAuthMetadata(t *testing.T) {
	srv := httptest.NewServer(newOAuthMux())
	defer srv.Close()

	resp := doGet(t, srv.URL+"/.well-known/oauth-authorization-server")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var meta map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if meta["token_endpoint"] == "" {
		t.Error("token_endpoint missing from metadata")
	}
	if meta["authorization_endpoint"] == "" {
		t.Error("authorization_endpoint missing from metadata")
	}
}

func TestOAuthTokenClientCredentials_Post(t *testing.T) {
	srv := httptest.NewServer(newOAuthMux())
	defer srv.Close()

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {testClientID},
		"client_secret": {testClientSecret},
	}
	resp := doPostForm(t, srv.URL+"/oauth/token", form)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var tok map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tok["access_token"] != testAccessToken {
		t.Errorf("access_token = %v, want %q", tok["access_token"], testAccessToken)
	}
	if tok["token_type"] != "Bearer" {
		t.Errorf("token_type = %v, want Bearer", tok["token_type"])
	}
}

func TestOAuthTokenClientCredentials_Basic(t *testing.T) {
	srv := httptest.NewServer(newOAuthMux())
	defer srv.Close()

	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(testClientID, testClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestOAuthTokenInvalidClient(t *testing.T) {
	srv := httptest.NewServer(newOAuthMux())
	defer srv.Close()

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"wrong-id"},
		"client_secret": {"wrong-secret"},
	}
	resp := doPostForm(t, srv.URL+"/oauth/token", form)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "invalid_client" {
		t.Errorf("error = %q, want invalid_client", body["error"])
	}
}

func TestOAuthTokenWrongGrantType(t *testing.T) {
	srv := httptest.NewServer(newOAuthMux())
	defer srv.Close()

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {testClientID},
		"client_secret": {testClientSecret},
	}
	resp := doPostForm(t, srv.URL+"/oauth/token", form)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestOAuthAuthorizeEndpoint(t *testing.T) {
	srv := httptest.NewServer(newOAuthMux())
	defer srv.Close()

	resp := doGet(t, srv.URL+"/oauth/authorize")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
