package bootstrap

import (
	"context"
)

type InitializerFactory struct {
	registry *InitializerRegistry
}

func NewInitializerFactory() *InitializerFactory {
	return &InitializerFactory{registry: NewInitializerRegistry()}
}

func (f *InitializerFactory) Register(initializer Initializer) error {
	return f.registry.Register(initializer)
}

func (f *InitializerFactory) All() []Initializer {
	return f.registry.All()
}

func (f *InitializerFactory) RunAll(ctx context.Context) error {
	return f.registry.RunAll(ctx)
}

func (f *InitializerFactory) RunByName(name string, ctx context.Context) error {
	return f.registry.RunByName(name, ctx)
}
