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
	Items                  []protocol.Item
	Entities               []protocol.Entity
	GitHubAuthoredComplete bool
}

func Collect(ctx context.Context, previous []protocol.Item, logger *slog.Logger) []protocol.Item {
	ingested := Ingest(ctx, logger)
	items := linker.Link(linker.Input{
		Items:    ingested.Items,
		Entities: ingested.Entities,
	})
	items = append(items, github.ResolveDonePullRequests(ctx, previous, items, ingested.GitHubAuthoredComplete, logger)...)
	return items
}

func Ingest(ctx context.Context, logger *slog.Logger) Ingested {
	result := Ingested{
		Items:    make([]protocol.Item, 0),
		Entities: make([]protocol.Entity, 0),
	}

	if github.EnsureGraphQLBudget(ctx, logger) {
		reviewItems, authoredItems, err := github.FetchPullRequests(ctx, logger)
		if err != nil {
			logger.Warn("github pull request collection failed", "error", err)
		} else {
			result.Items = append(result.Items, reviewItems...)
			result.Items = append(result.Items, authoredItems...)
			result.GitHubAuthoredComplete = true
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
