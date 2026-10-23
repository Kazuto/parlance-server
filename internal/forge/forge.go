// Package forge proposes file changes to repositories on git hosts, as pull requests.
package forge

import (
	"context"
	"crypto/sha1" //nolint:gosec // git names blobs by SHA-1
	"fmt"
	"net/http"
)

// File is a file's full content at its path in the repository.
type File struct {
	Path    string
	Content string
}

// Proposal is a set of files to change on a branch, offered for review against a base.
type Proposal struct {
	BaseBranch string
	// Branch is rebuilt from BaseBranch with the files, replacing what it held.
	Branch        string
	CommitMessage string
	Title         string
	Body          string
	Files         []File
}

// Outcomes of Propose
const (
	// The base branch already has the files.
	OutcomeNoChanges = "no_changes"
	// The branch already has the files.
	OutcomeUpToDate = "up_to_date"
	// The branch was rebuilt with the files.
	OutcomeUpdated = "updated"
)

// Result tells what Propose did.
type Result struct {
	Outcome string
	// ChangedFiles are the paths that differ from the base branch.
	ChangedFiles []string
	// PullRequestURL is the open pull request of the branch, if there is one.
	PullRequestURL string
}

// Host is a git host's API.
type Host interface {
	// Check tells whether the repository and its base branch can be read and pushed to.
	Check(ctx context.Context, repository, baseBranch string) error
	// Propose puts the files on the proposal's branch and opens a pull request for
	// it, or updates the open one.
	Propose(ctx context.Context, repository string, proposal Proposal) (Result, error)
}

// blobSHA is the id git gives a file with the content.
func blobSHA(content string) string {
	hash := sha1.New() //nolint:gosec // git names blobs by SHA-1
	fmt.Fprintf(hash, "blob %d\x00", len(content))
	hash.Write([]byte(content))

	return fmt.Sprintf("%x", hash.Sum(nil))
}

// NewHost returns the API of the host at apiURL; only GitHub for now.
func NewHost(apiURL, token string, client *http.Client) Host {
	return &GitHub{APIURL: apiURL, Token: token, Client: client}
}
