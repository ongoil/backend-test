package usecase

import (
	"context"
	"log/slog"
	"time"
)

// UserCounter periodically logs the total number of users.
type UserCounter struct {
	counter  UserCountReader
	interval time.Duration
	timeout  time.Duration
	logger   *slog.Logger
}

// NewUserCounter creates a UserCounter that logs every interval.
func NewUserCounter(counter UserCountReader, interval time.Duration, logger *slog.Logger) *UserCounter {
	return &UserCounter{
		counter:  counter,
		interval: interval,
		timeout:  5 * time.Second,
		logger:   logger,
	}
}

// Run blocks until ctx is cancelled, logging the user count on every tick.
// It is meant to be started in its own goroutine.
func (c *UserCounter) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Info("user counter started", "interval", c.interval.String())
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("user counter stopped")
			return
		case <-ticker.C:
			c.logCount(ctx)
		}
	}
}

func (c *UserCounter) logCount(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()

	n, err := c.counter.Count(ctx)
	if err != nil {
		if parent.Err() != nil {
			return // shutting down; not a real failure
		}
		c.logger.Error("count users failed", "error", err)
		return
	}
	c.logger.Info("total users", "count", n)
}
