package mcptools_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/drone"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/mcptools"
)

// mockClient implements drone.Client for testing.
type mockClient struct {
	listReposFn    func(ctx context.Context) ([]drone.Repo, error)
	getRepoFn      func(ctx context.Context, owner, name string) (*drone.Repo, error)
	listBuildsFn   func(ctx context.Context, owner, name string) ([]drone.Build, error)
	getBuildFn     func(ctx context.Context, owner, name string, number int) (*drone.Build, error)
	getBuildLogsFn func(ctx context.Context, owner, name string, buildNumber, stage, step int) ([]drone.LogLine, error)
	triggerBuildFn func(ctx context.Context, owner, name, branch string) (*drone.Build, error)
	cancelBuildFn  func(ctx context.Context, owner, name string, number int) error
	restartBuildFn func(ctx context.Context, owner, name string, number int) (*drone.Build, error)
}

func (m *mockClient) ListRepos(ctx context.Context) ([]drone.Repo, error) {
	return m.listReposFn(ctx)
}

func (m *mockClient) GetRepo(ctx context.Context, owner, name string) (*drone.Repo, error) {
	return m.getRepoFn(ctx, owner, name)
}

func (m *mockClient) ListBuilds(ctx context.Context, owner, name string) ([]drone.Build, error) {
	return m.listBuildsFn(ctx, owner, name)
}

func (m *mockClient) GetBuild(ctx context.Context, owner, name string, number int) (*drone.Build, error) {
	return m.getBuildFn(ctx, owner, name, number)
}

func (m *mockClient) GetBuildLogs(ctx context.Context, owner, name string, buildNumber, stage, step int) ([]drone.LogLine, error) {
	return m.getBuildLogsFn(ctx, owner, name, buildNumber, stage, step)
}

func (m *mockClient) TriggerBuild(ctx context.Context, owner, name, branch string) (*drone.Build, error) {
	return m.triggerBuildFn(ctx, owner, name, branch)
}

func (m *mockClient) CancelBuild(ctx context.Context, owner, name string, number int) error {
	return m.cancelBuildFn(ctx, owner, name, number)
}

func (m *mockClient) RestartBuild(ctx context.Context, owner, name string, number int) (*drone.Build, error) {
	return m.restartBuildFn(ctx, owner, name, number)
}

// connect creates an in-memory MCP session with the given drone client registered.
func connect(t *testing.T, client drone.Client) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	mcptools.Register(server, client)

	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	mc := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	session, err := mc.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
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

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestListRepos_Success(t *testing.T) {
	mock := &mockClient{
		listReposFn: func(_ context.Context) ([]drone.Repo, error) {
			return []drone.Repo{
				{ID: 1, Namespace: "acme", Name: "widget"},
				{ID: 2, Namespace: "acme", Name: "gadget"},
			}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "list_repos", nil)

	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, result))
	}
	var repos []drone.Repo
	if err := json.Unmarshal([]byte(resultText(t, result)), &repos); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("want 2 repos, got %d", len(repos))
	}
}

func TestListRepos_Error(t *testing.T) {
	mock := &mockClient{
		listReposFn: func(_ context.Context) ([]drone.Repo, error) {
			return nil, errors.New("connection refused")
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "list_repos", nil)

	if !result.IsError {
		t.Fatal("expected IsError=true, got false")
	}
}

func TestGetRepo_Success(t *testing.T) {
	mock := &mockClient{
		getRepoFn: func(_ context.Context, owner, name string) (*drone.Repo, error) {
			if owner != "acme" || name != "widget" {
				return nil, errors.New("unexpected args")
			}
			return &drone.Repo{ID: 1, Namespace: "acme", Name: "widget", IsActive: true}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "get_repo", map[string]any{
		"owner": "acme",
		"name":  "widget",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var repo drone.Repo
	if err := json.Unmarshal([]byte(resultText(t, result)), &repo); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if !repo.IsActive {
		t.Error("want IsActive=true")
	}
}

func TestListBuilds_Success(t *testing.T) {
	mock := &mockClient{
		listBuildsFn: func(_ context.Context, owner, name string) ([]drone.Build, error) {
			return []drone.Build{
				{Number: 1, Status: "success"},
				{Number: 2, Status: "running"},
			}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "list_builds", map[string]any{
		"owner": "acme",
		"name":  "widget",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var builds []drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &builds); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if len(builds) != 2 {
		t.Errorf("want 2 builds, got %d", len(builds))
	}
}

func TestGetBuild_Success(t *testing.T) {
	mock := &mockClient{
		getBuildFn: func(_ context.Context, owner, name string, number int) (*drone.Build, error) {
			if number != 42 {
				return nil, errors.New("unexpected build number")
			}
			return &drone.Build{Number: 42, Status: "failure"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "get_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 42,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if build.Status != "failure" {
		t.Errorf("want Status=failure, got %q", build.Status)
	}
}

func TestGetBuildLogs_Success(t *testing.T) {
	mock := &mockClient{
		getBuildLogsFn: func(_ context.Context, owner, name string, buildNumber, stage, step int) ([]drone.LogLine, error) {
			return []drone.LogLine{
				{Number: 0, Out: "line one\n"},
				{Number: 1, Out: "line two\n"},
			}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "get_build_logs", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 1,
		"stage": 1,
		"step":  1,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if text != "line one\nline two\n" {
		t.Errorf("unexpected log text: %q", text)
	}
}

func TestTriggerBuild_WithBranch(t *testing.T) {
	mock := &mockClient{
		triggerBuildFn: func(_ context.Context, owner, name, branch string) (*drone.Build, error) {
			if branch != "feature/x" {
				return nil, errors.New("unexpected branch: " + branch)
			}
			return &drone.Build{Number: 10, Status: "pending"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "trigger_build", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"branch": "feature/x",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if build.Number != 10 {
		t.Errorf("want Number=10, got %d", build.Number)
	}
}

func TestTriggerBuild_WithoutBranch(t *testing.T) {
	mock := &mockClient{
		triggerBuildFn: func(_ context.Context, owner, name, branch string) (*drone.Build, error) {
			if branch != "" {
				return nil, errors.New("expected empty branch, got: " + branch)
			}
			return &drone.Build{Number: 11, Status: "pending"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "trigger_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestCancelBuild_Success(t *testing.T) {
	mock := &mockClient{
		cancelBuildFn: func(_ context.Context, owner, name string, number int) error {
			if number != 5 {
				return errors.New("unexpected build number")
			}
			return nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "cancel_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 5,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty confirmation message")
	}
}

func TestRestartBuild_Success(t *testing.T) {
	mock := &mockClient{
		restartBuildFn: func(_ context.Context, owner, name string, number int) (*drone.Build, error) {
			return &drone.Build{Number: 6, Status: "pending"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "restart_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 5,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if build.Number != 6 {
		t.Errorf("want Number=6, got %d", build.Number)
	}
}

func TestMissingRequiredArg(t *testing.T) {
	mock := &mockClient{
		getRepoFn: func(_ context.Context, owner, name string) (*drone.Repo, error) {
			return &drone.Repo{}, nil
		},
	}
	session := connect(t, mock)
	// Omit "name" which is required.
	result := callTool(t, session, "get_repo", map[string]any{
		"owner": "acme",
	})
	if !result.IsError {
		t.Fatal("expected IsError=true for missing required arg")
	}
}
