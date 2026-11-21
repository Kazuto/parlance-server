package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/updates"
)

func getUpdates(t *testing.T, checker *updates.Checker) *pb.GetUpdatesResponse {
	t.Helper()

	res, err := NewServer(checker).GetUpdates(context.Background(), connect.NewRequest(&pb.GetUpdatesRequest{}))
	if err != nil {
		t.Fatalf("GetUpdates() error = %v", err)
	}

	return res.Msg
}

func TestGetUpdatesWhileTheCheckIsOff(t *testing.T) {
	if got := getUpdates(t, nil); got.Enabled || got.Server != nil || got.Web != nil {
		t.Errorf("GetUpdates() = %v, want nothing while off", got)
	}
}

func TestGetUpdatesReturnsTheNewestReleases(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag := "v1.3.0"
		if strings.Contains(r.URL.Path, "parlance-server") {
			tag = "v2.1.0"
		}

		_, _ = w.Write([]byte(`{"tag_name": "` + tag + `", "html_url": "https://github.com/releases/` + tag + `", "published_at": "2026-11-01T10:00:00Z"}`))
	}))
	defer github.Close()

	got := getUpdates(t, updates.New(github.URL, github.Client()))

	if !got.Enabled || got.Error != "" || got.CheckedAt == nil {
		t.Errorf("GetUpdates() = %v, want a successful check", got)
	}

	if got.Server.GetVersion() != "2.1.0" || got.Web.GetVersion() != "1.3.0" || got.Web.GetUrl() != "https://github.com/releases/v1.3.0" || got.Server.GetPublishedAt() != "2026-11-01T10:00:00Z" {
		t.Errorf("GetUpdates() = %v, want server 2.1.0 and web 1.3.0 with their notes", got)
	}
}

func TestGetUpdatesReportsAFailedCheck(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer github.Close()

	got := getUpdates(t, updates.New(github.URL, github.Client()))

	if !got.Enabled || !strings.Contains(got.Error, "403") || got.Server != nil || got.CheckedAt != nil {
		t.Errorf("GetUpdates() = %v, want the failure and no releases", got)
	}
}
