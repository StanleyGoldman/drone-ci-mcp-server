package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"io"
	"net/url"
	"strings"
	"testing"
)

const (
	testClientSecret = "test-client-secret"
	testAccessToken  = "test-access-token"
)

func newTestOAuthServer() (*oauthServer, *httptest.Server) {
	s := newOAuthServer(testClientSecret, testAccessToken)
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return s, httptest.NewServer(mux)
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, rawURL,
		strings.NewReader(form.Encode()))
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

// pkceChallenge returns a valid S256 code_challenge for the given verifier.
func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// --- Metadata ---

func TestOAuthMetadata_AdvertisesPKCE(t *testing.T) {
	_, srv := newTestOAuthServer()
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
	methods, _ := meta["code_challenge_methods_supported"].([]any)
	if len(methods) == 0 || methods[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v, want [S256]", methods)
	}
	types, _ := meta["grant_types_supported"].([]any)
	found := false
	for _, g := range types {
		if g == "authorization_code" {
			found = true
		}
	}
	if !found {
		t.Errorf("grant_types_supported does not include authorization_code: %v", types)
	}
}

// --- Authorize GET ---

func TestOAuthAuthorizeForm_ReturnsHTML(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	u := srv.URL + "/oauth/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"https://claude.ai/client"},
		"redirect_uri":          {"http://localhost:3000/cb"},
		"state":                 {"xyz"},
		"code_challenge":        {pkceChallenge("dummyverifier123456789012345678901234567890")},
		"code_challenge_method": {"S256"},
	}.Encode()

	resp := doGet(t, u)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestOAuthAuthorizeForm_RejectsBadResponseType(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	u := srv.URL + "/oauth/authorize?" + url.Values{
		"response_type":         {"token"},
		"client_id":             {"client"},
		"redirect_uri":          {"http://localhost/cb"},
		"code_challenge":        {"abc"},
		"code_challenge_method": {"S256"},
	}.Encode()

	resp := doGet(t, u)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestOAuthAuthorizeForm_RejectsNonS256(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	u := srv.URL + "/oauth/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"client"},
		"redirect_uri":          {"http://localhost/cb"},
		"code_challenge":        {"abc"},
		"code_challenge_method": {"plain"},
	}.Encode()

	resp := doGet(t, u)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Authorize POST ---

// startAuthFlow does the GET /oauth/authorize and returns the request_id from
// the hidden form field by parsing the HTML naively.
func startAuthFlow(t *testing.T, srv *httptest.Server, verifier string) (requestID, redirectURI string) {
	t.Helper()
	redirectURI = "http://localhost:3000/cb"
	u := srv.URL + "/oauth/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"https://claude.ai/client"},
		"redirect_uri":          {redirectURI},
		"state":                 {"teststate"},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}.Encode()

	resp := doGet(t, u)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorize GET status = %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)

	// Extract request_id from value="..."
	const needle = `name="request_id" value="`
	idx := strings.Index(body, needle)
	if idx == -1 {
		t.Fatalf("request_id not found in HTML")
	}
	start := idx + len(needle)
	end := strings.Index(body[start:], `"`)
	return body[start : start+end], redirectURI
}

func TestOAuthAuthorizeSubmit_Approve(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	requestID, redirectURI := startAuthFlow(t, srv, verifier)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // don't follow redirect
	}}

	form := url.Values{
		"request_id": {requestID},
		"action":     {"approve"},
		"auth_token": {testAccessToken},
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, redirectURI) {
		t.Errorf("Location = %q, want prefix %q", loc, redirectURI)
	}
	if !strings.Contains(loc, "code=") {
		t.Errorf("Location %q missing code param", loc)
	}
	if !strings.Contains(loc, "state=teststate") {
		t.Errorf("Location %q missing state", loc)
	}
}

func TestOAuthAuthorizeSubmit_Deny(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	requestID, _ := startAuthFlow(t, srv, verifier)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	form := url.Values{
		"request_id": {requestID},
		"action":     {"deny"},
		"auth_token": {""},
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Location"), "error=access_denied") {
		t.Errorf("Location %q missing error=access_denied", resp.Header.Get("Location"))
	}
}

func TestOAuthAuthorizeSubmit_WrongToken(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	requestID, _ := startAuthFlow(t, srv, verifier)

	resp := doPostForm(t, srv.URL+"/oauth/authorize", url.Values{
		"request_id": {requestID},
		"action":     {"approve"},
		"auth_token": {"wrong-token"},
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// --- Token: authorization_code ---

// fullAuthCodeFlow performs GET authorize + POST approve and returns the code.
func fullAuthCodeFlow(t *testing.T, srv *httptest.Server, verifier string) string {
	t.Helper()
	requestID, _ := startAuthFlow(t, srv, verifier)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	form := url.Values{
		"request_id": {requestID},
		"action":     {"approve"},
		"auth_token": {testAccessToken},
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	loc := resp.Header.Get("Location")
	parsed, _ := url.Parse(loc)
	return parsed.Query().Get("code")
}

func TestOAuthTokenAuthorizationCode_Success(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	code := fullAuthCodeFlow(t, srv, verifier)
	if code == "" {
		t.Fatal("no code returned from authorize flow")
	}

	resp := doPostForm(t, srv.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:3000/cb"},
		"code_verifier": {verifier},
	})
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
}

func TestOAuthTokenAuthorizationCode_SingleUse(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	code := fullAuthCodeFlow(t, srv, verifier)

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:3000/cb"},
		"code_verifier": {verifier},
	}
	r1 := doPostForm(t, srv.URL+"/oauth/token", tokenForm)
	_ = r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("first use: status = %d, want 200", r1.StatusCode)
	}

	r2 := doPostForm(t, srv.URL+"/oauth/token", tokenForm)
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusBadRequest {
		t.Fatalf("second use: status = %d, want 400", r2.StatusCode)
	}
}

func TestOAuthTokenAuthorizationCode_PKCEMismatch(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	code := fullAuthCodeFlow(t, srv, verifier)

	resp := doPostForm(t, srv.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:3000/cb"},
		"code_verifier": {"wrong-verifier-that-does-not-match-challenge"},
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestOAuthTokenAuthorizationCode_RedirectMismatch(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	verifier := "testverifier1234567890123456789012345678901234"
	code := fullAuthCodeFlow(t, srv, verifier)

	resp := doPostForm(t, srv.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:9999/different"},
		"code_verifier": {verifier},
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Token: client_credentials ---

func TestOAuthTokenClientCredentials_Post(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	resp := doPostForm(t, srv.URL+"/oauth/token", url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"any-client-id"},
		"client_secret": {testClientSecret},
	})
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
}

func TestOAuthTokenClientCredentials_Basic(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	form := url.Values{"grant_type": {"client_credentials"}}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("any-client", testClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestOAuthTokenClientCredentials_InvalidSecret(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	resp := doPostForm(t, srv.URL+"/oauth/token", url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"client"},
		"client_secret": {"wrong"},
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestOAuthTokenUnsupportedGrantType(t *testing.T) {
	_, srv := newTestOAuthServer()
	defer srv.Close()

	resp := doPostForm(t, srv.URL+"/oauth/token", url.Values{
		"grant_type": {"password"},
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
