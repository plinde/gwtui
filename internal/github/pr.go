package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
)

// PR represents a GitHub pull request.
type PR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"` // "OPEN", "CLOSED", "MERGED"
	IsDraft bool   `json:"isDraft"`
	HeadRef string `json:"headRefName"`
}

// RepositoryBranches identifies the local branches whose PR state is needed.
// Key is returned unchanged so callers do not need to key results by remote name.
type RepositoryBranches struct {
	Key      string
	Path     string
	Branches []string
}

type remoteRepository struct {
	Key      string
	Owner    string
	Name     string
	Branches []string
}

type queryBranch struct {
	Alias string
	Name  string
}

type queryRepository struct {
	Alias    string
	Key      string
	Branches []queryBranch
}

type graphQLRequest struct {
	Query     string            `json:"query"`
	Variables map[string]string `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// PRsByBranches returns the newest PR for each requested branch. Repositories
// on the same GitHub host are combined into one GraphQL request.
func PRsByBranches(repos []RepositoryBranches) (map[string]map[string]*PR, map[string]error) {
	result := make(map[string]map[string]*PR, len(repos))
	errs := make(map[string]error)
	byHost := make(map[string][]remoteRepository)

	for _, repo := range repos {
		result[repo.Key] = make(map[string]*PR)
		branches := uniqueBranches(repo.Branches)
		if len(branches) == 0 {
			continue
		}
		host, owner, name, err := resolveRemoteRepository(repo.Path)
		if err != nil {
			errs[repo.Key] = err
			continue
		}
		byHost[host] = append(byHost[host], remoteRepository{
			Key:      repo.Key,
			Owner:    owner,
			Name:     name,
			Branches: branches,
		})
	}

	for host, hostRepos := range byHost {
		prs, err := queryHost(host, hostRepos)
		if err != nil {
			for _, repo := range hostRepos {
				errs[repo.Key] = err
			}
			continue
		}
		for key, byBranch := range prs {
			result[key] = byBranch
		}
	}
	return result, errs
}

func uniqueBranches(branches []string) []string {
	seen := make(map[string]struct{}, len(branches))
	result := make([]string, 0, len(branches))
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}
		if _, ok := seen[branch]; ok {
			continue
		}
		seen[branch] = struct{}{}
		result = append(result, branch)
	}
	sort.Strings(result)
	return result
}

func resolveRemoteRepository(repoPath string) (host, owner, name string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("resolve origin remote: %w", err)
	}
	return parseRemoteURL(strings.TrimSpace(string(out)))
}

func parseRemoteURL(remote string) (host, owner, name string, err error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", "", "", fmt.Errorf("origin remote is empty")
	}

	var path string
	if parsed, parseErr := url.Parse(remote); parseErr == nil && parsed.Host != "" {
		host = parsed.Hostname()
		path = parsed.Path
	} else if colon := strings.Index(remote, ":"); colon > 0 &&
		!strings.Contains(remote[:colon], "/") {
		hostPart := remote[:colon]
		if at := strings.LastIndex(hostPart, "@"); at >= 0 {
			hostPart = hostPart[at+1:]
		}
		host = hostPart
		path = remote[colon+1:]
	}

	parts := strings.Split(strings.Trim(strings.TrimSuffix(path, ".git"), "/"), "/")
	if host == "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("unsupported origin remote %q", remote)
	}
	return strings.ToLower(host), parts[0], parts[1], nil
}

func queryHost(host string, repos []remoteRepository) (map[string]map[string]*PR, error) {
	request, queryRepos := buildBatchQuery(repos)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("gh", "api", "graphql", "--hostname", host, "--input", "-")
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("query pull requests: %w", err)
	}
	return parseBatchResponse(out, queryRepos)
}

func buildBatchQuery(repos []remoteRepository) (graphQLRequest, []queryRepository) {
	sorted := append([]remoteRepository(nil), repos...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Owner != sorted[j].Owner {
			return sorted[i].Owner < sorted[j].Owner
		}
		return sorted[i].Name < sorted[j].Name
	})

	var definitions []string
	var body strings.Builder
	variables := make(map[string]string)
	queryRepos := make([]queryRepository, 0, len(sorted))

	for repoIndex, repo := range sorted {
		ownerVar := fmt.Sprintf("owner%d", repoIndex)
		nameVar := fmt.Sprintf("name%d", repoIndex)
		repoAlias := fmt.Sprintf("r%d", repoIndex)
		definitions = append(definitions,
			fmt.Sprintf("$%s:String!", ownerVar),
			fmt.Sprintf("$%s:String!", nameVar),
		)
		variables[ownerVar] = repo.Owner
		variables[nameVar] = repo.Name
		fmt.Fprintf(&body, "%s:repository(owner:$%s,name:$%s){", repoAlias, ownerVar, nameVar)

		queryRepo := queryRepository{Alias: repoAlias, Key: repo.Key}
		for branchIndex, branch := range uniqueBranches(repo.Branches) {
			branchVar := fmt.Sprintf("branch%d_%d", repoIndex, branchIndex)
			branchAlias := fmt.Sprintf("b%d", branchIndex)
			definitions = append(definitions, fmt.Sprintf("$%s:String!", branchVar))
			variables[branchVar] = branch
			fmt.Fprintf(&body,
				"%s:pullRequests(headRefName:$%s,states:[OPEN,CLOSED,MERGED],first:1,orderBy:{field:CREATED_AT,direction:DESC}){nodes{number title state isDraft headRefName}}",
				branchAlias, branchVar,
			)
			queryRepo.Branches = append(queryRepo.Branches, queryBranch{
				Alias: branchAlias,
				Name:  branch,
			})
		}
		body.WriteString("}")
		queryRepos = append(queryRepos, queryRepo)
	}
	body.WriteString("rateLimit{cost}")

	return graphQLRequest{
		Query:     fmt.Sprintf("query(%s){%s}", strings.Join(definitions, ","), body.String()),
		Variables: variables,
	}, queryRepos
}

func parseBatchResponse(data []byte, repos []queryRepository) (map[string]map[string]*PR, error) {
	var response struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []graphQLError             `json:"errors"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		messages := make([]string, 0, len(response.Errors))
		for _, graphErr := range response.Errors {
			messages = append(messages, graphErr.Message)
		}
		return nil, fmt.Errorf("GraphQL: %s", strings.Join(messages, "; "))
	}

	result := make(map[string]map[string]*PR, len(repos))
	for _, repo := range repos {
		result[repo.Key] = make(map[string]*PR)
		var fields map[string]struct {
			Nodes []PR `json:"nodes"`
		}
		raw, ok := response.Data[repo.Alias]
		if !ok || string(raw) == "null" {
			continue
		}
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, err
		}
		for _, branch := range repo.Branches {
			connection := fields[branch.Alias]
			if len(connection.Nodes) == 0 {
				continue
			}
			pr := connection.Nodes[0]
			result[repo.Key][branch.Name] = &pr
		}
	}
	return result, nil
}
