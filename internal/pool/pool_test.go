package pool_test

import (
	"runtime/debug"
	"testing"

	"github.com/Bekw/go-musthave-shortener/internal/pool"
)

type obj struct {
	I int
	S []string
	M map[string]string

	resetCalls int
}

func (o *obj) Reset() {
	o.resetCalls++
	o.I = 0
	o.S = o.S[:0]
	clear(o.M)
}

func TestPool_PutCallsReset(t *testing.T) {
	p := pool.New(func() *obj {
		return &obj{
			S: make([]string, 0, 8),
			M: make(map[string]string),
		}
	})

	x := p.Get()
	x.I = 123
	x.S = append(x.S, "a", "b")
	x.M["k"] = "v"

	p.Put(x)

	if x.resetCalls != 1 {
		t.Fatalf("expected resetCalls=1, got %d", x.resetCalls)
	}
	if x.I != 0 {
		t.Fatalf("expected I=0, got %d", x.I)
	}
	if len(x.S) != 0 {
		t.Fatalf("expected len(S)=0, got %d", len(x.S))
	}
	if len(x.M) != 0 {
		t.Fatalf("expected len(M)=0, got %d", len(x.M))
	}
}

func TestPool_GetReusesAfterPut_NoGC(t *testing.T) {
	old := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(old) })

	newCalls := 0
	p := pool.New(func() *obj {
		newCalls++
		return &obj{M: make(map[string]string)}
	})

	a := p.Get()
	if newCalls != 1 {
		t.Fatalf("expected newCalls=1, got %d", newCalls)
	}

	p.Put(a)

	b := p.Get()

	if newCalls != 1 {
		t.Fatalf("expected newCalls=1 after reuse, got %d", newCalls)
	}
	if a != b {
		t.Fatalf("expected same pointer from pool reuse")
	}
}

func TestPool_PutNilIgnored(t *testing.T) {
	old := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(old) })

	newCalls := 0
	p := pool.New(func() *obj {
		newCalls++
		return &obj{M: make(map[string]string)}
	})

	p.Put(nil)

	_ = p.Get()
	if newCalls != 1 {
		t.Fatalf("expected newCalls=1, got %d", newCalls)
	}
}
