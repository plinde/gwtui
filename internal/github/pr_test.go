package github

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPRUnmarshal_AllStates(t *testing.T) {
	data := `[
		{"number":1,"title":"Open PR","state":"OPEN","isDraft":false,"headRefName":"feat-open"},
		{"number":2,"title":"Closed PR","state":"CLOSED","isDraft":false,"headRefName":"feat-closed"},
		{"number":3,"title":"Merged PR","state":"MERGED","isDraft":false,"headRefName":"feat-merged"}
	]`

	var prs []PR
	if err := json.Unmarshal([]byte(data), &prs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(prs) != 3 {
		t.Fatalf("expected 3 PRs, got %d", len(prs))
	}

	expected := []struct {
		Number  int
		Title   string
		State   string
		HeadRef string
	}{
		{1, "Open PR", "OPEN", "feat-open"},
		{2, "Closed PR", "CLOSED", "feat-closed"},
		{3, "Merged PR", "MERGED", "feat-merged"},
	}

	for i, want := range expected {
		pr := prs[i]
		if pr.Number != want.Number {
			t.Errorf("prs[%d].Number = %d, want %d", i, pr.Number, want.Number)
		}
		if pr.Title != want.Title {
			t.Errorf("prs[%d].Title = %q, want %q", i, pr.Title, want.Title)
		}
		if pr.State != want.State {
			t.Errorf("prs[%d].State = %q, want %q", i, pr.State, want.State)
		}
		if pr.HeadRef != want.HeadRef {
			t.Errorf("prs[%d].HeadRef = %q, want %q", i, pr.HeadRef, want.HeadRef)
		}
	}
}

func TestPRUnmarshal_Draft(t *testing.T) {
	data := `[{"number":42,"title":"WIP: draft","state":"OPEN","isDraft":true,"headRefName":"draft-branch"}]`

	var prs []PR
	if err := json.Unmarshal([]byte(data), &prs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if !prs[0].IsDraft {
		t.Error("expected IsDraft=true")
	}
	if prs[0].State != "OPEN" {
		t.Errorf("expected State=OPEN, got %q", prs[0].State)
	}
}

func TestPRUnmarshal_EmptyArray(t *testing.T) {
	data := `[]`

	var prs []PR
	if err := json.Unmarshal([]byte(data), &prs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(prs) != 0 {
		t.Fatalf("expected 0 PRs, got %d", len(prs))
	}
}

func TestPRMapConstruction_HeadRefAsKey(t *testing.T) {
	data := `[
		{"number":10,"title":"PR A","state":"OPEN","isDraft":false,"headRefName":"branch-a"},
		{"number":20,"title":"PR B","state":"MERGED","isDraft":false,"headRefName":"branch-b"}
	]`

	var prs []PR
	if err := json.Unmarshal([]byte(data), &prs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Build the branch-keyed map used by the TUI enrichment path.
	result := make(map[string]*PR, len(prs))
	for i := range prs {
		result[prs[i].HeadRef] = &prs[i]
	}

	if pr, ok := result["branch-a"]; !ok {
		t.Error("expected key 'branch-a' in map")
	} else if pr.Number != 10 {
		t.Errorf("expected PR number 10 for branch-a, got %d", pr.Number)
	}

	if pr, ok := result["branch-b"]; !ok {
		t.Error("expected key 'branch-b' in map")
	} else if pr.Number != 20 {
		t.Errorf("expected PR number 20 for branch-b, got %d", pr.Number)
	}
}

func TestPRMapConstruction_DuplicateBranch_LastWins(t *testing.T) {
	data := `[
		{"number":1,"title":"First","state":"CLOSED","isDraft":false,"headRefName":"same-branch"},
		{"number":2,"title":"Second","state":"OPEN","isDraft":false,"headRefName":"same-branch"}
	]`

	var prs []PR
	if err := json.Unmarshal([]byte(data), &prs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	result := make(map[string]*PR, len(prs))
	for i := range prs {
		result[prs[i].HeadRef] = &prs[i]
	}

	pr, ok := result["same-branch"]
	if !ok {
		t.Fatal("expected key 'same-branch' in map")
	}
	// The loop iterates in order, so the last PR with the same HeadRef wins
	if pr.Number != 2 {
		t.Errorf("expected last PR (number=2) to win for duplicate branch, got %d", pr.Number)
	}
	if pr.Title != "Second" {
		t.Errorf("expected title 'Second', got %q", pr.Title)
	}
}

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		remote string
		host   string
		owner  string
		repo   string
	}{
		{"git@github.com:plinde/gwtui.git", "github.com", "plinde", "gwtui"},
		{"https://github.com/Plinde/gwtui", "github.com", "Plinde", "gwtui"},
		{"ssh://git@git.example.com/Acme/api.git", "git.example.com", "Acme", "api"},
	}
	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			host, owner, repo, err := parseRemoteURL(tt.remote)
			if err != nil {
				t.Fatal(err)
			}
			if host != tt.host || owner != tt.owner || repo != tt.repo {
				t.Fatalf("got %s/%s/%s, want %s/%s/%s",
					host, owner, repo, tt.host, tt.owner, tt.repo)
			}
		})
	}
}

func TestBuildBatchQuery_TargetsOnlyRequestedBranches(t *testing.T) {
	request, repos := buildBatchQuery([]remoteRepository{
		{Key: "web", Owner: "Acme", Name: "web", Branches: []string{"feature/z", "feature/z"}},
		{Key: "api", Owner: "Acme", Name: "api", Branches: []string{"feature/a"}},
	})

	if len(repos) != 2 {
		t.Fatalf("expected 2 repository aliases, got %d", len(repos))
	}
	if strings.Count(request.Query, "repository(") != 2 {
		t.Fatalf("expected 2 repository fields: %s", request.Query)
	}
	if strings.Count(request.Query, "pullRequests(") != 2 {
		t.Fatalf("expected one connection per unique branch: %s", request.Query)
	}
	if strings.Contains(request.Query, "200") || !strings.Contains(request.Query, "first:1") {
		t.Fatalf("query should fetch only the newest matching PR: %s", request.Query)
	}
	if request.Variables["branch0_0"] != "feature/a" ||
		request.Variables["branch1_0"] != "feature/z" {
		t.Fatalf("unexpected branch variables: %#v", request.Variables)
	}
}

func TestPRsByBranches_BatchesRepositoriesOnSameHost(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "api")
	web := filepath.Join(root, "web")
	initRemoteRepo(t, api, "git@github.com:Acme/api.git")
	initRemoteRepo(t, web, "https://github.com/Acme/web.git")

	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "calls.log")
	ghPath := filepath.Join(binDir, "gh")
	script := `#!/bin/sh
printf 'call\n' >> "$GWTUI_GH_CALL_LOG"
cat >/dev/null
printf '%s\n' '{"data":{"r0":{"b0":{"nodes":[{"number":1,"title":"API","state":"MERGED","isDraft":false,"headRefName":"feature/api"}]}},"r1":{"b0":{"nodes":[{"number":2,"title":"Web","state":"OPEN","isDraft":false,"headRefName":"feature/web"}]}},"rateLimit":{"cost":1}}}'
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GWTUI_GH_CALL_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prs, errs := PRsByBranches([]RepositoryBranches{
		{Key: api, Path: api, Branches: []string{"feature/api"}},
		{Key: web, Path: web, Branches: []string{"feature/web"}},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(calls), "call\n"); got != 1 {
		t.Fatalf("expected one GraphQL process for one host, got %d", got)
	}
	if prs[api]["feature/api"].State != "MERGED" ||
		prs[web]["feature/web"].State != "OPEN" {
		t.Fatalf("unexpected PR results: %#v", prs)
	}
}

func initRemoteRepo(t *testing.T, path, remote string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"remote", "add", "origin", remote},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
}
