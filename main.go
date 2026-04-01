package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

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
		fmt.Fprint(w, "ok")
	})
	mux.Handle("/", authMiddleware(cfg.AuthToken, mcpHandler))

	addr := ":" + cfg.Port
	logger.Info("drone-ci MCP server starting",
		"addr", addr,
		"drone_server", cfg.DroneServer,
		"auth_enabled", cfg.AuthToken != "",
	)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	DroneServer string
	DroneToken  string
	Port        string
	AuthToken   string
}

func configFromEnv() config {
	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = "8080"
	}
	return config{
		DroneServer: os.Getenv("DRONE_SERVER"),
		DroneToken:  os.Getenv("DRONE_TOKEN"),
		Port:        port,
		AuthToken:   os.Getenv("MCP_AUTH_TOKEN"),
	}
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
