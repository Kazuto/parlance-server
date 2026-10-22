// Package translators builds the machine translator from the translation servers
// stored in the database.
package translators

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/translate"
)

// Source returns the translator for the enabled servers.
type Source interface {
	// Translator returns nil while no server is enabled.
	Translator(ctx context.Context) (translate.Translator, error)
}

// DBSource reads the servers on each call, so changes apply right away, and keeps
// one pool per configuration, so each server's limit holds across requests.
type DBSource struct {
	db    *database.DB
	box   *secret.Box
	pools *translate.Pools
}

func NewDBSource(db *database.DB, box *secret.Box) *DBSource {
	return &DBSource{db: db, box: box, pools: translate.NewPools(translate.NewClientFor)}
}

func (s *DBSource) Translator(ctx context.Context) (translate.Translator, error) {
	var servers []models.TranslationServer
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("created_at").Find(&servers).Error; err != nil {
		return nil, err
	}

	endpoints := make([]translate.Endpoint, 0, len(servers))
	for _, server := range servers {
		endpoint, err := Endpoint(server, s.box)
		if err != nil {
			// E.g. the encryption key changed; the other servers still work.
			log.Printf("skip translation server %s: %v", server.Name, err)
			continue
		}
		endpoints = append(endpoints, endpoint)
	}
	return s.pools.Get(endpoints), nil
}

// Endpoint returns the connection settings of a server, with its API key decrypted.
func Endpoint(server models.TranslationServer, box *secret.Box) (translate.Endpoint, error) {
	endpoint := translate.Endpoint{
		ID:             server.ID,
		BaseURL:        server.BaseURL,
		Model:          server.Model,
		MaxConcurrency: server.MaxConcurrency,
		Version:        strconv.FormatInt(server.UpdatedAt.UnixNano(), 10),
	}
	if server.APIKey != "" {
		key, err := box.Decrypt(server.APIKey)
		if err != nil {
			return translate.Endpoint{}, fmt.Errorf("decrypt API key: %w", err)
		}
		endpoint.APIKey = key
	}
	return endpoint, nil
}

// Enabled reports whether any translation server is enabled.
func Enabled(ctx context.Context, db *database.DB) bool {
	var count int64
	db.WithContext(ctx).Model(&models.TranslationServer{}).Where("enabled = ?", true).Count(&count)
	return count > 0
}
