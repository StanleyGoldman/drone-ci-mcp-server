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
