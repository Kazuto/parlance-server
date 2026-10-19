package bootstrap

import (
	"fmt"

	"gorm.io/gorm"
)

type Initializer interface {
	Name() string
	Description() string
	Run(db *gorm.DB) error
	Dependencies() []string
}

// InitializerRegistry runs initializers in the order they were registered, so a
// later one (e.g. the admin role) can rely on an earlier one (its permissions).
type InitializerRegistry struct {
	initializers map[string]Initializer
	order        []string
}

func NewInitializerRegistry() *InitializerRegistry {
	return &InitializerRegistry{initializers: make(map[string]Initializer)}
}

func (r *InitializerRegistry) Register(initializer Initializer) error {
	if _, exists := r.initializers[initializer.Name()]; exists {
		return fmt.Errorf("initializer %s already registered", initializer.Name())
	}
	r.initializers[initializer.Name()] = initializer
	r.order = append(r.order, initializer.Name())
	return nil
}

func (r *InitializerRegistry) All() []Initializer {
	result := make([]Initializer, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.initializers[name])
	}
	return result
}

func (r *InitializerRegistry) RunAll(db *gorm.DB) error {
	for _, init := range r.All() {
		if err := init.Run(db); err != nil {
			return fmt.Errorf("initializer %s failed: %w", init.Name(), err)
		}
	}
	return nil
}
