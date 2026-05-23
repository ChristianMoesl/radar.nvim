package collector

import (
	"context"
	"log/slog"

	"cockpit.nvim/internal/github"
	"cockpit.nvim/internal/protocol"
)

func Collect(ctx context.Context, previous []protocol.Item, logger *slog.Logger) []protocol.Item {
	items := make([]protocol.Item, 0)

	reviewItems, err := github.FetchReviewRequests(ctx, logger)
	if err != nil {
		logger.Warn("github review request collection failed", "error", err)
	} else {
		items = append(items, reviewItems...)
	}

	authoredItems, err := github.FetchAuthoredPullRequests(ctx, logger)
	if err != nil {
		logger.Warn("github authored PR collection failed", "error", err)
	} else {
		items = append(items, authoredItems...)
	}

	items = append(items, github.ResolveDonePullRequests(ctx, previous, items, logger)...)

	return items
}
