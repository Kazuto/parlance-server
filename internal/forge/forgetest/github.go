// Package forgetest fakes a git host's API for tests.
package forgetest

import (
	"crypto/sha1" //nolint:gosec // git names objects by SHA-1
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// PullRequest is a pull request on the fake host.
type PullRequest struct {
	Number int
	Head   string
	Base   string
	Title  string
	Body   string
	Open   bool
}

// GitHub fakes the parts of GitHub's REST API that forge.GitHub uses, for one
// repository with its files kept in memory.
type GitHub struct {
	Server     *httptest.Server
	Repository string
	Token      string
	// CanPush is what the repository reports about the token.
	CanPush bool

	mu      sync.Mutex
	trees   map[string]map[string]string // tree sha -> path -> content
	commits map[string]string            // commit sha -> tree sha
	refs    map[string]string            // branch -> commit sha
	pulls   []*PullRequest
	// Calls lists "METHOD /path" of each request, without the repository prefix.
	calls []string
}

// NewGitHub starts a fake with the repository "acme/app", whose base branch "main"
// has the files. It stops with the test.
func NewGitHub(t *testing.T, files map[string]string) *GitHub {
	t.Helper()

	g := &GitHub{
		Repository: "acme/app",
		Token:      "secret-token",
		CanPush:    true,
		trees:      map[string]map[string]string{},
		commits:    map[string]string{},
		refs:       map[string]string{},
	}

	tree := g.addTree(files)
	g.refs["main"] = g.addCommit(tree, "initial")

	g.Server = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.Server.Close)

	return g
}

// Files returns the files on a branch, or nil without the branch.
func (g *GitHub) Files(branch string) map[string]string {
	g.mu.Lock()
	defer g.mu.Unlock()

	commit, ok := g.refs[branch]
	if !ok {
		return nil
	}

	return g.trees[g.commits[commit]]
}

// SetFiles commits the files to the branch, as a merge or a push by someone would.
func (g *GitHub) SetFiles(branch string, files map[string]string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.refs[branch] = g.addCommit(g.addTree(files), branch)
}

// Pulls returns the pull requests.
func (g *GitHub) Pulls() []PullRequest {
	g.mu.Lock()
	defer g.mu.Unlock()

	pulls := make([]PullRequest, len(g.pulls))
	for i, pull := range g.pulls {
		pulls[i] = *pull
	}

	return pulls
}

// ClosePulls closes every pull request.
func (g *GitHub) ClosePulls() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, pull := range g.pulls {
		pull.Open = false
	}
}

// Calls returns the requests so far, as "METHOD /path".
func (g *GitHub) Calls() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return slices.Clone(g.calls)
}

func hash(parts ...string) string {
	h := sha1.New() //nolint:gosec // only an id
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func blobSHA(content string) string {
	h := sha1.New() //nolint:gosec // git names blobs by SHA-1
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write([]byte(content))

	return fmt.Sprintf("%x", h.Sum(nil))
}

func (g *GitHub) addTree(files map[string]string) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}

	slices.Sort(paths)

	parts := []string{"tree"}
	for _, path := range paths {
		parts = append(parts, path, files[path])
	}

	sha := hash(parts...)
	g.trees[sha] = files

	return sha
}

func (g *GitHub) addCommit(tree, message string) string {
	sha := hash("commit", tree, message, strconv.Itoa(len(g.commits)))
	g.commits[sha] = tree

	return sha
}

func (g *GitHub) serve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer "+g.Token {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})

		return
	}

	prefix := "/repos/" + g.Repository
	path := strings.TrimPrefix(r.URL.Path, prefix)

	if !strings.HasPrefix(r.URL.Path, prefix) {
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})

		return
	}

	g.calls = append(g.calls, r.Method+" "+path)

	var body map[string]any
	if r.Body != nil && r.Method != http.MethodGet {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	switch {
	case r.Method == http.MethodGet && path == "":
		writeJSON(w, http.StatusOK, map[string]any{"permissions": map[string]bool{"push": g.CanPush}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/git/ref/heads/"):
		branch := strings.TrimPrefix(path, "/git/ref/heads/")
		if sha, ok := g.refs[branch]; ok {
			writeJSON(w, http.StatusOK, map[string]any{"object": map[string]string{"sha": sha}})
		} else {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
		}
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/git/commits/"):
		tree, ok := g.commits[strings.TrimPrefix(path, "/git/commits/")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})

			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"tree": map[string]string{"sha": tree}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/git/trees/"):
		files, ok := g.trees[strings.TrimPrefix(path, "/git/trees/")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})

			return
		}

		entries := []map[string]string{}
		for path, content := range files {
			entries = append(entries, map[string]string{"path": path, "type": "blob", "sha": blobSHA(content)})
		}

		writeJSON(w, http.StatusOK, map[string]any{"tree": entries, "truncated": false})
	case r.Method == http.MethodPost && path == "/git/trees":
		files := map[string]string{}
		for path, content := range g.trees[body["base_tree"].(string)] {
			files[path] = content
		}

		for _, entry := range body["tree"].([]any) {
			entry := entry.(map[string]any)
			files[entry["path"].(string)] = entry["content"].(string)
		}

		writeJSON(w, http.StatusCreated, map[string]string{"sha": g.addTree(files)})
	case r.Method == http.MethodPost && path == "/git/commits":
		writeJSON(w, http.StatusCreated, map[string]string{"sha": g.addCommit(body["tree"].(string), body["message"].(string))})
	case r.Method == http.MethodPost && path == "/git/refs":
		branch := strings.TrimPrefix(body["ref"].(string), "refs/heads/")
		if _, ok := g.refs[branch]; ok {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"message": "Reference already exists"})

			return
		}

		g.refs[branch] = body["sha"].(string)

		writeJSON(w, http.StatusCreated, map[string]any{})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/git/refs/heads/"):
		g.refs[strings.TrimPrefix(path, "/git/refs/heads/")] = body["sha"].(string)

		writeJSON(w, http.StatusOK, map[string]any{})
	case r.Method == http.MethodGet && path == "/pulls":
		owner, _, _ := strings.Cut(g.Repository, "/")
		query := r.URL.Query()
		found := []map[string]any{}

		for _, pull := range g.pulls {
			if pull.Open && owner+":"+pull.Head == query.Get("head") && pull.Base == query.Get("base") {
				found = append(found, g.pullJSON(pull))
			}
		}

		writeJSON(w, http.StatusOK, found)
	case r.Method == http.MethodPost && path == "/pulls":
		pull := &PullRequest{
			Number: len(g.pulls) + 1,
			Head:   body["head"].(string),
			Base:   body["base"].(string),
			Title:  body["title"].(string),
			Body:   body["body"].(string),
			Open:   true,
		}
		g.pulls = append(g.pulls, pull)
		writeJSON(w, http.StatusCreated, g.pullJSON(pull))
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/pulls/"):
		number, _ := strconv.Atoi(strings.TrimPrefix(path, "/pulls/"))
		for _, pull := range g.pulls {
			if pull.Number == number {
				pull.Title = body["title"].(string)
				pull.Body = body["body"].(string)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
	}
}

func (g *GitHub) pullJSON(pull *PullRequest) map[string]any {
	return map[string]any{
		"number":   pull.Number,
		"html_url": fmt.Sprintf("https://github.example/%s/pull/%d", g.Repository, pull.Number),
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
