package collector

import (
	"context"
	"log/slog"

	"radar.nvim/internal/filters"
	gitcollector "radar.nvim/internal/git"
	"radar.nvim/internal/github"
	"radar.nvim/internal/ingestion"
	"radar.nvim/internal/jira"
	"radar.nvim/internal/linker"
	"radar.nvim/internal/protocol"
)

type Ingested struct {
	Items    []protocol.Item
	Entities []protocol.Entity
	Services []protocol.ServiceStatus
	Results  map[string]ingestion.Result
}

type Result struct {
	Items    []protocol.Item
	Services []protocol.ServiceStatus
}

func Sources() []ingestion.Source {
	return []ingestion.Source{
		github.NewService(),
		jira.NewService(),
		gitcollector.NewService(),
	}
}

func Collect(ctx context.Context, previous []protocol.Item, logger *slog.Logger) Result {
	sources := Sources()
	ingested := Ingest(ctx, previous, logger)
	items := linker.Link(linker.Input{
		Items:    ingested.Items,
		Entities: ingested.Entities,
	})
	for _, source := range sources {
		reconciler, ok := source.(ingestion.Reconciler)
		if !ok {
			continue
		}
		items = append(items, reconciler.ReconcileDone(ctx, ingestion.ReconcileRequest{
			Previous: previous,
			Active:   items,
			Result:   ingested.Results[source.Name()],
			Logger:   logger,
		})...)
	}
	return Result{Items: items, Services: ingested.Services}
}

func Ingest(ctx context.Context, previous []protocol.Item, logger *slog.Logger) Ingested {
	result := Ingested{
		Items:    make([]protocol.Item, 0),
		Entities: make([]protocol.Entity, 0),
		Services: make([]protocol.ServiceStatus, 0, 3),
		Results:  map[string]ingestion.Result{},
	}

	filterCfg, err := filters.Load()
	if err != nil {
		logger.Warn("could not load filters for ingestion", "error", err)
	}

	for _, source := range Sources() {
		status := ingestion.StatusResult{
			Status: protocol.ServiceStatus{Name: source.Name(), Status: "ok"},
			CanRun: true,
		}
		if reporter, ok := source.(ingestion.StatusReporter); ok {
			status = reporter.Status(ctx, logger)
		}
		result.Services = append(result.Services, status.Status)
		if !status.CanRun {
			continue
		}

		ingested := source.Ingest(ctx, ingestion.Request{
			Previous: previous,
			Filters:  filterCfg,
			Logger:   logger,
		})
		result.Results[source.Name()] = ingested
		result.Items = append(result.Items, ingested.Items...)
		result.Entities = append(result.Entities, ingested.Entities...)
	}

	return result
}
