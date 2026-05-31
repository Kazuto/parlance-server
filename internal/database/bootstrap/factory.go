package bootstrap

import (
	"gorm.io/gorm"
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

func (f *InitializerFactory) RunAll(db *gorm.DB) error {
	return f.registry.RunAll(db)
}
