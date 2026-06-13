package github

import (
	"context"
	"log/slog"

	"radar.nvim/internal/ingestion"
	"radar.nvim/internal/protocol"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

func (Service) Name() string {
	return "github"
}

func (Service) Status(ctx context.Context, logger *slog.Logger) ingestion.StatusResult {
	status, allowed := GraphQLServiceStatus(ctx, logger)
	return ingestion.StatusResult{Status: status, CanRun: allowed}
}

func (Service) Ingest(ctx context.Context, req ingestion.Request) ingestion.Result {
	result := ingestion.Result{
		Items: make([]protocol.Item, 0),
	}

	reviewItems, authoredItems, activityItems, err := FetchPullRequests(ctx, req.Previous, req.Logger)
	if err != nil {
		req.Logger.Warn("github pull request collection failed", "error", err)
		return result
	}

	result.Items = append(result.Items, reviewItems...)
	result.Items = append(result.Items, authoredItems...)
	result.Items = append(result.Items, activityItems...)

	trackedItems, err := FetchRulePullRequests(ctx, req.Filters, req.Logger)
	if err != nil {
		req.Logger.Warn("github rule pull request collection failed", "error", err)
	} else {
		result.Items = appendMissingPullRequests(result.Items, trackedItems)
	}

	result.Complete = true
	return result
}

func (Service) ReconcileDone(ctx context.Context, req ingestion.ReconcileRequest) []protocol.Item {
	return ResolveDonePullRequests(ctx, req.Previous, req.Active, req.Result.Complete, req.Logger)
}

func appendMissingPullRequests(items []protocol.Item, candidates []protocol.Item) []protocol.Item {
	seen := map[string]bool{}
	for _, item := range items {
		if item.URL != "" {
			seen[item.URL] = true
		}
	}
	for _, item := range candidates {
		if item.URL != "" && seen[item.URL] {
			continue
		}
		items = append(items, item)
		if item.URL != "" {
			seen[item.URL] = true
		}
	}
	return items
}
