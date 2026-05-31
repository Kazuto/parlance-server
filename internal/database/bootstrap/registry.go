package bootstrap

import (
	"context"
	"fmt"
)

type Initializer interface {
	Name() string
	Description() string
	Run(ctx context.Context) error
	Dependencies() []string
}

type InitializerRegistry struct {
	initializers map[string]Initializer
}

func NewInitializerRegistry() *InitializerRegistry {
	return &InitializerRegistry{initializers: make(map[string]Initializer)}
}

func (r *InitializerRegistry) Register(initializer Initializer) error {
	if _, exists := r.initializers[initializer.Name()]; exists {
		return fmt.Errorf("initializer %s already registered", initializer.Name())
	}
	r.initializers[initializer.Name()] = initializer
	return nil
}

func (r *InitializerRegistry) All() []Initializer {
	result := make([]Initializer, 0, len(r.initializers))
	for _, init := range r.initializers {
		result = append(result, init)
	}
	return result
}

func (r *InitializerRegistry) RunAll(ctx context.Context) error {
	for _, init := range r.All() {
		if err := init.Run(ctx); err != nil {
			return fmt.Errorf("initializer %s failed: %w", init.Name(), err)
		}
	}
	return nil
}

func (r *InitializerRegistry) RunByName(name string, ctx context.Context) error {
	init, ok := r.initializers[name]
	if !ok {
		return fmt.Errorf("initializer %s not found", name)
	}
	return init.Run(ctx)
}
