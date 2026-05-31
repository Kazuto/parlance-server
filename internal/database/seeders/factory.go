package seeders

import (
	"gorm.io/gorm"
)

type SeederFactory struct {
	registry *SeederRegistry
}

func NewSeederFactory() *SeederFactory {
	return &SeederFactory{registry: NewSeederRegistry()}
}

func (f *SeederFactory) Register(seeder Seeder) error {
	return f.registry.Register(seeder)
}

func (f *SeederFactory) All() []Seeder {
	return f.registry.All()
}

func (f *SeederFactory) RunAll(db *gorm.DB) error {
	return f.registry.RunAll(db)
}
