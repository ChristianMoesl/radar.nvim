package filters

import (
	"testing"

	"radar.nvim/internal/protocol"
)

func TestApplyMutesRepos(t *testing.T) {
	items := []protocol.Item{
		{ID: "1", Repo: "org/noisy", Attention: "attention"},
		{ID: "2", Repo: "org/useful", Attention: "attention"},
	}

	got := Apply(items, Config{MuteRepos: []string{"ORG/NOISY"}})
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("expected only useful item, got %#v", got)
	}
}

func TestApplyDeprioritizesRepos(t *testing.T) {
	items := []protocol.Item{{ID: "1", Repo: "org/noisy", Attention: "attention", Reason: "review requested"}}

	got := Apply(items, Config{DeprioritizeRepos: []string{"org/noisy"}})
	if len(got) != 1 {
		t.Fatalf("expected one item, got %d", len(got))
	}
	if got[0].Attention != "low_priority" {
		t.Fatalf("expected low_priority, got %q", got[0].Attention)
	}
	if got[0].Reason != "low priority: review requested" {
		t.Fatalf("unexpected reason %q", got[0].Reason)
	}
}

func TestApplyMatchesUsers(t *testing.T) {
	items := []protocol.Item{
		{ID: "1", Attention: "attention", Metadata: map[string]string{"author": "dependabot[bot]"}},
		{ID: "2", Attention: "attention", Metadata: map[string]string{"author": "person"}},
	}

	got := Apply(items, Config{MuteUsers: []string{"dependabot[bot]"}})
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("expected only person item, got %#v", got)
	}
}

func TestSummaryIncludesLowPriority(t *testing.T) {
	summary := Summary([]protocol.Item{
		{Attention: "attention"},
		{Attention: "low_priority"},
	})
	if summary.Attention != 1 || summary.LowPriority != 1 {
		t.Fatalf("unexpected summary %#v", summary)
	}
}
