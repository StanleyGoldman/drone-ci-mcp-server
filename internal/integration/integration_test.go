//go:build integration

// Package integration contains end-to-end tests that exercise the full HTTP
// stack: a mock Drone CI server, the real MCP server over HTTP, and a real MCP
// HTTP client. No Docker is required; everything runs in-process.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/drone"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/mcptools"
)

// newStack spins up a mock Drone CI server and a real MCP HTTP server wired to
// it. It returns an MCP client session and a cleanup function.
func newStack(t *testing.T, droneMux *http.ServeMux) *mcp.ClientSession {
	t.Helper()

	// 1. Start mock Drone CI HTTP server.
	droneSrv := httptest.NewServer(droneMux)
	t.Cleanup(droneSrv.Close)

	// 2. Create the real Drone client pointed at the mock server.
	droneClient := drone.NewHTTPClient(droneSrv.URL, "test-token")

	// 3. Create and configure the MCP server.
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "drone-ci", Version: "0.0.0"}, nil)
	mcptools.Register(mcpServer, droneClient)

	// 4. Start the MCP HTTP server.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	mcpSrv := httptest.NewServer(mcpHandler)
	t.Cleanup(mcpSrv.Close)

	// 5. Connect an MCP client to the MCP HTTP server.
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             mcpSrv.URL,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("mcp client.Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%q): %v", name, err)
	}
	return result
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", r.Content[0])
	}
	return tc.Text
}

func TestIntegration_ListRepos(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/user/repos", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.Repo{
			{ID: 1, Namespace: "acme", Name: "widget"},
		})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "list_repos", nil)
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var repos []drone.Repo
	if err := json.Unmarshal([]byte(resultText(t, result)), &repos); err != nil {
		t.Fatalf("parsing repos: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "widget" {
		t.Errorf("unexpected repos: %+v", repos)
	}
}

func TestIntegration_GetRepo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Repo{ID: 1, Namespace: "acme", Name: "widget", IsActive: true})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "get_repo", map[string]any{
		"owner": "acme",
		"name":  "widget",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var repo drone.Repo
	if err := json.Unmarshal([]byte(resultText(t, result)), &repo); err != nil {
		t.Fatalf("parsing repo: %v", err)
	}
	if !repo.IsActive {
		t.Error("want IsActive=true")
	}
}

func TestIntegration_ListBuilds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/builds", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.Build{
			{Number: 1, Status: "success"},
			{Number: 2, Status: "running"},
		})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "list_builds", map[string]any{
		"owner": "acme",
		"name":  "widget",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var builds []drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &builds); err != nil {
		t.Fatalf("parsing builds: %v", err)
	}
	if len(builds) != 2 {
		t.Errorf("want 2 builds, got %d", len(builds))
	}
	if builds[1].Status != "running" {
		t.Errorf("want builds[1].Status=running, got %q", builds[1].Status)
	}
}

func TestIntegration_GetBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/builds/7", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{
			Number: 7,
			Status: "failure",
			Stages: []drone.Stage{
				{Number: 1, Name: "default", Steps: []drone.Step{
					{Number: 1, Name: "clone", Status: "success"},
					{Number: 2, Name: "test", Status: "failure"},
				}},
			},
		})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "get_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 7,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing build: %v", err)
	}
	if build.Status != "failure" {
		t.Errorf("want Status=failure, got %q", build.Status)
	}
	if len(build.Stages) != 1 {
		t.Errorf("want 1 stage, got %d", len(build.Stages))
	}
}

func TestIntegration_GetBuildLogs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/builds/7/logs/1/2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.LogLine{
			{Number: 0, Out: "running tests\n"},
			{Number: 1, Out: "FAIL\n"},
		})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "get_build_logs", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 7,
		"stage": 1,
		"step":  2,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if text != "running tests\nFAIL\n" {
		t.Errorf("unexpected log output: %q", text)
	}
}

func TestIntegration_TriggerBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{Number: 99, Status: "pending"})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "trigger_build", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"branch": "main",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing build: %v", err)
	}
	if build.Number != 99 {
		t.Errorf("want Number=99, got %d", build.Number)
	}
}

func TestIntegration_CancelBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/repos/acme/widget/builds/3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	session := newStack(t, mux)

	result := callTool(t, session, "cancel_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 3,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestIntegration_RestartBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds/3", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{Number: 4, Status: "pending"})
	})
	session := newStack(t, mux)

	result := callTool(t, session, "restart_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 3,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing build: %v", err)
	}
	if build.Number != 4 {
		t.Errorf("want Number=4, got %d", build.Number)
	}
}

func TestIntegration_DroneAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	})
	session := newStack(t, mux)

	result := callTool(t, session, "get_repo", map[string]any{
		"owner": "acme",
		"name":  "missing",
	})
	if !result.IsError {
		t.Fatal("expected IsError=true for 404 from Drone")
	}
}
