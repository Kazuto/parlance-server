package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitHub answers for both repositories, or fails while down is set.
func fakeGitHub(t *testing.T, serverTag string, down *atomic.Bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var requests atomic.Int32

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		if r.Header.Get("User-Agent") == "" {
			t.Error("request without a User-Agent")
		}

		if down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)

			return
		}

		tag := "v1.3.0"
		if strings.Contains(r.URL.Path, serverRepository) {
			tag = serverTag
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "` + tag + `", "html_url": "https://github.com/` + r.URL.Path + `", "published_at": "2026-11-01T10:00:00Z"}`))
	}))
	t.Cleanup(github.Close)

	return github, &requests
}

func TestLatestAsksOnceAndKeepsTheAnswer(t *testing.T) {
	var down atomic.Bool

	github, requests := fakeGitHub(t, "v2.1.0", &down)
	checker := New(github.URL, github.Client())
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	got := checker.Latest(context.Background())
	if got.Err != nil {
		t.Fatalf("Latest() error = %v", got.Err)
	}

	if got.Server.Version != "2.1.0" || got.Web.Version != "1.3.0" {
		t.Errorf("Latest() = %+v, want server 2.1.0 and web 1.3.0", got)
	}

	if !got.CheckedAt.Equal(now) || got.Server.PublishedAt.IsZero() || !strings.Contains(got.Web.URL, webRepository) {
		t.Errorf("Latest() = %+v, want the check time, publish dates and release links", got)
	}

	// Within 12 hours the answer holds.
	now = now.Add(11 * time.Hour)

	checker.Latest(context.Background())

	if n := requests.Load(); n != 2 {
		t.Errorf("GitHub was asked %d times, want 2, once per repository", n)
	}

	now = now.Add(2 * time.Hour)

	checker.Latest(context.Background())

	if n := requests.Load(); n != 4 {
		t.Errorf("GitHub was asked %d times after 13 hours, want 4", n)
	}
}

func TestLatestKeepsTheReleasesWhenGitHubFails(t *testing.T) {
	var down atomic.Bool

	github, requests := fakeGitHub(t, "v2.1.0", &down)
	checker := New(github.URL, github.Client())
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	checked := now
	checker.now = func() time.Time { return now }

	checker.Latest(context.Background())

	down.Store(true)

	now = now.Add(13 * time.Hour)

	got := checker.Latest(context.Background())
	if got.Err == nil || !strings.Contains(got.Err.Error(), "503") {
		t.Errorf("Latest() error = %v, want GitHub's answer", got.Err)
	}

	if got.Server.Version != "2.1.0" || !got.CheckedAt.Equal(checked) {
		t.Errorf("Latest() = %+v, want the releases found before and when", got)
	}

	// A failed attempt waits an hour, not twelve.
	asked := requests.Load()
	now = now.Add(30 * time.Minute)

	checker.Latest(context.Background())

	if requests.Load() != asked {
		t.Error("GitHub was asked again within the hour")
	}

	down.Store(false)

	now = now.Add(time.Hour)

	if got := checker.Latest(context.Background()); got.Err != nil || !got.CheckedAt.Equal(now) {
		t.Errorf("Latest() after GitHub is back = %+v, want a fresh answer", got)
	}
}

func TestLatestReportsAReleaseWithoutTag(t *testing.T) {
	var down atomic.Bool

	github, _ := fakeGitHub(t, "", &down)

	got := New(github.URL, github.Client()).Latest(context.Background())
	if got.Err == nil || got.Server.Version != "" || !got.CheckedAt.IsZero() {
		t.Errorf("Latest() = %+v, want an error and nothing found", got)
	}
}
