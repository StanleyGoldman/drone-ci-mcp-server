package drone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is the interface for interacting with a Drone CI server.
type Client interface {
	ListRepos(ctx context.Context) ([]Repo, error)
	GetRepo(ctx context.Context, owner, name string) (*Repo, error)
	ListBuilds(ctx context.Context, owner, name string) ([]Build, error)
	GetBuild(ctx context.Context, owner, name string, number int) (*Build, error)
	GetBuildLogs(ctx context.Context, owner, name string, buildNumber, stage, step int) ([]LogLine, error)
	TriggerBuild(ctx context.Context, owner, name, branch string) (*Build, error)
	CancelBuild(ctx context.Context, owner, name string, number int) error
	RestartBuild(ctx context.Context, owner, name string, number int) (*Build, error)
}

// Repo represents a Drone CI repository.
type Repo struct {
	ID            int64  `json:"id"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	SCM           string `json:"scm"`
	GitHTTPURL    string `json:"git_http_url"`
	Link          string `json:"link"`
	DefaultBranch string `json:"default_branch"`
	IsPrivate     bool   `json:"private"`
	IsActive      bool   `json:"active"`
}

// Build represents a Drone CI build.
type Build struct {
	ID       int64   `json:"id"`
	RepoID   int64   `json:"repo_id"`
	Number   int     `json:"number"`
	Status   string  `json:"status"`
	Event    string  `json:"event"`
	Action   string  `json:"action"`
	Link     string  `json:"link"`
	Message  string  `json:"message"`
	Ref      string  `json:"ref"`
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Author   string  `json:"author_login"`
	Started  int64   `json:"started"`
	Finished int64   `json:"finished"`
	Created  int64   `json:"created"`
	Updated  int64   `json:"updated"`
	Stages   []Stage `json:"stages"`
}

// Stage represents a stage within a Drone CI build.
type Stage struct {
	ID       int64  `json:"id"`
	BuildID  int64  `json:"build_id"`
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Started  int64  `json:"started"`
	Finished int64  `json:"stopped"`
	Steps    []Step `json:"steps"`
}

// Step represents a step within a Drone CI stage.
type Step struct {
	ID       int64  `json:"id"`
	StageID  int64  `json:"stage_id"`
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Started  int64  `json:"started"`
	Finished int64  `json:"stopped"`
}

// LogLine represents a single line of Drone CI build logs.
type LogLine struct {
	Number int    `json:"pos"`
	Out    string `json:"out"`
	Time   int64  `json:"time"`
}

// HTTPClient is the concrete implementation of Client that communicates
// with a Drone CI server over HTTP.
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewHTTPClient creates a new HTTPClient pointed at the given Drone CI server.
func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	return resp, nil
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("drone API error %d: %s", resp.StatusCode, string(body))
}

func decode[T any](resp *http.Response, v *T) error {
	defer func() { _ = resp.Body.Close() }()
	if err := checkStatus(resp); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// ListRepos returns all repositories enabled for the authenticated user.
func (c *HTTPClient) ListRepos(ctx context.Context) ([]Repo, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/user/repos")
	if err != nil {
		return nil, err
	}
	var repos []Repo
	return repos, decode(resp, &repos)
}

// GetRepo returns details for a specific repository.
func (c *HTTPClient) GetRepo(ctx context.Context, owner, name string) (*Repo, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/repos/"+owner+"/"+name)
	if err != nil {
		return nil, err
	}
	var repo Repo
	return &repo, decode(resp, &repo)
}

// ListBuilds returns the build history for a repository.
func (c *HTTPClient) ListBuilds(ctx context.Context, owner, name string) ([]Build, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/repos/"+owner+"/"+name+"/builds")
	if err != nil {
		return nil, err
	}
	var builds []Build
	return builds, decode(resp, &builds)
}

// GetBuild returns details for a specific build, including its stages and steps.
func (c *HTTPClient) GetBuild(ctx context.Context, owner, name string, number int) (*Build, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d", owner, name, number)
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

// GetBuildLogs returns the log lines for a specific step in a build.
func (c *HTTPClient) GetBuildLogs(ctx context.Context, owner, name string, buildNumber, stage, step int) ([]LogLine, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d/logs/%d/%d", owner, name, buildNumber, stage, step)
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var lines []LogLine
	return lines, decode(resp, &lines)
}

// TriggerBuild triggers a new build for the given branch.
func (c *HTTPClient) TriggerBuild(ctx context.Context, owner, name, branch string) (*Build, error) {
	params := url.Values{}
	if branch != "" {
		params.Set("branch", branch)
	}
	path := "/api/repos/" + owner + "/" + name + "/builds?" + params.Encode()
	resp, err := c.do(ctx, http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

// CancelBuild cancels a running build.
func (c *HTTPClient) CancelBuild(ctx context.Context, owner, name string, number int) error {
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d", owner, name, number)
	resp, err := c.do(ctx, http.MethodDelete, path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return checkStatus(resp)
}

// RestartBuild creates a new build from an existing build's commit.
func (c *HTTPClient) RestartBuild(ctx context.Context, owner, name string, number int) (*Build, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d", owner, name, number)
	resp, err := c.do(ctx, http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

