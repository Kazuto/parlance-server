package forge

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DefaultGitHubAPI is GitHub's API; GitHub Enterprise has its own.
const DefaultGitHubAPI = "https://api.github.com"

// GitHub proposes changes through GitHub's REST API, without cloning: it builds the
// commit from trees and blobs, so only the changed files are sent.
type GitHub struct {
	APIURL string
	Token  string
	Client *http.Client
}

// APIError is an answer of the API other than success.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("GitHub answered %s %s with %d: %s", e.Method, e.Path, e.Status, e.Message)
}

func isNotFound(err error) bool {
	var apiError *APIError

	return stderrors.As(err, &apiError) && apiError.Status == http.StatusNotFound
}

// Check tells whether the repository and its base branch can be read, and the token
// may push to it.
func (g *GitHub) Check(ctx context.Context, repository, baseBranch string) error {
	var repo struct {
		Permissions *struct {
			Push bool `json:"push"`
		} `json:"permissions"`
	}

	if err := g.do(ctx, http.MethodGet, "/repos/"+repository, nil, &repo); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("repository %s not found, or the token can't see it", repository)
		}

		return err
	}

	if repo.Permissions == nil || !repo.Permissions.Push {
		return fmt.Errorf("the token may not push to %s", repository)
	}

	if _, err := g.branchHead(ctx, repository, baseBranch); err != nil {
		return err
	}

	return nil
}

type gitObject struct {
	SHA string `json:"sha"`
}

type treeEntry struct {
	Path    string  `json:"path"`
	Mode    string  `json:"mode,omitempty"`
	Type    string  `json:"type"`
	SHA     string  `json:"sha,omitempty"`
	Content *string `json:"content,omitempty"`
}

// Propose rebuilds the branch from the base with the files changed, and opens a
// pull request for it unless one is open. Nothing is pushed while the base or the
// branch already hold the files.
func (g *GitHub) Propose(ctx context.Context, repository string, proposal Proposal) (Result, error) {
	var result Result

	baseSHA, err := g.branchHead(ctx, repository, proposal.BaseBranch)
	if err != nil {
		return result, err
	}

	baseTree, err := g.commitTree(ctx, repository, baseSHA)
	if err != nil {
		return result, err
	}

	changed, err := g.changedFiles(ctx, repository, baseTree, proposal.Files)
	if err != nil {
		return result, err
	}

	for _, file := range changed {
		result.ChangedFiles = append(result.ChangedFiles, file.Path)
	}

	pullRequest, err := g.openPullRequest(ctx, repository, proposal)
	if err != nil {
		return result, err
	}

	if len(changed) == 0 {
		result.Outcome = OutcomeNoChanges
		result.PullRequestURL = pullRequest.HTMLURL

		return result, nil
	}

	entries := make([]treeEntry, len(changed))
	for i, file := range changed {
		entries[i] = treeEntry{Path: file.Path, Mode: "100644", Type: "blob", Content: &file.Content}
	}

	var tree gitObject
	if err := g.do(ctx, http.MethodPost, "/repos/"+repository+"/git/trees", map[string]any{
		"base_tree": baseTree,
		"tree":      entries,
	}, &tree); err != nil {
		return result, err
	}

	branchSHA, err := g.branchHead(ctx, repository, proposal.Branch)
	if err != nil && !isNotFound(err) {
		return result, err
	}

	if branchSHA != "" {
		branchTree, err := g.commitTree(ctx, repository, branchSHA)
		if err != nil {
			return result, err
		}

		// Unchanged since the last run: keep the branch, and a closed pull request closed.
		if branchTree == tree.SHA {
			result.Outcome = OutcomeUpToDate
			result.PullRequestURL = pullRequest.HTMLURL

			return result, nil
		}
	}

	var commit gitObject
	if err := g.do(ctx, http.MethodPost, "/repos/"+repository+"/git/commits", map[string]any{
		"message": proposal.CommitMessage,
		"tree":    tree.SHA,
		"parents": []string{baseSHA},
	}, &commit); err != nil {
		return result, err
	}

	if branchSHA == "" {
		err = g.do(ctx, http.MethodPost, "/repos/"+repository+"/git/refs", map[string]any{
			"ref": "refs/heads/" + proposal.Branch,
			"sha": commit.SHA,
		}, nil)
	} else {
		err = g.do(ctx, http.MethodPatch, "/repos/"+repository+"/git/refs/heads/"+escapePath(proposal.Branch), map[string]any{
			"sha":   commit.SHA,
			"force": true,
		}, nil)
	}

	if err != nil {
		return result, err
	}

	result.Outcome = OutcomeUpdated

	if pullRequest.Number != 0 {
		err = g.do(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/pulls/%d", repository, pullRequest.Number), map[string]any{
			"title": proposal.Title,
			"body":  proposal.Body,
		}, nil)
		result.PullRequestURL = pullRequest.HTMLURL

		return result, err
	}

	var created pullRequestInfo
	if err := g.do(ctx, http.MethodPost, "/repos/"+repository+"/pulls", map[string]any{
		"title": proposal.Title,
		"body":  proposal.Body,
		"head":  proposal.Branch,
		"base":  proposal.BaseBranch,
	}, &created); err != nil {
		return result, err
	}

	result.PullRequestURL = created.HTMLURL

	return result, nil
}

// branchHead returns the commit a branch points to.
func (g *GitHub) branchHead(ctx context.Context, repository, branch string) (string, error) {
	var ref struct {
		Object gitObject `json:"object"`
	}

	err := g.do(ctx, http.MethodGet, "/repos/"+repository+"/git/ref/heads/"+escapePath(branch), nil, &ref)

	var apiError *APIError
	if stderrors.As(err, &apiError) && apiError.Status == http.StatusNotFound {
		apiError.Message = "branch " + branch + " not found"
	}

	if err != nil {
		return "", err
	}

	return ref.Object.SHA, nil
}

// commitTree returns the tree of a commit.
func (g *GitHub) commitTree(ctx context.Context, repository, sha string) (string, error) {
	var commit struct {
		Tree gitObject `json:"tree"`
	}

	if err := g.do(ctx, http.MethodGet, "/repos/"+repository+"/git/commits/"+sha, nil, &commit); err != nil {
		return "", err
	}

	return commit.Tree.SHA, nil
}

// changedFiles returns the files whose content differs from the tree. Without the
// whole tree, which GitHub cuts off for huge repositories, every file counts as changed.
func (g *GitHub) changedFiles(ctx context.Context, repository, treeSHA string, files []File) ([]File, error) {
	var tree struct {
		Tree      []treeEntry `json:"tree"`
		Truncated bool        `json:"truncated"`
	}

	if err := g.do(ctx, http.MethodGet, "/repos/"+repository+"/git/trees/"+treeSHA+"?recursive=1", nil, &tree); err != nil {
		return nil, err
	}

	if tree.Truncated {
		return files, nil
	}

	existing := make(map[string]string, len(tree.Tree))

	for _, entry := range tree.Tree {
		if entry.Type == "blob" {
			existing[entry.Path] = entry.SHA
		}
	}

	var changed []File

	for _, file := range files {
		if existing[file.Path] != blobSHA(file.Content) {
			changed = append(changed, file)
		}
	}

	return changed, nil
}

type pullRequestInfo struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// openPullRequest returns the open pull request from the branch into the base, or
// an empty one without.
func (g *GitHub) openPullRequest(ctx context.Context, repository string, proposal Proposal) (pullRequestInfo, error) {
	owner, _, _ := strings.Cut(repository, "/")
	query := url.Values{
		"head":  {owner + ":" + proposal.Branch},
		"base":  {proposal.BaseBranch},
		"state": {"open"},
	}

	var pulls []pullRequestInfo
	if err := g.do(ctx, http.MethodGet, "/repos/"+repository+"/pulls?"+query.Encode(), nil, &pulls); err != nil {
		return pullRequestInfo{}, err
	}

	if len(pulls) == 0 {
		return pullRequestInfo{}, nil
	}

	return pulls[0], nil
}

// do sends a request to the API and decodes the answer into out, if given.
func (g *GitHub) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(encoded)
	}

	apiURL := g.APIURL
	if apiURL == "" {
		apiURL = DefaultGitHubAPI
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(apiURL, "/")+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= http.StatusBadRequest {
		var answer struct {
			Message string `json:"message"`
		}

		// Without a readable message, the status still says what went wrong.
		if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&answer); err != nil {
			answer.Message = http.StatusText(res.StatusCode)
		}

		pathOnly, _, _ := strings.Cut(path, "?")

		return &APIError{Method: method, Path: pathOnly, Status: res.StatusCode, Message: answer.Message}
	}

	if out == nil {
		return nil
	}

	return json.NewDecoder(res.Body).Decode(out)
}

// escapePath escapes each segment of a slash-separated name, such as a branch.
func escapePath(name string) string {
	segments := strings.Split(name, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return strings.Join(segments, "/")
}
