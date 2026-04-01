package drone_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stanleygoldman/drone-ci-mcp-server/internal/drone"
)

func newTestServer(mux *http.ServeMux) (*httptest.Server, *drone.HTTPClient) {
	srv := httptest.NewServer(mux)
	client := drone.NewHTTPClient(srv.URL, "test-token")
	return srv, client
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TestListRepos(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/user/repos", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, []drone.Repo{
			{ID: 1, Namespace: "acme", Name: "widget", IsActive: true},
			{ID: 2, Namespace: "acme", Name: "gadget", IsActive: false},
		})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	repos, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("want 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "widget" {
		t.Errorf("want repos[0].Name=widget, got %q", repos[0].Name)
	}
}

func TestGetRepo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Repo{ID: 1, Namespace: "acme", Name: "widget", IsActive: true})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	repo, err := client.GetRepo(context.Background(), "acme", "widget")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if repo.Name != "widget" {
		t.Errorf("want Name=widget, got %q", repo.Name)
	}
}

func TestListBuilds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/builds", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.Build{
			{ID: 1, Number: 1, Status: "success"},
			{ID: 2, Number: 2, Status: "running"},
		})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	builds, err := client.ListBuilds(context.Background(), "acme", "widget")
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if len(builds) != 2 {
		t.Fatalf("want 2 builds, got %d", len(builds))
	}
	if builds[1].Status != "running" {
		t.Errorf("want builds[1].Status=running, got %q", builds[1].Status)
	}
}

func TestGetBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/builds/42", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{
			ID:     42,
			Number: 42,
			Status: "failure",
			Stages: []drone.Stage{
				{Number: 1, Name: "default", Steps: []drone.Step{
					{Number: 1, Name: "clone", Status: "success"},
					{Number: 2, Name: "test", Status: "failure"},
				}},
			},
		})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.GetBuild(context.Background(), "acme", "widget", 42)
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if build.Status != "failure" {
		t.Errorf("want Status=failure, got %q", build.Status)
	}
	if len(build.Stages) != 1 || len(build.Stages[0].Steps) != 2 {
		t.Errorf("unexpected stages/steps: %+v", build.Stages)
	}
}

func TestGetBuildLogs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/builds/42/logs/1/2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.LogLine{
			{Number: 0, Out: "running tests\n", Time: 0},
			{Number: 1, Out: "PASS\n", Time: 1},
		})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	lines, err := client.GetBuildLogs(context.Background(), "acme", "widget", 42, 1, 2)
	if err != nil {
		t.Fatalf("GetBuildLogs: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 log lines, got %d", len(lines))
	}
	if lines[1].Out != "PASS\n" {
		t.Errorf("want lines[1].Out=PASS, got %q", lines[1].Out)
	}
}

func TestTriggerBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("branch") != "main" {
			http.Error(w, "bad branch", http.StatusBadRequest)
			return
		}
		writeJSON(w, drone.Build{ID: 99, Number: 99, Status: "pending"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.TriggerBuild(context.Background(), "acme", "widget", "main")
	if err != nil {
		t.Fatalf("TriggerBuild: %v", err)
	}
	if build.Number != 99 {
		t.Errorf("want Number=99, got %d", build.Number)
	}
}

func TestCancelBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/repos/acme/widget/builds/5", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	if err := client.CancelBuild(context.Background(), "acme", "widget", 5); err != nil {
		t.Fatalf("CancelBuild: %v", err)
	}
}

func TestRestartBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds/5", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{ID: 100, Number: 6, Status: "pending"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.RestartBuild(context.Background(), "acme", "widget", 5)
	if err != nil {
		t.Fatalf("RestartBuild: %v", err)
	}
	if build.Number != 6 {
		t.Errorf("want Number=6, got %d", build.Number)
	}
}

func TestAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	_, err := client.GetRepo(context.Background(), "acme", "missing")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}
