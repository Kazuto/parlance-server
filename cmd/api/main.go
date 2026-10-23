package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/activity"
	"github.com/kazuto/parlance-server/internal/server/auth"
	"github.com/kazuto/parlance-server/internal/server/definition"
	"github.com/kazuto/parlance-server/internal/server/entry"
	"github.com/kazuto/parlance-server/internal/server/export"
	"github.com/kazuto/parlance-server/internal/server/importservice"
	"github.com/kazuto/parlance-server/internal/server/locale"
	"github.com/kazuto/parlance-server/internal/server/localization"
	"github.com/kazuto/parlance-server/internal/server/permission"
	"github.com/kazuto/parlance-server/internal/server/rbac"
	"github.com/kazuto/parlance-server/internal/server/role"
	"github.com/kazuto/parlance-server/internal/server/scope"
	"github.com/kazuto/parlance-server/internal/server/setup"
	"github.com/kazuto/parlance-server/internal/server/stats"
	"github.com/kazuto/parlance-server/internal/server/task"
	"github.com/kazuto/parlance-server/internal/server/terminology"
	"github.com/kazuto/parlance-server/internal/server/translationserver"
	"github.com/kazuto/parlance-server/internal/server/user"
	"github.com/kazuto/parlance-server/internal/server/webhookservice"
	"github.com/kazuto/parlance-server/internal/taskrunner"
	"github.com/kazuto/parlance-server/internal/translators"
	"github.com/kazuto/parlance-server/internal/webhook"
)

func main() {
	log.Println("Starting Parlance Server...")

	cfg := config.Load()
	log.Printf("Environment: %s", cfg.Server.Env)

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Bootstrap is idempotent: it adds permissions introduced since the database was
	// set up and gives the admin role all of them again.
	if err := db.Bootstrap(); err != nil {
		log.Fatalf("Failed to bootstrap database: %v", err)
	}

	s3Client, err := newS3Client(cfg.S3)
	if err != nil {
		log.Fatalf("Failed to connect to S3: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte(`{"status":"ok","service":"parlance-api"}`)); err != nil {
			log.Printf("Failed to write health response: %v", err)
		}
	})

	authMiddleware := middleware.AuthMiddleware(cfg.JWT.Secret)

	secretBox, err := secret.New(cfg.Secrets.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to set up encryption: %v", err)
	}

	translatorSource := translators.NewDBSource(db, secretBox)

	// The task runner runs scheduled, triggered and requested task runs. Admins
	// pause and resume tasks in the app. With several instances of the server,
	// each run still happens once.
	runnerCtx, stopRunner := context.WithCancel(context.Background())
	defer stopRunner()

	runner := taskrunner.New(db.DB, translatorSource, secretBox)
	runnerDone := runner.Start(runnerCtx)

	// The dispatcher sends webhook deliveries and tries failed ones again. With
	// several instances of the server, each delivery is still sent once at a time.
	dispatcherCtx, stopDispatcher := context.WithCancel(context.Background())
	defer stopDispatcher()

	webhookSender := webhook.NewSender(secretBox)
	dispatcherDone := webhook.NewDispatcher(db.DB, webhookSender).Start(dispatcherCtx)

	// Protected services require a valid token and the permissions rbac.Rules lists.
	requireAuth := middleware.RequireAuth(cfg.JWT.Secret)
	authorized := connect.WithInterceptors(rbac.Interceptor(db))

	services := []struct {
		name      string
		register  func(...connect.HandlerOption) (string, http.Handler)
		protected bool
	}{
		{"SetupService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewSetupServiceHandler(setup.NewServer(db, cfg), o...)
		}, false},
		{"AuthService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewAuthServiceHandler(auth.NewServer(db, cfg), o...)
		}, false},
		{"LocaleService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewLocaleServiceHandler(locale.NewServer(db), o...)
		}, true},
		{"ScopeService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewScopeServiceHandler(scope.NewServer(db), o...)
		}, true},
		{"UserService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewUserServiceHandler(user.NewServer(db, s3Client, cfg.S3), o...)
		}, true},
		{"EntryService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewEntryServiceHandler(entry.NewServer(db), o...)
		}, true},
		{"ActivityService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewActivityServiceHandler(activity.NewServer(db), o...)
		}, true},
		{"LocalizationService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewLocalizationServiceHandler(localization.NewServer(db, cfg, translatorSource), o...)
		}, true},
		{"TranslationServerService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewTranslationServerServiceHandler(translationserver.NewServer(db, secretBox), o...)
		}, true},
		{"TerminologyService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewTerminologyServiceHandler(terminology.NewServer(db), o...)
		}, true},
		{"DefinitionService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewDefinitionServiceHandler(definition.NewServer(db), o...)
		}, true},
		{"PermissionService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewPermissionServiceHandler(permission.NewServer(db), o...)
		}, true},
		{"RoleService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewRoleServiceHandler(role.NewServer(db), o...)
		}, true},
		{"ExportService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewExportServiceHandler(export.NewServer(db), o...)
		}, true},
		{"ImportService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewImportServiceHandler(importservice.NewServer(db), o...)
		}, true},
		{"TaskService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewTaskServiceHandler(task.NewServer(db, secretBox, runner.Wake), o...)
		}, true},
		{"StatsService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewStatsServiceHandler(stats.NewServer(db), o...)
		}, true},
		{"WebhookService", func(o ...connect.HandlerOption) (string, http.Handler) {
			return parlancev1connect.NewWebhookServiceHandler(webhookservice.NewServer(db, secretBox, webhookSender), o...)
		}, true},
	}

	for _, service := range services {
		if !service.protected {
			mux.Handle(service.register())
			log.Printf("✓ Registered %s (public)", service.name)

			continue
		}

		path, handler := service.register(authorized)
		mux.Handle(path, requireAuth(handler))
		log.Printf("✓ Registered %s (protected)", service.name)
	}

	// Create server with h2c (HTTP/2 without TLS for development)
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h2c.NewHandler(corsMiddleware(authMiddleware(mux), cfg.CORS.AllowedOrigins), &http2.Server{}),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
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

	// A run in progress is ended as interrupted, so it doesn't wait to go stale.
	// Webhook deliveries cut short are sent again once their lease runs out.
	stopRunner()
	stopDispatcher()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	select {
	case <-runnerDone:
	case <-ctx.Done():
		log.Println("Task runner did not stop in time")
	}

	select {
	case <-dispatcherDone:
	case <-ctx.Done():
		log.Println("Webhook dispatcher did not stop in time")
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

func newS3Client(cfg config.S3Config) (*s3.Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
		awsconfig.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 config: %w", err)
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(fmt.Sprintf("http://%s", cfg.Endpoint))
	}), nil
}
