package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/laddawan/backend-test/usecase"
)

// countFunc adapts a function to usecase.UserCountReader.
type countFunc func(ctx context.Context) (int64, error)

func (f countFunc) Count(ctx context.Context) (int64, error) { return f(ctx) }

func TestUserCounter_TicksAndStops(t *testing.T) {
	calls := make(chan struct{}, 10)
	counter := countFunc(func(context.Context) (int64, error) {
		calls <- struct{}{}
		return 3, nil
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	uc := usecase.NewUserCounter(counter, 5*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		uc.Run(ctx)
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-calls:
		case <-time.After(time.Second):
			t.Fatalf("expected Count to be called on each tick (got %d calls)", i)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestUserCounter_KeepsRunningOnError(t *testing.T) {
	calls := make(chan struct{}, 10)
	counter := countFunc(func(context.Context) (int64, error) {
		calls <- struct{}{}
		return 0, errors.New("database down")
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	uc := usecase.NewUserCounter(counter, 5*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go uc.Run(ctx)

	for i := 0; i < 2; i++ {
		select {
		case <-calls:
		case <-time.After(time.Second):
			t.Fatalf("counter stopped after an error (got %d calls)", i)
		}
	}
}
