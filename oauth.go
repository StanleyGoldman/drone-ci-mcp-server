package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"sync"
	"time"
)

// oauthServer implements a minimal OAuth 2.1 authorization server supporting:
//   - Authorization Code flow with mandatory PKCE (for claude.ai)
//   - Client Credentials flow (for direct API access)
//
// All state is in-memory; a server restart invalidates pending sessions and
// issued codes, requiring users to re-authorize.
type oauthServer struct {
	clientSecret string // shared secret for client_credentials grant
	accessToken  string // MCP_AUTH_TOKEN — issued as the access token

	mu      sync.Mutex
	pending map[string]pendingAuth // requestID → consent session
	codes   map[string]issuedCode  // auth code → PKCE/redirect binding
}

// pendingAuth holds an in-progress authorization request while the user is on
// the consent page.
type pendingAuth struct {
	clientID    string
	redirectURI string
	challenge   string
	method      string
	state       string
	expiresAt   time.Time
}

// issuedCode holds the data bound to an authorization code until it is
// exchanged for a token (single-use, 5-minute TTL).
type issuedCode struct {
	clientID    string
	redirectURI string
	challenge   string
	expiresAt   time.Time
}

func newOAuthServer(clientSecret, accessToken string) *oauthServer {
	return &oauthServer{
		clientSecret: clientSecret,
		accessToken:  accessToken,
		pending:      make(map[string]pendingAuth),
		codes:        make(map[string]issuedCode),
	}
}

// registerRoutes attaches all OAuth endpoints to mux.
func (s *oauthServer) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.handleMetadata)
	mux.HandleFunc("GET /oauth/authorize", s.handleAuthorizeForm)
	mux.HandleFunc("POST /oauth/authorize", s.handleAuthorizeSubmit)
	mux.HandleFunc("POST /oauth/token", s.handleToken)
}

// handleMetadata serves RFC 8414 Authorization Server Metadata.
func (s *oauthServer) handleMetadata(w http.ResponseWriter, r *http.Request) {
	base := requestBase(r)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "client_credentials"},
		"code_challenge_methods_supported":       []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleAuthorizeForm validates the authorization request and renders a consent
// page for the server operator to approve or deny.
func (s *oauthServer) handleAuthorizeForm(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if q.Get("response_type") != "code" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_response_type", "only response_type=code is supported")
		return
	}
	if q.Get("code_challenge_method") != "S256" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code_challenge_method must be S256")
		return
	}

	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	challenge := q.Get("code_challenge")
	state := q.Get("state")

	if clientID == "" || redirectURI == "" || challenge == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id, redirect_uri, and code_challenge are required")
		return
	}

	requestID := randomHex(16)
	s.mu.Lock()
	s.pending[requestID] = pendingAuth{
		clientID:    clientID,
		redirectURI: redirectURI,
		challenge:   challenge,
		method:      "S256",
		state:       state,
		expiresAt:   time.Now().Add(10 * time.Minute),
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := consentTmpl.Execute(w, map[string]string{
		"RequestID": requestID,
		"ClientID":  clientID,
	}); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// handleAuthorizeSubmit processes the consent form submission.
func (s *oauthServer) handleAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID := r.FormValue("request_id")
	action := r.FormValue("action")
	authToken := r.FormValue("auth_token")

	s.mu.Lock()
	auth, ok := s.pending[requestID]
	if ok {
		delete(s.pending, requestID)
	}
	s.mu.Unlock()

	if !ok || time.Now().After(auth.expiresAt) {
		http.Error(w, "authorization session expired or not found", http.StatusBadRequest)
		return
	}

	redirectBase := auth.redirectURI
	if auth.state != "" {
		redirectBase += "?state=" + auth.state
	}
	sep := "?"
	if auth.state != "" {
		sep = "&"
	}

	if action == "deny" {
		http.Redirect(w, r, redirectBase+sep+"error=access_denied", http.StatusFound)
		return
	}

	if authToken != s.accessToken {
		http.Error(w, "Unauthorized: wrong auth token", http.StatusUnauthorized)
		return
	}

	code := randomHex(24)
	s.mu.Lock()
	s.codes[code] = issuedCode{
		clientID:    auth.clientID,
		redirectURI: auth.redirectURI,
		challenge:   auth.challenge,
		expiresAt:   time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()

	http.Redirect(w, r, redirectBase+sep+"code="+code, http.StatusFound)
}

// handleToken handles POST /oauth/token for both authorization_code and
// client_credentials grants.
func (s *oauthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad_request", http.StatusBadRequest)
		return
	}

	switch r.FormValue("grant_type") {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "client_credentials":
		s.handleClientCredentialsGrant(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "supported: authorization_code, client_credentials")
	}
}

func (s *oauthServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	verifier := r.FormValue("code_verifier")

	s.mu.Lock()
	stored, ok := s.codes[code]
	if ok {
		delete(s.codes, code) // single-use
	}
	s.mu.Unlock()

	if !ok || time.Now().After(stored.expiresAt) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code is invalid or expired")
		return
	}
	if redirectURI != stored.redirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}
	if !verifyPKCE(stored.challenge, verifier) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": s.accessToken,
		"token_type":   "Bearer",
	})
}

func (s *oauthServer) handleClientCredentialsGrant(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id = r.FormValue("client_id")
		secret = r.FormValue("client_secret")
	}
	// For client_credentials the client_id is not validated — only the secret.
	_ = id
	if secret != s.clientSecret {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": s.accessToken,
		"token_type":   "Bearer",
	})
}

// verifyPKCE checks that SHA256(verifier) matches the stored challenge.
func verifyPKCE(challenge, verifier string) bool {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:]) == challenge
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": description,
	})
}

// requestBase returns the scheme+host of the request, honouring the
// X-Forwarded-Proto header set by reverse proxies such as Caddy.
func requestBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// consentTmpl is the HTML consent page shown to the server operator.
var consentTmpl = template.Must(template.New("consent").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Authorize — Drone CI MCP Server</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 480px; margin: 80px auto; padding: 0 1rem; color: #1a1a1a; }
    h1   { font-size: 1.3rem; margin-bottom: 0.25rem; }
    .client { font-size: 0.85rem; color: #555; word-break: break-all; margin-bottom: 1.5rem; }
    label { display: block; margin-bottom: 0.4rem; font-size: 0.9rem; font-weight: 600; }
    input[type=password] { width: 100%; padding: 0.5rem; border: 1px solid #ccc; border-radius: 4px; font-size: 1rem; box-sizing: border-box; }
    .actions { display: flex; gap: 0.75rem; margin-top: 1rem; }
    button { flex: 1; padding: 0.6rem; border: none; border-radius: 4px; font-size: 1rem; cursor: pointer; }
    .approve { background: #2563eb; color: #fff; }
    .deny    { background: #f3f4f6; color: #374151; border: 1px solid #d1d5db; }
  </style>
</head>
<body>
  <h1>Authorize Drone CI MCP Server</h1>
  <p class="client">Requested by: <code>{{.ClientID}}</code></p>
  <form method="POST" action="/oauth/authorize">
    <input type="hidden" name="request_id" value="{{.RequestID}}">
    <label for="auth_token">MCP Auth Token</label>
    <input type="password" id="auth_token" name="auth_token" placeholder="Enter MCP_AUTH_TOKEN to approve" autocomplete="current-password">
    <div class="actions">
      <button type="submit" name="action" value="approve" class="approve">Approve</button>
      <button type="submit" name="action" value="deny"    class="deny">Deny</button>
    </div>
  </form>
</body>
</html>
`))
