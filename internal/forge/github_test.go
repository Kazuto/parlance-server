package forge_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/forge/forgetest"
)

func proposal(files ...forge.File) forge.Proposal {
	return forge.Proposal{
		BaseBranch:    "main",
		Branch:        "parlance/translations",
		CommitMessage: "Update translations",
		Title:         "Update translations",
		Body:          "From Parlance",
		Files:         files,
	}
}

func TestGitHubPropose(t *testing.T) {
	ctx := context.Background()
	fake := forgetest.NewGitHub(t, map[string]string{"README.md": "hi", "lang/en.json": "{}"})
	host := forge.NewHost(fake.Server.URL, fake.Token, nil)

	t.Run("opens a pull request with the changed files", func(t *testing.T) {
		result, err := host.Propose(ctx, fake.Repository, proposal(
			forge.File{Path: "lang/en.json", Content: "{}"},
			forge.File{Path: "lang/de.json", Content: `{"a":"b"}`},
		))
		if err != nil {
			t.Fatalf("Propose() error = %v", err)
		}

		if result.Outcome != forge.OutcomeUpdated || !slices.Equal(result.ChangedFiles, []string{"lang/de.json"}) {
			t.Fatalf("result = %+v, want updated with lang/de.json", result)
		}

		if !strings.HasSuffix(result.PullRequestURL, "/pull/1") {
			t.Errorf("pull request url = %q", result.PullRequestURL)
		}

		files := fake.Files("parlance/translations")
		if files["lang/de.json"] != `{"a":"b"}` || files["README.md"] != "hi" {
			t.Errorf("branch files = %v", files)
		}

		if fake.Files("main")["lang/de.json"] != "" {
			t.Error("the base branch changed")
		}
	})

	t.Run("leaves an up-to-date branch alone", func(t *testing.T) {
		before := len(fake.Calls())

		result, err := host.Propose(ctx, fake.Repository, proposal(forge.File{Path: "lang/de.json", Content: `{"a":"b"}`}))
		if err != nil {
			t.Fatalf("Propose() error = %v", err)
		}

		if result.Outcome != forge.OutcomeUpToDate || result.PullRequestURL == "" {
			t.Fatalf("result = %+v, want up to date with the open pull request", result)
		}

		for _, call := range fake.Calls()[before:] {
			if strings.HasPrefix(call, "POST /git/commits") || strings.HasPrefix(call, "PATCH") {
				t.Errorf("pushed although up to date: %s", call)
			}
		}
	})

	t.Run("rebuilds the branch and updates the open pull request", func(t *testing.T) {
		result, err := host.Propose(ctx, fake.Repository, proposal(forge.File{Path: "lang/de.json", Content: `{"a":"c"}`}))
		if err != nil {
			t.Fatalf("Propose() error = %v", err)
		}

		if result.Outcome != forge.OutcomeUpdated || len(fake.Pulls()) != 1 {
			t.Fatalf("result = %+v, pulls = %d, want one updated pull request", result, len(fake.Pulls()))
		}

		if got := fake.Files("parlance/translations")["lang/de.json"]; got != `{"a":"c"}` {
			t.Errorf("branch has %q", got)
		}
	})

	t.Run("does nothing once the base has the files", func(t *testing.T) {
		fake.SetFiles("main", map[string]string{"lang/de.json": `{"a":"c"}`})
		fake.ClosePulls()

		result, err := host.Propose(ctx, fake.Repository, proposal(forge.File{Path: "lang/de.json", Content: `{"a":"c"}`}))
		if err != nil {
			t.Fatalf("Propose() error = %v", err)
		}

		if result.Outcome != forge.OutcomeNoChanges || len(result.ChangedFiles) != 0 || len(fake.Pulls()) != 1 {
			t.Errorf("result = %+v, pulls = %d, want no changes and no new pull request", result, len(fake.Pulls()))
		}
	})

	t.Run("opens a new pull request after the last was closed", func(t *testing.T) {
		result, err := host.Propose(ctx, fake.Repository, proposal(forge.File{Path: "lang/de.json", Content: `{"a":"d"}`}))
		if err != nil {
			t.Fatalf("Propose() error = %v", err)
		}

		if !strings.HasSuffix(result.PullRequestURL, "/pull/2") {
			t.Errorf("pull request url = %q, want a second pull request", result.PullRequestURL)
		}
	})
}

func TestGitHubErrors(t *testing.T) {
	ctx := context.Background()
	fake := forgetest.NewGitHub(t, map[string]string{})

	_, err := forge.NewHost(fake.Server.URL, "wrong", nil).Propose(ctx, fake.Repository, proposal())
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("wrong token: error = %v", err)
	}

	host := forge.NewHost(fake.Server.URL, fake.Token, nil)

	_, err = host.Propose(ctx, fake.Repository, forge.Proposal{BaseBranch: "develop", Branch: "x"})
	if err == nil || !strings.Contains(err.Error(), "branch develop not found") {
		t.Errorf("missing base branch: error = %v", err)
	}

	if err := host.Check(ctx, fake.Repository, "main"); err != nil {
		t.Errorf("Check() error = %v", err)
	}

	if err := host.Check(ctx, "acme/other", "main"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Check(unknown repository) error = %v", err)
	}

	fake.CanPush = false
	if err := host.Check(ctx, fake.Repository, "main"); err == nil || !strings.Contains(err.Error(), "may not push") {
		t.Errorf("Check(read-only token) error = %v", err)
	}
}
