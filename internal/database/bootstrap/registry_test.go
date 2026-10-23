package bootstrap

import (
	"testing"

	"gorm.io/gorm"
)

type namedInitializer string

func (n namedInitializer) Name() string           { return string(n) }
func (n namedInitializer) Description() string    { return "" }
func (n namedInitializer) Run(*gorm.DB) error     { return nil }
func (n namedInitializer) Dependencies() []string { return nil }

func TestRegistryKeepsRegistrationOrder(t *testing.T) {
	registry := NewInitializerRegistry()
	names := []string{"Permissions", "roles", "a", "b", "c", "d", "e", "f"}

	for _, name := range names {
		if err := registry.Register(namedInitializer(name)); err != nil {
			t.Fatal(err)
		}
	}

	for i, initializer := range registry.All() {
		if initializer.Name() != names[i] {
			t.Fatalf("position %d: got %s, want %s", i, initializer.Name(), names[i])
		}
	}
}
