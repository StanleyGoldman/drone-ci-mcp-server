package main

import (
	"encoding/json"
	"net/http"
)

// registerOAuth registers the three OAuth 2.0 endpoints required by the MCP
// authorization spec so the server can be added to claude.ai.
//
// Endpoints:
//
//	GET  /.well-known/oauth-authorization-server  – RFC 8414 metadata
//	GET  /oauth/authorize                         – authorization endpoint (client_credentials only)
//	POST /oauth/token                             – token endpoint
func registerOAuth(mux *http.ServeMux, clientID, clientSecret, accessToken string) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", oauthMetadataHandler)
	mux.HandleFunc("GET /oauth/authorize", oauthAuthorizeHandler)
	mux.HandleFunc("POST /oauth/token", oauthTokenHandler(clientID, clientSecret, accessToken))
}

// oauthMetadataHandler serves RFC 8414 Authorization Server Metadata.
// The issuer and endpoint URLs are derived from the incoming request so the
// server works correctly whether accessed directly or behind a reverse proxy.
func oauthMetadataHandler(w http.ResponseWriter, r *http.Request) {
	base := requestBase(r)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"grant_types_supported":                 []string{"client_credentials"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// oauthAuthorizeHandler is present to satisfy the spec. The client_credentials
// grant does not use the authorization endpoint, so we return an informative error.
func oauthAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             "unsupported_response_type",
		"error_description": "only client_credentials grant is supported; use POST /oauth/token",
	})
}

// oauthTokenHandler returns an http.HandlerFunc that validates client credentials
// and issues the configured access token (client_credentials grant only).
func oauthTokenHandler(clientID, clientSecret, accessToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad_request", http.StatusBadRequest)
			return
		}

		if r.FormValue("grant_type") != "client_credentials" {
			writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "only client_credentials is supported")
			return
		}

		// Support both client_secret_basic (HTTP Basic Auth) and client_secret_post.
		id, secret, ok := r.BasicAuth()
		if !ok {
			id = r.FormValue("client_id")
			secret = r.FormValue("client_secret")
		}

		if id != clientID || secret != clientSecret {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": accessToken,
			"token_type":   "Bearer",
		})
	}
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
