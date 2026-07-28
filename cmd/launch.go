package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type launchScope struct {
	targetPath    string
	initialFilter string
}

func resolveLaunch(cwd, positional, orgArg, repoArg string) (launchScope, error) {
	if positional != "" {
		if orgArg != "" || repoArg != "" {
			return launchScope{}, fmt.Errorf("path argument cannot be combined with --org or --repo")
		}
		return launchScope{targetPath: absoluteFrom(cwd, positional)}, nil
	}

	if orgArg != "" {
		root, err := resolveOrgRoot(cwd, orgArg)
		if err != nil {
			return launchScope{}, err
		}
		if strings.ContainsAny(repoArg, `/\`) {
			return launchScope{}, fmt.Errorf("--repo with --org must be a repository name, not a path")
		}
		scope := launchScope{targetPath: root}
		if repoArg != "" {
			scope.initialFilter = "repo:" + repoArg
		}
		return scope, nil
	}

	// Backward compatibility: --repo without --org remains a target path.
	if repoArg != "" {
		return launchScope{targetPath: absoluteFrom(cwd, repoArg)}, nil
	}

	return inferLaunchFromCWD(cwd)
}

func inferLaunchFromCWD(cwd string) (launchScope, error) {
	_, orgRoot, err := inferOrgFromPath(cwd)
	if err == nil {
		scope := launchScope{targetPath: orgRoot}
		if repoRoot, repoErr := owningRepoRoot(cwd); repoErr == nil &&
			samePath(filepath.Dir(repoRoot), orgRoot) {
			scope.initialFilter = "repo:" + filepath.Base(repoRoot)
		}
		return scope, nil
	}

	if repoRoot, repoErr := gitRepoRootAt(cwd); repoErr == nil {
		return launchScope{targetPath: repoRoot}, nil
	}
	return launchScope{targetPath: filepath.Clean(cwd)}, nil
}

func samePath(a, b string) bool {
	clean := func(path string) string {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Clean(resolved)
		}
		return filepath.Clean(path)
	}
	return clean(a) == clean(b)
}

func resolveOrgRoot(cwd, orgArg string) (string, error) {
	if filepath.IsAbs(orgArg) || strings.ContainsAny(orgArg, `/\`) || strings.HasPrefix(orgArg, ".") {
		return absoluteFrom(cwd, orgArg), nil
	}
	if info, err := os.Stat(filepath.Join(cwd, orgArg)); err == nil && info.IsDir() {
		return filepath.Join(cwd, orgArg), nil
	}

	_, currentRoot, err := inferOrgFromPath(cwd)
	if err != nil {
		return "", fmt.Errorf("cannot resolve org %q outside a github.com/<org> path; pass an org-root path", orgArg)
	}
	if strings.EqualFold(filepath.Base(currentRoot), orgArg) {
		return currentRoot, nil
	}
	return filepath.Join(filepath.Dir(currentRoot), orgArg), nil
}

func inferOrgFromPath(path string) (org, root string, err error) {
	clean := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(clean, "/")
	for i, part := range parts {
		if part == "github.com" && i+1 < len(parts) {
			org = parts[i+1]
			root = filepath.FromSlash(strings.Join(parts[:i+2], "/"))
			return org, root, nil
		}
	}
	return "", "", fmt.Errorf("no github.com/<org> segment found in %s", path)
}

func owningRepoRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	commonDir := strings.TrimSpace(string(out))
	return filepath.Dir(commonDir), nil
}

func gitRepoRootAt(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func absoluteFrom(cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}
