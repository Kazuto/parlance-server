package translate

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// slowTranslator records how many requests it handles at once.
type slowTranslator struct {
	name    string
	fail    bool
	delay   time.Duration
	current atomic.Int32
	peak    atomic.Int32
	calls   atomic.Int32
}

func (s *slowTranslator) Translate(ctx context.Context, _ Request) (string, error) {
	s.calls.Add(1)
	now := s.current.Add(1)
	defer s.current.Add(-1)
	for {
		peak := s.peak.Load()
		if now <= peak || s.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if s.fail {
		return "", errors.New(s.name + " is down")
	}
	return s.name, nil
}

func poolOf(translators map[string]*slowTranslator, endpoints ...Endpoint) *Pool {
	return NewPool(endpoints, func(endpoint Endpoint) Translator {
		return translators[endpoint.ID]
	})
}

func TestPool(t *testing.T) {
	t.Run("keeps each server within its limit and uses all of them", func(t *testing.T) {
		a := &slowTranslator{name: "a", delay: 20 * time.Millisecond}
		b := &slowTranslator{name: "b", delay: 20 * time.Millisecond}
		pool := poolOf(map[string]*slowTranslator{"a": a, "b": b},
			Endpoint{ID: "a", MaxConcurrency: 2}, Endpoint{ID: "b", MaxConcurrency: 1})

		var wg sync.WaitGroup
		for range 12 {
			wg.Go(func() {
				if _, err := pool.Translate(context.Background(), Request{}); err != nil {
					t.Errorf("Translate: %v", err)
				}
			})
		}
		wg.Wait()

		if a.peak.Load() > 2 || b.peak.Load() > 1 {
			t.Errorf("peaks: a %d, b %d", a.peak.Load(), b.peak.Load())
		}
		if a.calls.Load() == 0 || b.calls.Load() == 0 || a.calls.Load()+b.calls.Load() != 12 {
			t.Errorf("calls: a %d, b %d", a.calls.Load(), b.calls.Load())
		}
	})

	t.Run("tries another server when one fails", func(t *testing.T) {
		down := &slowTranslator{name: "down", fail: true}
		up := &slowTranslator{name: "up"}
		pool := poolOf(map[string]*slowTranslator{"down": down, "up": up},
			Endpoint{ID: "down", MaxConcurrency: 4}, Endpoint{ID: "up", MaxConcurrency: 4})

		for range 10 {
			answer, err := pool.Translate(context.Background(), Request{})
			if err != nil || answer != "up" {
				t.Fatalf("got %q, %v", answer, err)
			}
		}
	})

	t.Run("reports the error when every server fails", func(t *testing.T) {
		a := &slowTranslator{name: "a", fail: true}
		b := &slowTranslator{name: "b", fail: true}
		pool := poolOf(map[string]*slowTranslator{"a": a, "b": b},
			Endpoint{ID: "a", MaxConcurrency: 1}, Endpoint{ID: "b", MaxConcurrency: 1})

		if _, err := pool.Translate(context.Background(), Request{}); err == nil {
			t.Fatal("expected an error")
		}
		if a.calls.Load() != 1 || b.calls.Load() != 1 {
			t.Errorf("calls: a %d, b %d", a.calls.Load(), b.calls.Load())
		}
	})

	t.Run("stops waiting when the request is canceled", func(t *testing.T) {
		busy := &slowTranslator{name: "busy", delay: time.Second}
		pool := poolOf(map[string]*slowTranslator{"busy": busy}, Endpoint{ID: "busy", MaxConcurrency: 1})
		go func() { _, _ = pool.Translate(context.Background(), Request{}) }()
		time.Sleep(10 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if _, err := pool.Translate(ctx, Request{}); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("got %v", err)
		}
	})
}

func TestPools(t *testing.T) {
	built := 0
	pools := NewPools(func(Endpoint) Translator {
		built++
		return &slowTranslator{}
	})

	if pools.Get(nil) != nil {
		t.Error("expected no pool without endpoints")
	}
	first := pools.Get([]Endpoint{{ID: "a", Version: "1"}})
	if pools.Get([]Endpoint{{ID: "a", Version: "1"}}) != first {
		t.Error("expected the same pool for the same settings")
	}
	if pools.Get([]Endpoint{{ID: "a", Version: "2"}}) == first {
		t.Error("expected a new pool after a change")
	}
	if built != 2 {
		t.Errorf("built %d translators, want 2", built)
	}
}
