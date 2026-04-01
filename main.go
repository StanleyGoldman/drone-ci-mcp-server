package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/drone"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/mcptools"
)

func main() {
	cfg := configFromEnv()

	if cfg.DroneServer == "" {
		fmt.Fprintln(os.Stderr, "error: DRONE_SERVER is required")
		os.Exit(1)
	}
	if cfg.DroneToken == "" {
		fmt.Fprintln(os.Stderr, "error: DRONE_TOKEN is required")
		os.Exit(1)
	}

	oauthEnabled := cfg.OAuthClientID != "" && cfg.OAuthClientSecret != ""
	if oauthEnabled && cfg.AuthToken == "" {
		fmt.Fprintln(os.Stderr, "error: MCP_AUTH_TOKEN is required when OAUTH_CLIENT_ID and OAUTH_CLIENT_SECRET are set")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	droneClient := drone.NewHTTPClient(cfg.DroneServer, cfg.DroneToken)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "drone-ci",
		Version: "1.0.0",
	}, nil)
	mcptools.Register(server, droneClient)

	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	})
	mux.Handle("/", authMiddleware(cfg.AuthToken, mcpHandler))

	if oauthEnabled {
		registerOAuth(mux, cfg.OAuthClientID, cfg.OAuthClientSecret, cfg.AuthToken)
	}

	addr := ":" + cfg.Port
	logger.Info("drone-ci MCP server starting",
		"addr", addr,
		"drone_server", cfg.DroneServer,
		"auth_enabled", cfg.AuthToken != "",
		"oauth_enabled", oauthEnabled,
	)

	httpServer := &http.Server{Addr: addr, Handler: mux}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
		}
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	DroneServer       string
	DroneToken        string
	Port              string
	AuthToken         string
	OAuthClientID     string
	OAuthClientSecret string
}

func configFromEnv() config {
	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = "8080"
	}
	return config{
		DroneServer:       os.Getenv("DRONE_SERVER"),
		DroneToken:        envOrFile("DRONE_TOKEN"),
		Port:              port,
		AuthToken:         envOrFile("MCP_AUTH_TOKEN"),
		OAuthClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		OAuthClientSecret: envOrFile("OAUTH_CLIENT_SECRET"),
	}
}

// envOrFile reads a config value from the named env var, falling back to the
// file path in <key>_FILE (Docker secrets convention).
func envOrFile(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if path := os.Getenv(key + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

// authMiddleware enforces Bearer token authentication when a token is configured.
// When MCP_AUTH_TOKEN is empty the handler is passed through unchanged.
func authMiddleware(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	expected := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
