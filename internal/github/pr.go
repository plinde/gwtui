package github

import (
	"encoding/json"
	"os/exec"
)

// PR represents a GitHub pull request.
type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`     // "OPEN", "CLOSED", "MERGED"
	IsDraft   bool   `json:"isDraft"`
	HeadRef   string `json:"headRefName"`
}

// PRsByBranch returns a map of branch name → PR for the repo at repoPath.
// Uses `gh pr list --state all --json ...`.
func PRsByBranch(repoPath string) (map[string]*PR, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--state", "all",
		"--limit", "200",
		"--json", "number,title,state,isDraft,headRefName",
	)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}

	result := make(map[string]*PR, len(prs))
	for i := range prs {
		result[prs[i].HeadRef] = &prs[i]
	}
	return result, nil
}
