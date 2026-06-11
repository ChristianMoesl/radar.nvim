package github

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radar.nvim/internal/protocol"
)

func TestFetchPullRequestsClassifiesGraphQLResults(t *testing.T) {
	installFakeGH(t, `#!/bin/sh
case "$*" in
  *"api graphql"*)
    cat <<'JSON'
{
  "data": {
    "reviewRequested": {
      "nodes": [
        {
          "number": 12,
          "title": "Review me",
          "url": "https://github.com/acme/widgets/pull/12",
          "state": "OPEN",
          "isDraft": false,
          "repository": { "nameWithOwner": "acme/widgets" }
        }
      ]
    },
    "authored": {
      "nodes": [
        {
          "number": 34,
          "title": "My draft",
          "url": "https://github.com/acme/app/pull/34",
          "state": "OPEN",
          "isDraft": true,
          "repository": { "nameWithOwner": "acme/app" }
        }
      ]
    }
  }
}
JSON
    ;;
  *)
    echo "unexpected gh args: $*" >&2
    exit 1
    ;;
esac
`)

	reviewItems, authoredItems, err := FetchPullRequests(context.Background(), testLogger())
	if err != nil {
		t.Fatalf("FetchPullRequests() error = %v", err)
	}

	if len(reviewItems) != 1 {
		t.Fatalf("review item count = %d, want 1", len(reviewItems))
	}
	assertItem(t, reviewItems[0], protocol.Item{
		ID:        "github:review_request:acme/widgets:12",
		Kind:      "github_review_request",
		Title:     "Review me",
		Repo:      "acme/widgets",
		URL:       "https://github.com/acme/widgets/pull/12",
		Attention: "attention",
		Reason:    "review requested",
	})
	assertEntity(t, reviewItems[0].Entities, "github:review_request:acme/widgets:12", "review requested")

	if len(authoredItems) != 1 {
		t.Fatalf("authored item count = %d, want 1", len(authoredItems))
	}
	assertItem(t, authoredItems[0], protocol.Item{
		ID:        "github:own_pr:acme/app:34",
		Kind:      "github_own_pr",
		Title:     "My draft",
		Repo:      "acme/app",
		URL:       "https://github.com/acme/app/pull/34",
		Attention: "in_progress",
		Reason:    "draft PR",
	})
	assertEntity(t, authoredItems[0].Entities, "github:own_pr:acme/app:34", "draft PR")
}

func TestResolveDonePullRequestsSkipsRemoteFetchWhenAuthoredIncomplete(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "called")
	installFakeGH(t, `#!/bin/sh
touch "`+marker+`"
echo "gh should not be called" >&2
exit 1
`)

	today := time.Now().Format(time.RFC3339)
	yesterday := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	previous := []protocol.Item{
		{ID: "github:own_pr:acme/app:34", Kind: "github_own_pr", Attention: "in_progress", Title: "Still unknown"},
		{ID: "github:done_pr:acme/app:33", Kind: "github_done_pr", Attention: "done", DoneAt: today, Title: "Done today"},
		{ID: "github:done_pr:acme/app:32", Kind: "github_done_pr", Attention: "done", DoneAt: yesterday, Title: "Done yesterday"},
	}

	items := ResolveDonePullRequests(context.Background(), previous, nil, false, testLogger())
	if len(items) != 1 {
		t.Fatalf("resolved items = %d, want 1", len(items))
	}
	if items[0].ID != "github:done_pr:acme/app:33" {
		t.Fatalf("resolved item ID = %q, want today's done item", items[0].ID)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("gh was called even though authored collection was incomplete")
	}
}

func TestParseOwnPullRequestID(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantRepo   string
		wantNumber int
		wantOK     bool
	}{
		{name: "valid", id: "github:own_pr:acme/app:42", wantRepo: "acme/app", wantNumber: 42, wantOK: true},
		{name: "repo with colon", id: "github:own_pr:enterprise:acme/app:42", wantRepo: "enterprise:acme/app", wantNumber: 42, wantOK: true},
		{name: "wrong prefix", id: "github:review_request:acme/app:42"},
		{name: "missing number", id: "github:own_pr:acme/app:"},
		{name: "non numeric", id: "github:own_pr:acme/app:nope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepo, gotNumber, gotOK := parseOwnPullRequestID(tt.id)
			if gotOK != tt.wantOK || gotRepo != tt.wantRepo || gotNumber != tt.wantNumber {
				t.Fatalf("parseOwnPullRequestID(%q) = (%q, %d, %v), want (%q, %d, %v)", tt.id, gotRepo, gotNumber, gotOK, tt.wantRepo, tt.wantNumber, tt.wantOK)
			}
		})
	}
}

func TestAPPath(t *testing.T) {
	path, err := apiPath("https://api.github.com/repos/acme/app/pulls/42")
	if err != nil {
		t.Fatalf("apiPath() error = %v", err)
	}
	if path != "/repos/acme/app/pulls/42" {
		t.Fatalf("apiPath() = %q", path)
	}

	if _, err := apiPath("https://api.github.com"); err == nil {
		t.Fatalf("apiPath() error = nil, want error for URL without path")
	}
}

func TestReviewRequestedFromIsCaseInsensitive(t *testing.T) {
	pr := pullRequest{RequestedReviewers: []user{{Login: "ChristianMoesl"}}}
	if !reviewRequestedFrom("christianmoesl", pr) {
		t.Fatalf("reviewRequestedFrom() = false, want true")
	}
	if reviewRequestedFrom("someoneelse", pr) {
		t.Fatalf("reviewRequestedFrom() = true, want false")
	}
}

func installFakeGH(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertItem(t *testing.T, got protocol.Item, want protocol.Item) {
	t.Helper()
	if got.ID != want.ID || got.Kind != want.Kind || got.Title != want.Title || got.Repo != want.Repo || got.URL != want.URL || got.Attention != want.Attention || got.Reason != want.Reason {
		t.Fatalf("item = %+v, want matching %+v", got, want)
	}
}

func assertEntity(t *testing.T, entities []protocol.Entity, wantID string, wantStatus string) {
	t.Helper()
	if len(entities) != 1 {
		t.Fatalf("entity count = %d, want 1", len(entities))
	}
	entity := entities[0]
	if entity.ID != wantID || entity.Source != "github" || entity.Kind != "pull_request" || entity.Status != wantStatus {
		t.Fatalf("entity = %+v, want github pull_request %q status %q", entity, wantID, wantStatus)
	}
}
