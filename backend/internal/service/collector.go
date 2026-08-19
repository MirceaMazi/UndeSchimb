package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/undeschimb/undeschimb/internal/domain"
	"github.com/undeschimb/undeschimb/internal/providers"
)

type CollectionStore interface {
	SaveCollection(context.Context, string, []domain.RateSnapshot, time.Time, time.Time, error) error
}

type Collector struct {
	providers []providers.Provider
	store     CollectionStore
	logger    *slog.Logger
	interval  time.Duration
}

func NewCollector(store CollectionStore, logger *slog.Logger, interval time.Duration, sources ...providers.Provider) *Collector {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &Collector{providers: sources, store: store, logger: logger, interval: interval}
}

func (c *Collector) Run(ctx context.Context) {
	c.Refresh(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Refresh(ctx)
		}
	}
}

func (c *Collector) Refresh(ctx context.Context) {
	var group sync.WaitGroup
	for _, source := range c.providers {
		source := source
		group.Add(1)
		go func() {
			defer group.Done()
			startedAt := time.Now().UTC()
			fetchContext, cancel := context.WithTimeout(ctx, 20*time.Second)
			snapshots, fetchErr := source.Fetch(fetchContext)
			cancel()
			completedAt := time.Now().UTC()
			if err := c.store.SaveCollection(ctx, source.ID(), snapshots, startedAt, completedAt, fetchErr); err != nil {
				c.logger.Error("could not persist collection run", "provider", source.ID(), "error", err)
				return
			}
			if fetchErr != nil {
				c.logger.Warn("rate source failed; serving its last valid snapshot", "provider", source.ID(), "error", fetchErr)
				return
			}
			c.logger.Info("rate source collected", "provider", source.ID(), "currencies", len(snapshots))
		}()
	}
	group.Wait()
}
