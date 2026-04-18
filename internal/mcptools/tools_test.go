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
	listReposFn       func(ctx context.Context) ([]drone.Repo, error)
	getRepoFn         func(ctx context.Context, owner, name string) (*drone.Repo, error)
	listBuildsFn      func(ctx context.Context, owner, name string) ([]drone.Build, error)
	getBuildFn        func(ctx context.Context, owner, name string, number int) (*drone.Build, error)
	getBuildLogsFn    func(ctx context.Context, owner, name string, buildNumber, stage, step int) ([]drone.LogLine, error)
	triggerBuildFn    func(ctx context.Context, owner, name, branch string) (*drone.Build, error)
	cancelBuildFn     func(ctx context.Context, owner, name string, number int) error
	restartBuildFn    func(ctx context.Context, owner, name string, number int) (*drone.Build, error)
	promoteBuildFn    func(ctx context.Context, owner, name string, number int, target string) (*drone.Build, error)
	rollbackBuildFn   func(ctx context.Context, owner, name string, number int, target string) (*drone.Build, error)
	approveBuildFn    func(ctx context.Context, owner, name string, number, stage int) (*drone.Build, error)
	declineBuildFn    func(ctx context.Context, owner, name string, number, stage int) (*drone.Build, error)
	listSecretsFn     func(ctx context.Context, owner, name string) ([]drone.Secret, error)
	getSecretFn       func(ctx context.Context, owner, name, secret string) (*drone.Secret, error)
	createSecretFn    func(ctx context.Context, owner, name string, input drone.SecretInput) (*drone.Secret, error)
	updateSecretFn    func(ctx context.Context, owner, name, secret string, input drone.SecretInput) (*drone.Secret, error)
	deleteSecretFn    func(ctx context.Context, owner, name, secret string) error
	listOrgSecretsFn  func(ctx context.Context, namespace string) ([]drone.Secret, error)
	getOrgSecretFn    func(ctx context.Context, namespace, name string) (*drone.Secret, error)
	createOrgSecretFn func(ctx context.Context, namespace string, input drone.SecretInput) (*drone.Secret, error)
	updateOrgSecretFn func(ctx context.Context, namespace, name string, input drone.SecretInput) (*drone.Secret, error)
	deleteOrgSecretFn func(ctx context.Context, namespace, name string) error
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

func (m *mockClient) PromoteBuild(ctx context.Context, owner, name string, number int, target string) (*drone.Build, error) {
	return m.promoteBuildFn(ctx, owner, name, number, target)
}

func (m *mockClient) RollbackBuild(ctx context.Context, owner, name string, number int, target string) (*drone.Build, error) {
	return m.rollbackBuildFn(ctx, owner, name, number, target)
}

func (m *mockClient) ApproveBuild(ctx context.Context, owner, name string, number, stage int) (*drone.Build, error) {
	return m.approveBuildFn(ctx, owner, name, number, stage)
}

func (m *mockClient) DeclineBuild(ctx context.Context, owner, name string, number, stage int) (*drone.Build, error) {
	return m.declineBuildFn(ctx, owner, name, number, stage)
}

func (m *mockClient) ListSecrets(ctx context.Context, owner, name string) ([]drone.Secret, error) {
	return m.listSecretsFn(ctx, owner, name)
}

func (m *mockClient) GetSecret(ctx context.Context, owner, name, secret string) (*drone.Secret, error) {
	return m.getSecretFn(ctx, owner, name, secret)
}

func (m *mockClient) CreateSecret(ctx context.Context, owner, name string, input drone.SecretInput) (*drone.Secret, error) {
	return m.createSecretFn(ctx, owner, name, input)
}

func (m *mockClient) UpdateSecret(ctx context.Context, owner, name, secret string, input drone.SecretInput) (*drone.Secret, error) {
	return m.updateSecretFn(ctx, owner, name, secret, input)
}

func (m *mockClient) DeleteSecret(ctx context.Context, owner, name, secret string) error {
	return m.deleteSecretFn(ctx, owner, name, secret)
}

func (m *mockClient) ListOrgSecrets(ctx context.Context, namespace string) ([]drone.Secret, error) {
	return m.listOrgSecretsFn(ctx, namespace)
}

func (m *mockClient) GetOrgSecret(ctx context.Context, namespace, name string) (*drone.Secret, error) {
	return m.getOrgSecretFn(ctx, namespace, name)
}

func (m *mockClient) CreateOrgSecret(ctx context.Context, namespace string, input drone.SecretInput) (*drone.Secret, error) {
	return m.createOrgSecretFn(ctx, namespace, input)
}

func (m *mockClient) UpdateOrgSecret(ctx context.Context, namespace, name string, input drone.SecretInput) (*drone.Secret, error) {
	return m.updateOrgSecretFn(ctx, namespace, name, input)
}

func (m *mockClient) DeleteOrgSecret(ctx context.Context, namespace, name string) error {
	return m.deleteOrgSecretFn(ctx, namespace, name)
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
	t.Cleanup(func() { _ = session.Close() })
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

func TestPromoteBuild_Success(t *testing.T) {
	mock := &mockClient{
		promoteBuildFn: func(_ context.Context, owner, name string, number int, target string) (*drone.Build, error) {
			if target != "production" {
				return nil, errors.New("unexpected target: " + target)
			}
			return &drone.Build{Number: number + 1, Status: "pending"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "promote_build", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"build":  5,
		"target": "production",
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

func TestRollbackBuild_Success(t *testing.T) {
	mock := &mockClient{
		rollbackBuildFn: func(_ context.Context, owner, name string, number int, target string) (*drone.Build, error) {
			return &drone.Build{Number: number + 1, Status: "pending"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "rollback_build", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"build":  3,
		"target": "staging",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestApproveBuild_Success(t *testing.T) {
	mock := &mockClient{
		approveBuildFn: func(_ context.Context, owner, name string, number, stage int) (*drone.Build, error) {
			if stage != 2 {
				return nil, errors.New("unexpected stage")
			}
			return &drone.Build{Number: number, Status: "running"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "approve_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 5,
		"stage": 2,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var build drone.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if build.Status != "running" {
		t.Errorf("want Status=running, got %q", build.Status)
	}
}

func TestDeclineBuild_Success(t *testing.T) {
	mock := &mockClient{
		declineBuildFn: func(_ context.Context, owner, name string, number, stage int) (*drone.Build, error) {
			return &drone.Build{Number: number, Status: "declined"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "decline_build", map[string]any{
		"owner": "acme",
		"name":  "widget",
		"build": 5,
		"stage": 1,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestListSecrets_Success(t *testing.T) {
	mock := &mockClient{
		listSecretsFn: func(_ context.Context, owner, name string) ([]drone.Secret, error) {
			return []drone.Secret{
				{ID: 1, Name: "docker_password"},
				{ID: 2, Name: "npm_token"},
			}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "list_secrets", map[string]any{
		"owner": "acme",
		"name":  "widget",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var secrets []drone.Secret
	if err := json.Unmarshal([]byte(resultText(t, result)), &secrets); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if len(secrets) != 2 {
		t.Errorf("want 2 secrets, got %d", len(secrets))
	}
}

func TestGetSecret_Success(t *testing.T) {
	mock := &mockClient{
		getSecretFn: func(_ context.Context, owner, name, secret string) (*drone.Secret, error) {
			if secret != "docker_password" {
				return nil, errors.New("unexpected secret: " + secret)
			}
			return &drone.Secret{ID: 1, Name: "docker_password"}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "get_secret", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"secret": "docker_password",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var s drone.Secret
	if err := json.Unmarshal([]byte(resultText(t, result)), &s); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if s.Name != "docker_password" {
		t.Errorf("want Name=docker_password, got %q", s.Name)
	}
}

func TestCreateSecret_Success(t *testing.T) {
	mock := &mockClient{
		createSecretFn: func(_ context.Context, owner, name string, input drone.SecretInput) (*drone.Secret, error) {
			return &drone.Secret{ID: 3, Name: input.Name, PullRequest: input.PullRequest}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "create_secret", map[string]any{
		"owner":        "acme",
		"name":         "widget",
		"secret":       "new_secret",
		"data":         "s3cr3t",
		"pull_request": true,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var s drone.Secret
	if err := json.Unmarshal([]byte(resultText(t, result)), &s); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if s.Name != "new_secret" {
		t.Errorf("want Name=new_secret, got %q", s.Name)
	}
	if !s.PullRequest {
		t.Error("want PullRequest=true")
	}
}

func TestUpdateSecret_Success(t *testing.T) {
	mock := &mockClient{
		updateSecretFn: func(_ context.Context, owner, name, secret string, input drone.SecretInput) (*drone.Secret, error) {
			return &drone.Secret{ID: 1, Name: secret}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "update_secret", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"secret": "docker_password",
		"data":   "newval",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestDeleteSecret_Success(t *testing.T) {
	mock := &mockClient{
		deleteSecretFn: func(_ context.Context, owner, name, secret string) error {
			if secret != "docker_password" {
				return errors.New("unexpected secret: " + secret)
			}
			return nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "delete_secret", map[string]any{
		"owner":  "acme",
		"name":   "widget",
		"secret": "docker_password",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	if text := resultText(t, result); text == "" {
		t.Error("expected non-empty confirmation message")
	}
}

func TestListOrgSecrets_Success(t *testing.T) {
	mock := &mockClient{
		listOrgSecretsFn: func(_ context.Context, namespace string) ([]drone.Secret, error) {
			return []drone.Secret{{ID: 10, Namespace: namespace, Name: "shared_key"}}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "list_org_secrets", map[string]any{
		"namespace": "acme",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var secrets []drone.Secret
	if err := json.Unmarshal([]byte(resultText(t, result)), &secrets); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if len(secrets) != 1 || secrets[0].Name != "shared_key" {
		t.Errorf("unexpected secrets: %+v", secrets)
	}
}

func TestGetOrgSecret_Success(t *testing.T) {
	mock := &mockClient{
		getOrgSecretFn: func(_ context.Context, namespace, name string) (*drone.Secret, error) {
			return &drone.Secret{ID: 10, Namespace: namespace, Name: name}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "get_org_secret", map[string]any{
		"namespace": "acme",
		"secret":    "shared_key",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var s drone.Secret
	if err := json.Unmarshal([]byte(resultText(t, result)), &s); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if s.Name != "shared_key" {
		t.Errorf("want Name=shared_key, got %q", s.Name)
	}
}

func TestCreateOrgSecret_Success(t *testing.T) {
	mock := &mockClient{
		createOrgSecretFn: func(_ context.Context, namespace string, input drone.SecretInput) (*drone.Secret, error) {
			return &drone.Secret{ID: 11, Namespace: namespace, Name: input.Name}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "create_org_secret", map[string]any{
		"namespace": "acme",
		"secret":    "org_secret",
		"data":      "val",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var s drone.Secret
	if err := json.Unmarshal([]byte(resultText(t, result)), &s); err != nil {
		t.Fatalf("parsing result JSON: %v", err)
	}
	if s.Name != "org_secret" {
		t.Errorf("want Name=org_secret, got %q", s.Name)
	}
}

func TestUpdateOrgSecret_Success(t *testing.T) {
	mock := &mockClient{
		updateOrgSecretFn: func(_ context.Context, namespace, name string, input drone.SecretInput) (*drone.Secret, error) {
			return &drone.Secret{ID: 10, Namespace: namespace, Name: name}, nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "update_org_secret", map[string]any{
		"namespace": "acme",
		"secret":    "shared_key",
		"data":      "newval",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestDeleteOrgSecret_Success(t *testing.T) {
	mock := &mockClient{
		deleteOrgSecretFn: func(_ context.Context, namespace, name string) error {
			return nil
		},
	}
	session := connect(t, mock)
	result := callTool(t, session, "delete_org_secret", map[string]any{
		"namespace": "acme",
		"secret":    "shared_key",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	if text := resultText(t, result); text == "" {
		t.Error("expected non-empty confirmation message")
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
