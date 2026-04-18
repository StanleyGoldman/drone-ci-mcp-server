package drone

import (
	"bytes"
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
	PromoteBuild(ctx context.Context, owner, name string, number int, target string) (*Build, error)
	RollbackBuild(ctx context.Context, owner, name string, number int, target string) (*Build, error)
	ApproveBuild(ctx context.Context, owner, name string, number, stage int) (*Build, error)
	DeclineBuild(ctx context.Context, owner, name string, number, stage int) (*Build, error)
	ListSecrets(ctx context.Context, owner, name string) ([]Secret, error)
	GetSecret(ctx context.Context, owner, name, secret string) (*Secret, error)
	CreateSecret(ctx context.Context, owner, name string, input SecretInput) (*Secret, error)
	UpdateSecret(ctx context.Context, owner, name, secret string, input SecretInput) (*Secret, error)
	DeleteSecret(ctx context.Context, owner, name, secret string) error
	ListOrgSecrets(ctx context.Context, namespace string) ([]Secret, error)
	GetOrgSecret(ctx context.Context, namespace, name string) (*Secret, error)
	CreateOrgSecret(ctx context.Context, namespace string, input SecretInput) (*Secret, error)
	UpdateOrgSecret(ctx context.Context, namespace, name string, input SecretInput) (*Secret, error)
	DeleteOrgSecret(ctx context.Context, namespace, name string) error
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

// Secret represents a Drone CI repository or organization secret.
type Secret struct {
	ID              int64  `json:"id"`
	RepoID          int64  `json:"repo_id"`
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	Data            string `json:"data"`
	PullRequest     bool   `json:"pull_request"`
	PullRequestPush bool   `json:"pull_request_push"`
}

// SecretInput is the request body for creating or updating a secret.
type SecretInput struct {
	Name            string `json:"name,omitempty"`
	Data            string `json:"data"`
	PullRequest     bool   `json:"pull_request"`
	PullRequestPush bool   `json:"pull_request_push"`
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

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	return resp, nil
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

// PromoteBuild promotes a build to a target environment.
func (c *HTTPClient) PromoteBuild(ctx context.Context, owner, name string, number int, target string) (*Build, error) {
	params := url.Values{}
	params.Set("target", target)
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d/promote?%s", owner, name, number, params.Encode())
	resp, err := c.do(ctx, http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

// RollbackBuild rolls back a build to a target environment.
func (c *HTTPClient) RollbackBuild(ctx context.Context, owner, name string, number int, target string) (*Build, error) {
	params := url.Values{}
	params.Set("target", target)
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d/rollback?%s", owner, name, number, params.Encode())
	resp, err := c.do(ctx, http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

// ApproveBuild approves a blocked stage in a build.
func (c *HTTPClient) ApproveBuild(ctx context.Context, owner, name string, number, stage int) (*Build, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d/approve/%d", owner, name, number, stage)
	resp, err := c.do(ctx, http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

// DeclineBuild declines a blocked stage in a build.
func (c *HTTPClient) DeclineBuild(ctx context.Context, owner, name string, number, stage int) (*Build, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/builds/%d/decline/%d", owner, name, number, stage)
	resp, err := c.do(ctx, http.MethodPost, path)
	if err != nil {
		return nil, err
	}
	var build Build
	return &build, decode(resp, &build)
}

// ListSecrets returns all secrets for a repository.
func (c *HTTPClient) ListSecrets(ctx context.Context, owner, name string) ([]Secret, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/secrets", owner, name)
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var secrets []Secret
	return secrets, decode(resp, &secrets)
}

// GetSecret returns a single secret by name for a repository.
func (c *HTTPClient) GetSecret(ctx context.Context, owner, name, secret string) (*Secret, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/secrets/%s", owner, name, secret)
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var s Secret
	return &s, decode(resp, &s)
}

// CreateSecret creates a new secret for a repository.
func (c *HTTPClient) CreateSecret(ctx context.Context, owner, name string, input SecretInput) (*Secret, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/secrets", owner, name)
	resp, err := c.doJSON(ctx, http.MethodPost, path, input)
	if err != nil {
		return nil, err
	}
	var s Secret
	return &s, decode(resp, &s)
}

// UpdateSecret updates an existing secret for a repository.
func (c *HTTPClient) UpdateSecret(ctx context.Context, owner, name, secret string, input SecretInput) (*Secret, error) {
	path := fmt.Sprintf("/api/repos/%s/%s/secrets/%s", owner, name, secret)
	resp, err := c.doJSON(ctx, http.MethodPatch, path, input)
	if err != nil {
		return nil, err
	}
	var s Secret
	return &s, decode(resp, &s)
}

// DeleteSecret deletes a secret from a repository.
func (c *HTTPClient) DeleteSecret(ctx context.Context, owner, name, secret string) error {
	path := fmt.Sprintf("/api/repos/%s/%s/secrets/%s", owner, name, secret)
	resp, err := c.do(ctx, http.MethodDelete, path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return checkStatus(resp)
}

// ListOrgSecrets returns all secrets for an organization namespace.
func (c *HTTPClient) ListOrgSecrets(ctx context.Context, namespace string) ([]Secret, error) {
	path := fmt.Sprintf("/api/secrets/%s", namespace)
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var secrets []Secret
	return secrets, decode(resp, &secrets)
}

// GetOrgSecret returns a single secret by name for an organization namespace.
func (c *HTTPClient) GetOrgSecret(ctx context.Context, namespace, name string) (*Secret, error) {
	path := fmt.Sprintf("/api/secrets/%s/%s", namespace, name)
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var s Secret
	return &s, decode(resp, &s)
}

// CreateOrgSecret creates a new secret for an organization namespace.
func (c *HTTPClient) CreateOrgSecret(ctx context.Context, namespace string, input SecretInput) (*Secret, error) {
	path := fmt.Sprintf("/api/secrets/%s", namespace)
	resp, err := c.doJSON(ctx, http.MethodPost, path, input)
	if err != nil {
		return nil, err
	}
	var s Secret
	return &s, decode(resp, &s)
}

// UpdateOrgSecret updates an existing secret for an organization namespace.
func (c *HTTPClient) UpdateOrgSecret(ctx context.Context, namespace, name string, input SecretInput) (*Secret, error) {
	path := fmt.Sprintf("/api/secrets/%s/%s", namespace, name)
	resp, err := c.doJSON(ctx, http.MethodPatch, path, input)
	if err != nil {
		return nil, err
	}
	var s Secret
	return &s, decode(resp, &s)
}

// DeleteOrgSecret deletes a secret from an organization namespace.
func (c *HTTPClient) DeleteOrgSecret(ctx context.Context, namespace, name string) error {
	path := fmt.Sprintf("/api/secrets/%s/%s", namespace, name)
	resp, err := c.do(ctx, http.MethodDelete, path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return checkStatus(resp)
}

