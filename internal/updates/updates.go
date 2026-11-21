// Package updates finds the newest releases of the server and the web app on
// GitHub, so those who run Parlance see when to update. It asks at most every 12
// hours, or an hour after a failed attempt, and keeps the answer in between.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/version"
)

// GitHubAPI is where the releases are asked for.
const GitHubAPI = "https://api.github.com"

// The repositories whose releases are the server's and the web app's.
const (
	serverRepository = "Kazuto/parlance-server"
	webRepository    = "Kazuto/parlance-client"
)

const (
	// How long an answer holds.
	checkEvery = 12 * time.Hour
	// How long a failed attempt waits before the next.
	retryAfter = time.Hour
	// How long an attempt may take; callers wait for it.
	timeout = 10 * time.Second
)

// Release is the newest release of one part.
type Release struct {
	// E.g. "2.1.0", without the tag's "v".
	Version string
	// The release notes on GitHub.
	URL         string
	PublishedAt time.Time
}

// Result is what the checker knows.
type Result struct {
	// The newest releases; empty until GitHub answered.
	Server, Web Release
	// When GitHub last answered; zero until it did.
	CheckedAt time.Time
	// Why the latest attempt failed, if it did. The releases are then those found
	// before, if any.
	Err error
}

// Checker asks GitHub for the newest releases and keeps the answer.
type Checker struct {
	baseURL string
	client  *http.Client
	now     func() time.Time

	mu        sync.Mutex
	result    Result
	attempted time.Time
}

// New returns a checker that asks the GitHub API at baseURL, see GitHubAPI.
func New(baseURL string, client *http.Client) *Checker {
	return &Checker{baseURL: strings.TrimSuffix(baseURL, "/"), client: client, now: time.Now}
}

// Latest returns the newest releases, asking GitHub when the last answer is too old.
// Callers wait for the attempt, which outlives their request so it isn't wasted.
func (c *Checker) Latest(ctx context.Context) Result {
	c.mu.Lock()
	defer c.mu.Unlock()

	wait := checkEvery
	if c.result.Err != nil {
		wait = retryAfter
	}

	if !c.attempted.IsZero() && c.now().Sub(c.attempted) < wait {
		return c.result
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	c.attempted = c.now()

	server, err := c.fetch(ctx, serverRepository)
	if err == nil {
		var web Release

		web, err = c.fetch(ctx, webRepository)
		if err == nil {
			c.result = Result{Server: server, Web: web, CheckedAt: c.attempted}

			return c.result
		}
	}

	c.result.Err = err

	return c.result
}

// fetch asks for a repository's newest release, leaving out drafts and pre-releases.
func (c *Checker) fetch(ctx context.Context, repository string) (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return Release{}, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	// GitHub turns away requests without one.
	req.Header.Set("User-Agent", "parlance-server/"+version.Version)

	res, err := c.client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("asking GitHub for the releases of %s: %w", repository, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %s for the releases of %s", res.Status, repository)
	}

	var body struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
	}

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("reading the releases of %s: %w", repository, err)
	}

	if body.TagName == "" {
		return Release{}, fmt.Errorf("the newest release of %s has no tag", repository)
	}

	return Release{
		Version:     strings.TrimPrefix(body.TagName, "v"),
		URL:         body.HTMLURL,
		PublishedAt: body.PublishedAt,
	}, nil
}
