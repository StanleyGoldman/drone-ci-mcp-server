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

func TestPromoteBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds/5/promote", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("target") != "production" {
			http.Error(w, "bad target", http.StatusBadRequest)
			return
		}
		writeJSON(w, drone.Build{ID: 200, Number: 6, Status: "pending"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.PromoteBuild(context.Background(), "acme", "widget", 5, "production")
	if err != nil {
		t.Fatalf("PromoteBuild: %v", err)
	}
	if build.Number != 6 {
		t.Errorf("want Number=6, got %d", build.Number)
	}
}

func TestRollbackBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds/5/rollback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("target") != "staging" {
			http.Error(w, "bad target", http.StatusBadRequest)
			return
		}
		writeJSON(w, drone.Build{ID: 201, Number: 7, Status: "pending"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.RollbackBuild(context.Background(), "acme", "widget", 5, "staging")
	if err != nil {
		t.Fatalf("RollbackBuild: %v", err)
	}
	if build.Number != 7 {
		t.Errorf("want Number=7, got %d", build.Number)
	}
}

func TestApproveBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds/5/approve/2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{ID: 5, Number: 5, Status: "running"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.ApproveBuild(context.Background(), "acme", "widget", 5, 2)
	if err != nil {
		t.Fatalf("ApproveBuild: %v", err)
	}
	if build.Status != "running" {
		t.Errorf("want Status=running, got %q", build.Status)
	}
}

func TestDeclineBuild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/builds/5/decline/2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Build{ID: 5, Number: 5, Status: "declined"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	build, err := client.DeclineBuild(context.Background(), "acme", "widget", 5, 2)
	if err != nil {
		t.Fatalf("DeclineBuild: %v", err)
	}
	if build.Status != "declined" {
		t.Errorf("want Status=declined, got %q", build.Status)
	}
}

func TestListSecrets(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/secrets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.Secret{
			{ID: 1, Name: "docker_password"},
			{ID: 2, Name: "npm_token"},
		})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	secrets, err := client.ListSecrets(context.Background(), "acme", "widget")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(secrets) != 2 {
		t.Fatalf("want 2 secrets, got %d", len(secrets))
	}
	if secrets[0].Name != "docker_password" {
		t.Errorf("want secrets[0].Name=docker_password, got %q", secrets[0].Name)
	}
}

func TestGetSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos/acme/widget/secrets/docker_password", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Secret{ID: 1, Name: "docker_password"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	s, err := client.GetSecret(context.Background(), "acme", "widget", "docker_password")
	if err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if s.Name != "docker_password" {
		t.Errorf("want Name=docker_password, got %q", s.Name)
	}
}

func TestCreateSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/repos/acme/widget/secrets", func(w http.ResponseWriter, r *http.Request) {
		var input drone.SecretInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, drone.Secret{ID: 3, Name: input.Name, PullRequest: input.PullRequest})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	s, err := client.CreateSecret(context.Background(), "acme", "widget", drone.SecretInput{
		Name:        "new_secret",
		Data:        "s3cr3t",
		PullRequest: true,
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	if s.Name != "new_secret" {
		t.Errorf("want Name=new_secret, got %q", s.Name)
	}
	if !s.PullRequest {
		t.Error("want PullRequest=true")
	}
}

func TestUpdateSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/repos/acme/widget/secrets/docker_password", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Secret{ID: 1, Name: "docker_password"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	s, err := client.UpdateSecret(context.Background(), "acme", "widget", "docker_password", drone.SecretInput{Data: "newval"})
	if err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	if s.Name != "docker_password" {
		t.Errorf("want Name=docker_password, got %q", s.Name)
	}
}

func TestDeleteSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/repos/acme/widget/secrets/docker_password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	if err := client.DeleteSecret(context.Background(), "acme", "widget", "docker_password"); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
}

func TestListOrgSecrets(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/secrets/acme", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []drone.Secret{
			{ID: 10, Namespace: "acme", Name: "shared_key"},
		})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	secrets, err := client.ListOrgSecrets(context.Background(), "acme")
	if err != nil {
		t.Fatalf("ListOrgSecrets: %v", err)
	}
	if len(secrets) != 1 || secrets[0].Name != "shared_key" {
		t.Errorf("unexpected secrets: %+v", secrets)
	}
}

func TestGetOrgSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/secrets/acme/shared_key", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Secret{ID: 10, Namespace: "acme", Name: "shared_key"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	s, err := client.GetOrgSecret(context.Background(), "acme", "shared_key")
	if err != nil {
		t.Fatalf("GetOrgSecret: %v", err)
	}
	if s.Name != "shared_key" {
		t.Errorf("want Name=shared_key, got %q", s.Name)
	}
}

func TestCreateOrgSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/secrets/acme", func(w http.ResponseWriter, r *http.Request) {
		var input drone.SecretInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, drone.Secret{ID: 11, Namespace: "acme", Name: input.Name})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	s, err := client.CreateOrgSecret(context.Background(), "acme", drone.SecretInput{Name: "org_secret", Data: "val"})
	if err != nil {
		t.Fatalf("CreateOrgSecret: %v", err)
	}
	if s.Name != "org_secret" {
		t.Errorf("want Name=org_secret, got %q", s.Name)
	}
}

func TestUpdateOrgSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/secrets/acme/shared_key", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, drone.Secret{ID: 10, Namespace: "acme", Name: "shared_key"})
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	s, err := client.UpdateOrgSecret(context.Background(), "acme", "shared_key", drone.SecretInput{Data: "newval"})
	if err != nil {
		t.Fatalf("UpdateOrgSecret: %v", err)
	}
	if s.Name != "shared_key" {
		t.Errorf("want Name=shared_key, got %q", s.Name)
	}
}

func TestDeleteOrgSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/secrets/acme/shared_key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv, client := newTestServer(mux)
	defer srv.Close()

	if err := client.DeleteOrgSecret(context.Background(), "acme", "shared_key"); err != nil {
		t.Fatalf("DeleteOrgSecret: %v", err)
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
