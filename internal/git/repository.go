package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Repository is a direct checkout under an org root.
type Repository struct {
	Name string
	Path string
}

// RepositorySet describes the checkout scope gwtui should load.
type RepositorySet struct {
	Root         string
	Repositories []Repository
	IsOrg        bool
}

// ResolveTarget resolves targetPath as either a single git checkout or an org
// root containing direct child checkouts. Linked worktree directories are
// skipped when scanning an org root.
func ResolveTarget(targetPath string) (RepositorySet, error) {
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return RepositorySet{}, err
	}

	if isGitWorkTree(abs) {
		return RepositorySet{
			Root: abs,
			Repositories: []Repository{{
				Name: filepath.Base(abs),
				Path: abs,
			}},
		}, nil
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return RepositorySet{}, err
	}

	var repos []Repository
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		child := filepath.Join(abs, entry.Name())
		if isLinkedWorktree(child) || !isGitWorkTree(child) {
			continue
		}
		repos = append(repos, Repository{
			Name: entry.Name(),
			Path: child,
		})
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})

	if len(repos) == 0 {
		return RepositorySet{}, fmt.Errorf("%s is neither a git repository nor an org root with direct checkouts", targetPath)
	}

	return RepositorySet{
		Root:         abs,
		Repositories: repos,
		IsOrg:        true,
	}, nil
}

func isGitWorkTree(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func isLinkedWorktree(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}
