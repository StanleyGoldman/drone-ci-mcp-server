// Package mcptools registers Drone CI tools on an MCP server.
package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stanleygoldman/drone-ci-mcp-server/internal/drone"
)

// Register adds all Drone CI tools to the given MCP server.
func Register(server *mcp.Server, client drone.Client) {
	h := &handler{client: client}

	server.AddTool(&mcp.Tool{
		Name:        "list_repos",
		Description: "List all repositories with Drone CI enabled for the authenticated user",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, h.listRepos)

	server.AddTool(&mcp.Tool{
		Name:        "get_repo",
		Description: "Get details for a specific repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"}
			}
		}`),
	}, h.getRepo)

	server.AddTool(&mcp.Tool{
		Name:        "list_builds",
		Description: "List recent builds for a repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"}
			}
		}`),
	}, h.listBuilds)

	server.AddTool(&mcp.Tool{
		Name:        "get_build",
		Description: "Get details for a specific build, including its stages and steps",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"},
				"build": {"type": "integer", "description": "Build number"}
			}
		}`),
	}, h.getBuild)

	server.AddTool(&mcp.Tool{
		Name:        "get_build_logs",
		Description: "Get the log output for a specific step in a build",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build", "stage", "step"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"},
				"build": {"type": "integer", "description": "Build number"},
				"stage": {"type": "integer", "description": "Stage number (1-indexed)"},
				"step":  {"type": "integer", "description": "Step number (1-indexed)"}
			}
		}`),
	}, h.getBuildLogs)

	server.AddTool(&mcp.Tool{
		Name:        "trigger_build",
		Description: "Trigger a new build for a branch",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name"],
			"properties": {
				"owner":  {"type": "string", "description": "Repository owner or namespace"},
				"name":   {"type": "string", "description": "Repository name"},
				"branch": {"type": "string", "description": "Branch to build (defaults to the repository default branch)"}
			}
		}`),
	}, h.triggerBuild)

	server.AddTool(&mcp.Tool{
		Name:        "cancel_build",
		Description: "Cancel a running build",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"},
				"build": {"type": "integer", "description": "Build number to cancel"}
			}
		}`),
	}, h.cancelBuild)

	server.AddTool(&mcp.Tool{
		Name:        "restart_build",
		Description: "Restart an existing build from the same commit",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"},
				"build": {"type": "integer", "description": "Build number to restart"}
			}
		}`),
	}, h.restartBuild)

	server.AddTool(&mcp.Tool{
		Name:        "promote_build",
		Description: "Promote a build to a target environment (e.g. staging, production)",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build", "target"],
			"properties": {
				"owner":  {"type": "string", "description": "Repository owner or namespace"},
				"name":   {"type": "string", "description": "Repository name"},
				"build":  {"type": "integer", "description": "Build number to promote"},
				"target": {"type": "string", "description": "Target environment name (e.g. production, staging)"}
			}
		}`),
	}, h.promoteBuild)

	server.AddTool(&mcp.Tool{
		Name:        "rollback_build",
		Description: "Rollback a build to a target environment",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build", "target"],
			"properties": {
				"owner":  {"type": "string", "description": "Repository owner or namespace"},
				"name":   {"type": "string", "description": "Repository name"},
				"build":  {"type": "integer", "description": "Build number to roll back"},
				"target": {"type": "string", "description": "Target environment name"}
			}
		}`),
	}, h.rollbackBuild)

	server.AddTool(&mcp.Tool{
		Name:        "approve_build",
		Description: "Approve a blocked stage in a build",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build", "stage"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"},
				"build": {"type": "integer", "description": "Build number"},
				"stage": {"type": "integer", "description": "Stage number to approve"}
			}
		}`),
	}, h.approveBuild)

	server.AddTool(&mcp.Tool{
		Name:        "decline_build",
		Description: "Decline a blocked stage in a build",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "build", "stage"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"},
				"build": {"type": "integer", "description": "Build number"},
				"stage": {"type": "integer", "description": "Stage number to decline"}
			}
		}`),
	}, h.declineBuild)

	server.AddTool(&mcp.Tool{
		Name:        "list_secrets",
		Description: "List all secrets for a repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name"],
			"properties": {
				"owner": {"type": "string", "description": "Repository owner or namespace"},
				"name":  {"type": "string", "description": "Repository name"}
			}
		}`),
	}, h.listSecrets)

	server.AddTool(&mcp.Tool{
		Name:        "get_secret",
		Description: "Get a secret by name for a repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "secret"],
			"properties": {
				"owner":  {"type": "string", "description": "Repository owner or namespace"},
				"name":   {"type": "string", "description": "Repository name"},
				"secret": {"type": "string", "description": "Secret name"}
			}
		}`),
	}, h.getSecret)

	server.AddTool(&mcp.Tool{
		Name:        "create_secret",
		Description: "Create a new secret for a repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "secret", "data"],
			"properties": {
				"owner":             {"type": "string", "description": "Repository owner or namespace"},
				"name":              {"type": "string", "description": "Repository name"},
				"secret":            {"type": "string", "description": "Secret name"},
				"data":              {"type": "string", "description": "Secret value"},
				"pull_request":      {"type": "boolean", "description": "Expose secret to pull request builds"},
				"pull_request_push": {"type": "boolean", "description": "Expose secret to pull request push builds"}
			}
		}`),
	}, h.createSecret)

	server.AddTool(&mcp.Tool{
		Name:        "update_secret",
		Description: "Update an existing secret for a repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "secret", "data"],
			"properties": {
				"owner":             {"type": "string", "description": "Repository owner or namespace"},
				"name":              {"type": "string", "description": "Repository name"},
				"secret":            {"type": "string", "description": "Secret name"},
				"data":              {"type": "string", "description": "New secret value"},
				"pull_request":      {"type": "boolean", "description": "Expose secret to pull request builds"},
				"pull_request_push": {"type": "boolean", "description": "Expose secret to pull request push builds"}
			}
		}`),
	}, h.updateSecret)

	server.AddTool(&mcp.Tool{
		Name:        "delete_secret",
		Description: "Delete a secret from a repository",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["owner", "name", "secret"],
			"properties": {
				"owner":  {"type": "string", "description": "Repository owner or namespace"},
				"name":   {"type": "string", "description": "Repository name"},
				"secret": {"type": "string", "description": "Secret name to delete"}
			}
		}`),
	}, h.deleteSecret)

	server.AddTool(&mcp.Tool{
		Name:        "list_org_secrets",
		Description: "List all secrets for an organization namespace",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["namespace"],
			"properties": {
				"namespace": {"type": "string", "description": "Organization namespace"}
			}
		}`),
	}, h.listOrgSecrets)

	server.AddTool(&mcp.Tool{
		Name:        "get_org_secret",
		Description: "Get a secret by name for an organization namespace",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["namespace", "secret"],
			"properties": {
				"namespace": {"type": "string", "description": "Organization namespace"},
				"secret":    {"type": "string", "description": "Secret name"}
			}
		}`),
	}, h.getOrgSecret)

	server.AddTool(&mcp.Tool{
		Name:        "create_org_secret",
		Description: "Create a new secret for an organization namespace",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["namespace", "secret", "data"],
			"properties": {
				"namespace":         {"type": "string", "description": "Organization namespace"},
				"secret":            {"type": "string", "description": "Secret name"},
				"data":              {"type": "string", "description": "Secret value"},
				"pull_request":      {"type": "boolean", "description": "Expose secret to pull request builds"},
				"pull_request_push": {"type": "boolean", "description": "Expose secret to pull request push builds"}
			}
		}`),
	}, h.createOrgSecret)

	server.AddTool(&mcp.Tool{
		Name:        "update_org_secret",
		Description: "Update an existing secret for an organization namespace",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["namespace", "secret", "data"],
			"properties": {
				"namespace":         {"type": "string", "description": "Organization namespace"},
				"secret":            {"type": "string", "description": "Secret name"},
				"data":              {"type": "string", "description": "New secret value"},
				"pull_request":      {"type": "boolean", "description": "Expose secret to pull request builds"},
				"pull_request_push": {"type": "boolean", "description": "Expose secret to pull request push builds"}
			}
		}`),
	}, h.updateOrgSecret)

	server.AddTool(&mcp.Tool{
		Name:        "delete_org_secret",
		Description: "Delete a secret from an organization namespace",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["namespace", "secret"],
			"properties": {
				"namespace": {"type": "string", "description": "Organization namespace"},
				"secret":    {"type": "string", "description": "Secret name to delete"}
			}
		}`),
	}, h.deleteOrgSecret)
}

type handler struct {
	client drone.Client
}

// args is a helper for parsing tool call arguments.
type args map[string]json.RawMessage

func parseArgs(raw json.RawMessage) (args, error) {
	var a args
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("parsing arguments: %w", err)
	}
	return a, nil
}

func (a args) string(key string) (string, error) {
	v, ok := a[key]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return "", fmt.Errorf("argument %q: %w", key, err)
	}
	return s, nil
}

func (a args) optString(key string) string {
	v, ok := a[key]
	if !ok {
		return ""
	}
	var s string
	json.Unmarshal(v, &s) //nolint:errcheck
	return s
}

func (a args) optBool(key string) bool {
	v, ok := a[key]
	if !ok {
		return false
	}
	var b bool
	json.Unmarshal(v, &b) //nolint:errcheck
	return b
}

func (a args) integer(key string) (int, error) {
	v, ok := a[key]
	if !ok {
		return 0, fmt.Errorf("missing required argument %q", key)
	}
	var n int
	if err := json.Unmarshal(v, &n); err != nil {
		return 0, fmt.Errorf("argument %q: %w", key, err)
	}
	return n, nil
}

func textResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func errResult(err error) *mcp.CallToolResult {
	res := &mcp.CallToolResult{}
	res.SetError(err)
	return res
}

// --- Tool handlers ---

func (h *handler) listRepos(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repos, err := h.client.ListRepos(ctx)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(repos)
}

func (h *handler) getRepo(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}

	repo, err := h.client.GetRepo(ctx, owner, name)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(repo)
}

func (h *handler) listBuilds(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}

	builds, err := h.client.ListBuilds(ctx, owner, name)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(builds)
}

func (h *handler) getBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}

	build, err := h.client.GetBuild(ctx, owner, name, buildNum)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) getBuildLogs(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}
	stageNum, err := a.integer("stage")
	if err != nil {
		return errResult(err), nil
	}
	stepNum, err := a.integer("step")
	if err != nil {
		return errResult(err), nil
	}

	lines, err := h.client.GetBuildLogs(ctx, owner, name, buildNum, stageNum, stepNum)
	if err != nil {
		return errResult(err), nil
	}

	// Format logs as plain text for readability.
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(line.Out)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil
}

func (h *handler) triggerBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	branch := a.optString("branch")

	build, err := h.client.TriggerBuild(ctx, owner, name, branch)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) cancelBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}

	if err := h.client.CancelBuild(ctx, owner, name, buildNum); err != nil {
		return errResult(err), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Build %d for %s/%s has been cancelled.", buildNum, owner, name),
		}},
	}, nil
}

func (h *handler) restartBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}

	build, err := h.client.RestartBuild(ctx, owner, name, buildNum)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) promoteBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}
	target, err := a.string("target")
	if err != nil {
		return errResult(err), nil
	}

	build, err := h.client.PromoteBuild(ctx, owner, name, buildNum, target)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) rollbackBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}
	target, err := a.string("target")
	if err != nil {
		return errResult(err), nil
	}

	build, err := h.client.RollbackBuild(ctx, owner, name, buildNum, target)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) approveBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}
	stageNum, err := a.integer("stage")
	if err != nil {
		return errResult(err), nil
	}

	build, err := h.client.ApproveBuild(ctx, owner, name, buildNum, stageNum)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) declineBuild(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	buildNum, err := a.integer("build")
	if err != nil {
		return errResult(err), nil
	}
	stageNum, err := a.integer("stage")
	if err != nil {
		return errResult(err), nil
	}

	build, err := h.client.DeclineBuild(ctx, owner, name, buildNum, stageNum)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(build)
}

func (h *handler) listSecrets(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}

	secrets, err := h.client.ListSecrets(ctx, owner, name)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(secrets)
}

func (h *handler) getSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	secret, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}

	s, err := h.client.GetSecret(ctx, owner, name, secret)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(s)
}

func (h *handler) createSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}
	data, err := a.string("data")
	if err != nil {
		return errResult(err), nil
	}

	s, err := h.client.CreateSecret(ctx, owner, name, drone.SecretInput{
		Name:            secretName,
		Data:            data,
		PullRequest:     a.optBool("pull_request"),
		PullRequestPush: a.optBool("pull_request_push"),
	})
	if err != nil {
		return errResult(err), nil
	}
	return textResult(s)
}

func (h *handler) updateSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}
	data, err := a.string("data")
	if err != nil {
		return errResult(err), nil
	}

	s, err := h.client.UpdateSecret(ctx, owner, name, secretName, drone.SecretInput{
		Data:            data,
		PullRequest:     a.optBool("pull_request"),
		PullRequestPush: a.optBool("pull_request_push"),
	})
	if err != nil {
		return errResult(err), nil
	}
	return textResult(s)
}

func (h *handler) deleteSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	owner, err := a.string("owner")
	if err != nil {
		return errResult(err), nil
	}
	name, err := a.string("name")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}

	if err := h.client.DeleteSecret(ctx, owner, name, secretName); err != nil {
		return errResult(err), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Secret %q deleted from %s/%s.", secretName, owner, name),
		}},
	}, nil
}

func (h *handler) listOrgSecrets(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	namespace, err := a.string("namespace")
	if err != nil {
		return errResult(err), nil
	}

	secrets, err := h.client.ListOrgSecrets(ctx, namespace)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(secrets)
}

func (h *handler) getOrgSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	namespace, err := a.string("namespace")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}

	s, err := h.client.GetOrgSecret(ctx, namespace, secretName)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(s)
}

func (h *handler) createOrgSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	namespace, err := a.string("namespace")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}
	data, err := a.string("data")
	if err != nil {
		return errResult(err), nil
	}

	s, err := h.client.CreateOrgSecret(ctx, namespace, drone.SecretInput{
		Name:            secretName,
		Data:            data,
		PullRequest:     a.optBool("pull_request"),
		PullRequestPush: a.optBool("pull_request_push"),
	})
	if err != nil {
		return errResult(err), nil
	}
	return textResult(s)
}

func (h *handler) updateOrgSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	namespace, err := a.string("namespace")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}
	data, err := a.string("data")
	if err != nil {
		return errResult(err), nil
	}

	s, err := h.client.UpdateOrgSecret(ctx, namespace, secretName, drone.SecretInput{
		Data:            data,
		PullRequest:     a.optBool("pull_request"),
		PullRequestPush: a.optBool("pull_request_push"),
	})
	if err != nil {
		return errResult(err), nil
	}
	return textResult(s)
}

func (h *handler) deleteOrgSecret(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := parseArgs(req.Params.Arguments)
	if err != nil {
		return errResult(err), nil
	}
	namespace, err := a.string("namespace")
	if err != nil {
		return errResult(err), nil
	}
	secretName, err := a.string("secret")
	if err != nil {
		return errResult(err), nil
	}

	if err := h.client.DeleteOrgSecret(ctx, namespace, secretName); err != nil {
		return errResult(err), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Secret %q deleted from namespace %q.", secretName, namespace),
		}},
	}, nil
}
