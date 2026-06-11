package linker

import (
	"testing"

	"radar.nvim/internal/protocol"
)

func TestLinkMatchesTicketKeysCaseInsensitivelyInBranch(t *testing.T) {
	items := []protocol.Item{
		{
			ID:        "github:own_pr:acme/app:42",
			Kind:      "github_own_pr",
			Title:     "Implement feature",
			Attention: "in_progress",
			Entities: []protocol.Entity{
				{
					ID:     "github:own_pr:acme/app:42",
					Source: "github",
					Kind:   "pull_request",
					Branch: "feature/dpscap-544-panel-deletion-navigation",
				},
			},
		},
	}
	entities := []protocol.Entity{
		{
			ID:     "jira:issue:DPSCAP-544",
			Source: "jira",
			Kind:   "issue",
			Title:  "DPSCAP-544 Fix navigation",
		},
	}

	linked := Link(Input{Items: items, Entities: entities})

	if len(linked) != 1 {
		t.Fatalf("linked item count = %d, want 1", len(linked))
	}
	if len(linked[0].Entities) != 2 {
		t.Fatalf("linked entity count = %d, want 2: %+v", len(linked[0].Entities), linked[0].Entities)
	}
	if linked[0].Entities[1].ID != "jira:issue:DPSCAP-544" {
		t.Fatalf("attached entity = %q, want jira issue", linked[0].Entities[1].ID)
	}
}

func TestLinkExtractsTicketKeysFromAnyMetadataValue(t *testing.T) {
	items := []protocol.Item{
		{
			ID:        "github:own_pr:acme/app:43",
			Kind:      "github_own_pr",
			Title:     "Implement feature",
			Attention: "in_progress",
			Entities: []protocol.Entity{
				{
					ID:       "github:own_pr:acme/app:43",
					Source:   "github",
					Kind:     "pull_request",
					Metadata: map[string]string{"custom_reference": "related to cap-123"},
				},
			},
		},
	}
	entities := []protocol.Entity{
		{
			ID:       "jira:issue:CAP-123",
			Source:   "jira",
			Kind:     "issue",
			Metadata: map[string]string{"key": "CAP-123"},
		},
	}

	linked := Link(Input{Items: items, Entities: entities})

	if len(linked) != 1 {
		t.Fatalf("linked item count = %d, want 1", len(linked))
	}
	if len(linked[0].Entities) != 2 {
		t.Fatalf("linked entity count = %d, want 2: %+v", len(linked[0].Entities), linked[0].Entities)
	}
}
