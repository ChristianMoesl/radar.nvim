package collector

import (
	"context"
	"log/slog"

	"cockpit.nvim/internal/github"
	"cockpit.nvim/internal/protocol"
)

func Collect(ctx context.Context, logger *slog.Logger) []protocol.Item {
	items := make([]protocol.Item, 0)

	githubItems, err := github.FetchReviewRequests(ctx, logger)
	if err != nil {
		logger.Warn("github collection failed", "error", err)
	} else {
		items = append(items, githubItems...)
	}

	return items
}
