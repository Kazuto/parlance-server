package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/server/auth"
	"github.com/kazuto/parlance-server/internal/server/definition"
	"github.com/kazuto/parlance-server/internal/server/entry"
	"github.com/kazuto/parlance-server/internal/server/export"
	importservice "github.com/kazuto/parlance-server/internal/server/import"
	"github.com/kazuto/parlance-server/internal/server/locale"
	"github.com/kazuto/parlance-server/internal/server/localization"
	"github.com/kazuto/parlance-server/internal/server/permission"
	"github.com/kazuto/parlance-server/internal/server/role"
	"github.com/kazuto/parlance-server/internal/server/scope"
	"github.com/kazuto/parlance-server/internal/server/terminology"
	"github.com/kazuto/parlance-server/internal/server/user"
)

func main() {
	log.Println("Starting Parlance Server...")

	cfg := config.Load()
	log.Printf("Environment: %s", cfg.Server.Env)

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	if err := db.Bootstrap(); err != nil {
		log.Fatalf("Failed to bootstrap database: %v", err)
	}

	if cfg.Server.Env == "development" {
		if err := db.Seed(); err != nil {
			log.Fatalf("Failed to seed database: %v", err)
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"parlance-api"}`))
	})

	authMiddleware := middleware.AuthMiddleware(cfg.JWT.Secret)

	// Register Connect RPC services
	localeServer := locale.NewServer(db)
	localePath, localeHandler := parlancev1connect.NewLocaleServiceHandler(localeServer)
	mux.Handle(localePath, localeHandler)

	log.Println("✓ Registered LocaleService")

	authServer := auth.NewServer(db, cfg)
	authPath, authHandler := parlancev1connect.NewAuthServiceHandler(authServer)
	mux.Handle(authPath, authHandler)

	log.Println("✓ Registered AuthService")

	scopeServer := scope.NewServer(db)
	scopePath, scopeHandler := parlancev1connect.NewScopeServiceHandler(scopeServer)
	mux.Handle(scopePath, middleware.RequireAuth(cfg.JWT.Secret)(scopeHandler))

	log.Println("✓ Registered ScopeService (protected)")

	userServer := user.NewServer(db)
	userPath, userHandler := parlancev1connect.NewUserServiceHandler(userServer)
	mux.Handle(userPath, middleware.RequireAuth(cfg.JWT.Secret)(userHandler))

	log.Println("✓ Registered UserService (protected)")

	entryServer := entry.NewServer(db)
	entryPath, entryHandler := parlancev1connect.NewEntryServiceHandler(entryServer)
	mux.Handle(entryPath, middleware.RequireAuth(cfg.JWT.Secret)(entryHandler))

	log.Println("✓ Registered EntryService (protected)")

	localizationServer := localization.NewServer(db, cfg)
	localizationPath, localizationHandler := parlancev1connect.NewLocalizationServiceHandler(localizationServer)
	mux.Handle(localizationPath, middleware.RequireAuth(cfg.JWT.Secret)(localizationHandler))

	log.Println("✓ Registered LocalizationService (protected)")

	terminologyServer := terminology.NewServer(db)
	terminologyPath, terminologyHandler := parlancev1connect.NewTerminologyServiceHandler(terminologyServer)
	mux.Handle(terminologyPath, middleware.RequireAuth(cfg.JWT.Secret)(terminologyHandler))

	log.Println("✓ Registered TerminologyService (protected)")

	definitionServer := definition.NewServer(db)
	definitionPath, definitionHandler := parlancev1connect.NewDefinitionServiceHandler(definitionServer)
	mux.Handle(definitionPath, middleware.RequireAuth(cfg.JWT.Secret)(definitionHandler))

	log.Println("✓ Registered DefinitionService (protected)")

	permissionServer := permission.NewServer(db)
	permissionPath, permissionHandler := parlancev1connect.NewPermissionServiceHandler(permissionServer)
	mux.Handle(permissionPath, middleware.RequireAuth(cfg.JWT.Secret)(permissionHandler))

	log.Println("✓ Registered PermissionService (protected)")

	roleServer := role.NewServer(db)
	rolePath, roleHandler := parlancev1connect.NewRoleServiceHandler(roleServer)
	mux.Handle(rolePath, middleware.RequireAuth(cfg.JWT.Secret)(roleHandler))

	log.Println("✓ Registered RoleService (protected)")

	exportServer := export.NewServer(db)
	exportPath, exportHandler := parlancev1connect.NewExportServiceHandler(exportServer)
	mux.Handle(exportPath, middleware.RequireAuth(cfg.JWT.Secret)(exportHandler))

	log.Println("✓ Registered ExportService (protected)")

	importServer := importservice.NewServer(db)
	importPath, importHandler := parlancev1connect.NewImportServiceHandler(importServer)
	mux.Handle(importPath, middleware.RequireAuth(cfg.JWT.Secret)(importHandler))

	log.Println("✓ Registered ImportService (protected)")

	// Create server with h2c (HTTP/2 without TLS for development)
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h2c.NewHandler(corsMiddleware(authMiddleware(mux), cfg.CORS.AllowedOrigins), &http2.Server{}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server listening on %s", addr)
		log.Printf("Health check: http://localhost%s/health", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin || allowedOrigin == "*" {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Connect-Protocol-Version, Connect-Timeout-Ms")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
