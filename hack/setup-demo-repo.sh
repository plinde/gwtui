#!/usr/bin/env bash
# setup-demo-repo.sh — Create a throwaway GitHub repo with realistic worktrees
# for taking gwtui screenshots.
#
# Usage: ./hack/setup-demo-repo.sh
# Cleanup: gh repo delete plinde/gwtui-demo --yes && rm -rf ~/workspace/github.com/plinde/gwtui-demo

set -euo pipefail

REPO_NAME="gwtui-demo"
REPO_OWNER="plinde"
REPO_FULL="${REPO_OWNER}/${REPO_NAME}"
LOCAL_DIR="${HOME}/workspace/github.com/${REPO_OWNER}/${REPO_NAME}"
WT_DIR="${LOCAL_DIR}/.worktrees"

# ──────────────────────────────────────────────
# Branch definitions
# Format: branch|pr_title|pr_body|final_state
#   final_state: merged, closed, open, draft, none
# ──────────────────────────────────────────────
BRANCHES=(
  "feat/add-bar-support|Add bar format support|Implements bar file parsing and rendering|merged"
  "fix/baz-null-pointer|Fix null pointer in baz handler|Fixes crash when baz input is nil|merged"
  "docs/update-examples|Update usage examples|Refresh examples for v2 API changes|merged"
  "refactor/foo-module|Refactor foo module internals|Extract shared logic into helper functions|merged"
  "chore/cleanup-baz-deps|Clean up baz dependencies|Remove unused transitive deps from baz|merged"
  "feat/example-widgets|Add example widget gallery|Adds interactive widget examples to docs|merged"
  "feat/experimental-qux|Experimental qux integration|Prototype qux protocol support|closed"
  "fix/deprecated-bar-api|Fix deprecated bar API calls|Migrate from bar v1 to v2 endpoints|closed"
  "feat/foo-dashboard|Add foo analytics dashboard|Real-time metrics dashboard for foo operations|open"
  "feat/bar-notifications|Add bar event notifications|Push notifications when bar events trigger|open"
  "feat/foo-search|Add full-text search for foo|Elasticsearch-backed search across foo records|open"
  "feat/baz-streaming|Add baz streaming support|Server-sent events for real-time baz updates|draft"
  "wip/example-plugin|WIP: example plugin system|Exploring plugin architecture for extensibility|draft"
  "feat/bar-dark-mode|Add dark mode for bar UI|Alternate color scheme for bar components|draft"
  "spike/foo-prototype|n/a|n/a|none"
  "test/bar-integration|n/a|n/a|none"
)

# Dummy files to create per branch (for realistic commits)
declare -A BRANCH_FILES=(
  ["feat/add-bar-support"]="internal/bar/parser.go"
  ["fix/baz-null-pointer"]="internal/baz/handler.go"
  ["docs/update-examples"]="docs/examples.md"
  ["refactor/foo-module"]="internal/foo/helpers.go"
  ["chore/cleanup-baz-deps"]="go.mod"
  ["feat/example-widgets"]="docs/widgets.md"
  ["feat/experimental-qux"]="internal/qux/proto.go"
  ["fix/deprecated-bar-api"]="internal/bar/client.go"
  ["feat/foo-dashboard"]="internal/foo/dashboard.go"
  ["feat/bar-notifications"]="internal/bar/notify.go"
  ["feat/foo-search"]="internal/foo/search.go"
  ["feat/baz-streaming"]="internal/baz/stream.go"
  ["wip/example-plugin"]="internal/plugin/loader.go"
  ["feat/bar-dark-mode"]="internal/bar/theme.go"
  ["spike/foo-prototype"]="internal/foo/prototype.go"
  ["test/bar-integration"]="tests/bar_integration_test.go"
)

echo "==> Checking GitHub repo ${REPO_FULL}"
if gh repo view "${REPO_FULL}" --json name &>/dev/null; then
  echo "  -> Repo already exists, reusing it"
else
  echo "  -> Creating repo (private)"
  gh repo create "${REPO_FULL}" --private --clone=false --description "Demo repo for gwtui screenshots"
fi

echo "==> Initializing local repo at ${LOCAL_DIR}"
mkdir -p "${LOCAL_DIR}"
cd "${LOCAL_DIR}"
git init

# Disable global hooks for this throwaway repo
git config core.hooksPath /dev/null

git remote add origin "https://github.com/${REPO_FULL}.git"

# Create initial commit on main
mkdir -p internal
cat > main.go <<'GOEOF'
package main

import "fmt"

func main() {
	fmt.Println("gwtui-demo")
}
GOEOF

cat > go.mod <<'GOMOD'
module github.com/plinde/gwtui-demo

go 1.22
GOMOD

git add main.go go.mod
git commit -m "feat: initial project scaffold"
git branch -M main
git push -u origin main

echo "==> Creating branches and pushing"
for entry in "${BRANCHES[@]}"; do
  IFS='|' read -r branch _title _body _state <<< "${entry}"
  file="${BRANCH_FILES[$branch]}"

  echo "  -> ${branch}"
  git checkout main
  git checkout -b "${branch}"

  # Create a unique file for this branch
  dir="$(dirname "${file}")"
  pkg="$(basename "${dir}")"
  mkdir -p "${dir}"
  cat > "${file}" <<FEOF
// ${branch} — placeholder for demo
package ${pkg}
FEOF

  git add "${file}"
  git commit -m "feat: add ${branch} changes"
  git push -u origin "${branch}"
done

# Return to main
git checkout main

echo "==> Creating PRs"
for entry in "${BRANCHES[@]}"; do
  IFS='|' read -r branch title body state <<< "${entry}"

  [[ "${state}" == "none" ]] && continue

  echo "  -> PR for ${branch} (target: ${state})"
  if [[ "${state}" == "draft" ]]; then
    gh pr create --head "${branch}" --base main --title "${title}" --body "${body}" --draft
  else
    gh pr create --head "${branch}" --base main --title "${title}" --body "${body}"
  fi
done

echo "==> Merging PRs"
for entry in "${BRANCHES[@]}"; do
  IFS='|' read -r branch _title _body state <<< "${entry}"
  [[ "${state}" != "merged" ]] && continue

  echo "  -> Merging ${branch}"
  gh pr merge "${branch}" --squash --delete-branch=false
done

echo "==> Closing PRs"
for entry in "${BRANCHES[@]}"; do
  IFS='|' read -r branch _title _body state <<< "${entry}"
  [[ "${state}" != "closed" ]] && continue

  echo "  -> Closing ${branch}"
  gh pr close "${branch}"
done

echo "==> Creating worktrees"
mkdir -p "${WT_DIR}"
for entry in "${BRANCHES[@]}"; do
  IFS='|' read -r branch _title _body _state <<< "${entry}"
  wt_name="${branch//\//-}"

  echo "  -> worktree: ${wt_name}"
  git worktree add "${WT_DIR}/${wt_name}" "${branch}"
done

echo ""
echo "=== Done! ==="
echo "Run:  gwtui ${LOCAL_DIR}"
echo "Cleanup: gh repo delete ${REPO_FULL} --yes && rm -rf ${LOCAL_DIR}"
