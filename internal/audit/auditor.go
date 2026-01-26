package audit

import (
	"context"
	"sync"
)

type Auditor struct {
	mu    sync.RWMutex
	sinks []Sink
}

func New() *Auditor { return &Auditor{} }

func (a *Auditor) Add(s Sink) {
	if s == nil {
		return
	}
	a.mu.Lock()
	a.sinks = append(a.sinks, s)
	a.mu.Unlock()
}

func (a *Auditor) Publish(ctx context.Context, e Event) {
	a.mu.RLock()
	sinks := append([]Sink(nil), a.sinks...)
	a.mu.RUnlock()

	for _, s := range sinks {
		_ = s.Write(ctx, e)
	}
}
