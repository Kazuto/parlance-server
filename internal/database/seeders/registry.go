package seeders

import (
	"fmt"

	"gorm.io/gorm"
)

type Seeder interface {
	Name() string
	Run(db *gorm.DB) error
}

type SeederRegistry struct {
	seeders map[string]Seeder
}

func NewSeederRegistry() *SeederRegistry {
	return &SeederRegistry{seeders: make(map[string]Seeder)}
}

func (r *SeederRegistry) Register(initializer Seeder) error {
	if _, exists := r.seeders[initializer.Name()]; exists {
		return fmt.Errorf("initializer %s already registered", initializer.Name())
	}
	r.seeders[initializer.Name()] = initializer
	return nil
}

func (r *SeederRegistry) All() []Seeder {
	result := make([]Seeder, 0, len(r.seeders))
	for _, init := range r.seeders {
		result = append(result, init)
	}
	return result
}

func (r *SeederRegistry) Run(seeder Seeder, db *gorm.DB) error {
	if err := seeder.Run(db); err != nil {
		return fmt.Errorf("seeder %s failed: %w", seeder.Name(), err)
	}
	return nil
}

func (r *SeederRegistry) RunAll(db *gorm.DB) error {
	for _, init := range r.All() {
		if err := init.Run(db); err != nil {
			return fmt.Errorf("seeder %s failed: %w", init.Name(), err)
		}
	}
	return nil
}
