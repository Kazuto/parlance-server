package translate

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"
)

// RequestTimeout limits how long one request to a translation server may take.
const RequestTimeout = 60 * time.Second

// Endpoint is one server running a translation model.
type Endpoint struct {
	ID      string
	BaseURL string
	Model   string
	APIKey  string
	// MaxConcurrency is how many requests the server takes at once.
	MaxConcurrency int
	// Version changes whenever the settings change, e.g. the update time.
	Version string
}

// NewClientFor creates the client for an endpoint.
func NewClientFor(endpoint Endpoint) Translator {
	return NewClient(endpoint.BaseURL, endpoint.Model, endpoint.APIKey, RequestTimeout)
}

type member struct {
	translator Translator
	// slots holds a value per request in progress.
	slots chan struct{}
}

// Pool spreads requests over several servers. Each server takes at most its
// MaxConcurrency requests at once; a request waits until any server has room. A
// failed request is tried again on each server that hasn't failed it yet.
type Pool struct {
	members []*member
}

// NewPool creates a pool of the endpoints, with a translator from newTranslator.
func NewPool(endpoints []Endpoint, newTranslator func(Endpoint) Translator) *Pool {
	pool := &Pool{}
	for _, endpoint := range endpoints {
		pool.members = append(pool.members, &member{
			translator: newTranslator(endpoint),
			slots:      make(chan struct{}, max(endpoint.MaxConcurrency, 1)),
		})
	}
	return pool
}

func (p *Pool) Translate(ctx context.Context, req Request) (string, error) {
	tried := make([]bool, len(p.members))
	var lastErr error
	for range p.members {
		index, err := p.acquire(ctx, tried)
		if err != nil {
			return "", err
		}
		answer, err := p.members[index].translator.Translate(ctx, req)
		<-p.members[index].slots
		if err == nil {
			return answer, nil
		}
		if ctx.Err() != nil {
			return "", err
		}
		tried[index] = true
		lastErr = err
	}
	return "", lastErr
}

// acquire waits for a free slot on a server not yet tried, and returns which.
// Among several free servers, the choice is random, which spreads the load.
func (p *Pool) acquire(ctx context.Context, tried []bool) (int, error) {
	cases := []reflect.SelectCase{{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}}
	indexes := []int{-1}
	for i, m := range p.members {
		if tried[i] {
			continue
		}
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectSend,
			Chan: reflect.ValueOf(m.slots),
			Send: reflect.ValueOf(struct{}{}),
		})
		indexes = append(indexes, i)
	}
	chosen, _, _ := reflect.Select(cases)
	if chosen == 0 {
		return -1, ctx.Err()
	}
	return indexes[chosen], nil
}

// Pools keeps the pool of the current server settings, so the limits hold across
// requests. A change of settings starts a new pool; requests still running on the
// old one finish there.
type Pools struct {
	newTranslator func(Endpoint) Translator
	mu            sync.Mutex
	key           string
	pool          *Pool
}

// NewPools creates the cache, with a translator per endpoint from newTranslator.
func NewPools(newTranslator func(Endpoint) Translator) *Pools {
	return &Pools{newTranslator: newTranslator}
}

// Get returns the pool for these endpoints, or nil without any.
func (p *Pools) Get(endpoints []Endpoint) Translator {
	if len(endpoints) == 0 {
		return nil
	}
	parts := make([]string, len(endpoints))
	for i, endpoint := range endpoints {
		parts[i] = endpoint.ID + "@" + endpoint.Version
	}
	key := strings.Join(parts, ",")

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pool == nil || p.key != key {
		p.pool = NewPool(endpoints, p.newTranslator)
		p.key = key
	}
	return p.pool
}
