package backfills

import (
	"context"
	"fmt"

	"github.com/sjperalta/fintera-api/internal/services"
)

// Dependencies carries the services used by built-in backfill tasks.
type Dependencies struct {
	HistoricalSale *services.HistoricalSaleService
}

// DefaultRegistry returns the backfills known to this deployment.
func DefaultRegistry(deps Dependencies) *Registry {
	registry := NewRegistry()
	if deps.HistoricalSale != nil {
		_ = registry.Register(Task{
			Name:        "historical_clients",
			Version:     "1",
			Description: "Link historical sales to customer accounts",
			Run: func(ctx context.Context) (Result, error) {
				outcome, err := deps.HistoricalSale.BackfillClients(ctx, 200)
				if err != nil {
					return Result{}, err
				}
				details := fmt.Sprintf(
					"linked=%d skipped=%d errors=%d",
					outcome.Linked,
					outcome.Skipped,
					outcome.Errors,
				)
				for _, message := range outcome.Messages {
					details += "; " + message
				}
				return Result{
					RowsAffected: int64(outcome.Linked),
					Details:      truncateDetails(details),
				}, nil
			},
		})
	}
	return registry
}

func truncateDetails(value string) string {
	const max = 4000
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
