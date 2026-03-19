package tui

import "strings"

// filterRows returns only rows that match the filter text (case-insensitive substring).
// Matches against branch name, status text, and path.
func filterRows(rows []WorktreeRow, text string) []WorktreeRow {
	if text == "" {
		return rows
	}
	needle := strings.ToLower(text)
	var result []WorktreeRow
	for _, r := range rows {
		if matchesFilter(r, needle) {
			result = append(result, r)
		}
	}
	return result
}

func matchesFilter(r WorktreeRow, needle string) bool {
	if strings.Contains(strings.ToLower(r.Worktree.Branch), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Worktree.Path), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(string(r.State)), needle) {
		return true
	}
	if r.PR != nil {
		if strings.Contains(strings.ToLower(r.PR.State), needle) {
			return true
		}
		if strings.Contains(strings.ToLower(r.PR.HeadRef), needle) {
			return true
		}
	}
	return false
}
