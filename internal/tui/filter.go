package tui

import "strings"

type filterOp struct {
	negate bool
	field  string
	value  string
}

type filterQuery struct {
	ops []filterOp
}

func parseFilter(text string) filterQuery {
	var query filterQuery
	for _, token := range strings.Fields(text) {
		op := filterOp{}
		if len(token) > 1 && strings.HasPrefix(token, "-") {
			op.negate = true
			token = token[1:]
		}
		if field, value, ok := strings.Cut(token, ":"); ok && strings.EqualFold(field, "repo") {
			if value == "" {
				continue
			}
			op.field = "repo"
			op.value = strings.ToLower(value)
		} else {
			op.value = strings.ToLower(token)
		}
		query.ops = append(query.ops, op)
	}
	return query
}

func (q filterQuery) matches(row WorktreeRow) bool {
	for _, op := range q.ops {
		matched := op.matches(row)
		if op.negate {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}

func (op filterOp) matches(row WorktreeRow) bool {
	if op.field == "repo" {
		return strings.Contains(strings.ToLower(row.Worktree.RepoName), op.value)
	}
	return matchesBareFilter(row, op.value)
}

// filterRows returns rows matching every whitespace-separated filter operand.
// Bare operands preserve the original case-insensitive substring behavior.
func filterRows(rows []WorktreeRow, text string) []WorktreeRow {
	query := parseFilter(text)
	if len(query.ops) == 0 {
		return rows
	}
	var result []WorktreeRow
	for _, r := range rows {
		if query.matches(r) {
			result = append(result, r)
		}
	}
	return result
}

// FilterRows exposes the TUI's filter contract to the non-interactive printer.
func FilterRows(rows []WorktreeRow, text string) []WorktreeRow {
	return filterRows(rows, text)
}

func matchesBareFilter(r WorktreeRow, needle string) bool {
	if strings.Contains(strings.ToLower(BranchLabel(r)), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Worktree.RepoName), needle) {
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
