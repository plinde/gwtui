package github

import (
	"encoding/json"
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

	// Build the map the same way PRsByBranch does
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
