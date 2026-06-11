package collector

import (
	"context"
	"log/slog"

	gitcollector "radar.nvim/internal/git"
	"radar.nvim/internal/github"
	"radar.nvim/internal/jira"
	"radar.nvim/internal/linker"
	"radar.nvim/internal/protocol"
)

type Ingested struct {
	Items    []protocol.Item
	Entities []protocol.Entity
}

func Collect(ctx context.Context, previous []protocol.Item, logger *slog.Logger) []protocol.Item {
	ingested := Ingest(ctx, logger)
	items := linker.Link(linker.Input{
		Items:    ingested.Items,
		Entities: ingested.Entities,
	})
	items = append(items, github.ResolveDonePullRequests(ctx, previous, items, logger)...)
	return items
}

func Ingest(ctx context.Context, logger *slog.Logger) Ingested {
	result := Ingested{
		Items:    make([]protocol.Item, 0),
		Entities: make([]protocol.Entity, 0),
	}

	if github.EnsureSearchBudget(ctx, logger) {
		reviewItems, err := github.FetchReviewRequests(ctx, logger)
		if err != nil {
			logger.Warn("github review request collection failed", "error", err)
		} else {
			result.Items = append(result.Items, reviewItems...)
		}
	}

	if github.EnsureSearchBudget(ctx, logger) {
		authoredItems, err := github.FetchAuthoredPullRequests(ctx, logger)
		if err != nil {
			logger.Warn("github authored PR collection failed", "error", err)
		} else {
			result.Items = append(result.Items, authoredItems...)
		}
	}

	jiraEntities, err := jira.FetchAssignedIssues(ctx, logger)
	if err != nil {
		logger.Warn("jira issue collection failed", "error", err)
	} else {
		result.Entities = append(result.Entities, jiraEntities...)
	}

	result.Entities = append(result.Entities, gitcollector.FetchWorktrees(ctx, logger)...)
	return result
}
