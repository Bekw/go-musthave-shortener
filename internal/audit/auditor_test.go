package audit

import (
	"context"
	"errors"
	"testing"
)

type sinkFn func(ctx context.Context, e Event) error

func (f sinkFn) Write(ctx context.Context, e Event) error { return f(ctx, e) }

func TestAuditor_Publish_BestEffort(t *testing.T) {
	a := New()

	calls := 0
	a.Add(sinkFn(func(ctx context.Context, e Event) error {
		calls++
		return errors.New("fail")
	}))
	a.Add(sinkFn(func(ctx context.Context, e Event) error {
		calls++
		return nil
	}))

	a.Publish(context.Background(), Event{Action: "shorten", URL: "x"})

	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
