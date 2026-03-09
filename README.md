# Parlance Server

Backend API server for Parlance - a powerful, developer-friendly translation management platform that streamlines localization workflows with AI-powered translations, custom terminology control, and flexible export options.

This repository contains the **Go backend server** built with Connect RPC. For the frontend client, see [parlance-client](https://github.com/criticaldevs/parlance).

## Table of Contents

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Quick Start](#quick-start)
- [Usage](#usage)
- [Project Structure](#project-structure)
- [Database Schema](#database-schema)
- [Development Workflow](#development-workflow)
- [Configuration](#configuration)
- [Client Integration](#client-integration)
- [API Integration](#api-integration)
- [Implementation Phases](#implementation-phases)
- [Roadmap](#roadmap)
- [Why Connect RPC?](#why-connect-rpc)
- [Security](#security)
- [Testing](#testing)
- [Deployment](#deployment)
- [Contributing](#contributing)
- [Documentation](#documentation)

## Features

### Core Functionality

- **Connect RPC API** - Type-safe RPC framework with JSON/Protobuf support over HTTP
- **Key-Based Translation System** - Manage translations with unique identifiers
- **Multi-Locale Support** - Handle unlimited languages with PostgreSQL backing
- **AI-Powered Translations** - DeepL API integration for automated translations
- **Custom Terminology** - Glossary system to ensure consistent translations
- **Scopes & Organization** - Group translations by context (frontend, validation, errors, etc.)

### API Features

- **Protocol Buffers** - Type-safe API definitions shared across clients
- **Dual Protocol Support** - JSON over HTTP/1.1 and Binary Protobuf over HTTP/2
- **Multiple Export Formats** - Export to Laravel, Vue i18n, JSON, and other frameworks
- **Version Control** - Complete audit trail for all translation changes
- **Search & Filter** - Full-text search across translations

### Authentication & Security

- **JWT Authentication** - Secure token-based authentication
- **Role-Based Access Control** - Granular permissions for admins, translators, reviewers
- **Audit Trail** - Track who changed what and when
- **Input Validation** - Protobuf-based validation and sanitization

## Tech Stack

### Core

- **Language**: Go 1.23+
- **RPC Framework**: [Connect RPC](https://connectrpc.com/) - gRPC-compatible RPC over HTTP
- **Protocol Buffers**: protobuf v3 for service definitions
- **Database**: PostgreSQL 16+
- **ORM**: [GORM](https://gorm.io/) or [sqlc](https://sqlc.dev/)
- **Migrations**: [golang-migrate](https://github.com/golang-migrate/migrate)

### Supporting Libraries

- **Validation**: [go-playground/validator](https://github.com/go-playground/validator) + protobuf validation
- **Configuration**: [viper](https://github.com/spf13/viper)
- **Authentication**: JWT ([golang-jwt/jwt](https://github.com/golang-jwt/jwt))
- **Logging**: [zap](https://github.com/uber-go/zap) or [zerolog](https://github.com/rs/zerolog)
- **HTTP Client**: Standard library `net/http` (for DeepL API)
- **Code Generation**: [Buf CLI](https://buf.build/)
- **gRPC-Gateway**: (Optional) Traditional REST URLs from proto files

### Containerization

- **Runtime**: Docker
- **Orchestration**: Docker Compose (dev), Kubernetes (prod)
- **Base Images**: `golang:1.23-alpine` (build), `alpine:latest` (runtime)

See [PLAN.md](PLAN.md) for detailed implementation plan and architecture.

## Quick Start

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- PostgreSQL 16+ (or use Docker)
- Buf CLI (for proto code generation)

### Installation

```bash
# Clone the repository
git clone https://github.com/criticaldevs/parlance-server.git
cd parlance-server

# Install development tools
make install-tools

# Configure environment
cp .env.example .env

# Start services (PostgreSQL + API)
docker-compose up -d

# Run database migrations
make migrate-up

# Generate code from proto files
make proto-gen

# View logs
docker-compose logs -f api
```

### Development Setup (Local)

```bash
# Generate proto code
make proto-gen

# Run the server locally
make run

# Run tests
make test

# Lint proto files
make proto-lint
```

## Usage

### API Endpoints

The server exposes Connect RPC endpoints at:

```
# Authentication
POST /parlance.v1.AuthService/Login
POST /parlance.v1.AuthService/Register
POST /parlance.v1.AuthService/RefreshToken

# Entries (Translation Keys)
POST /parlance.v1.EntryService/CreateEntry
POST /parlance.v1.EntryService/GetEntry
POST /parlance.v1.EntryService/ListEntries
POST /parlance.v1.EntryService/UpdateEntry
POST /parlance.v1.EntryService/DeleteEntry

# Localizations (Translations)
POST /parlance.v1.LocalizationService/CreateLocalization
POST /parlance.v1.LocalizationService/GetLocalization
POST /parlance.v1.LocalizationService/ListLocalizations
POST /parlance.v1.LocalizationService/UpdateLocalization
POST /parlance.v1.LocalizationService/TranslateLocalization

# Terminologies (Glossary)
POST /parlance.v1.TerminologyService/CreateTerminology
POST /parlance.v1.TerminologyService/ListTerminologies
POST /parlance.v1.TerminologyService/UpdateTerminology

# Scopes, Locales, Users, Export
POST /parlance.v1.ScopeService/...
POST /parlance.v1.LocaleService/...
POST /parlance.v1.UserService/...
POST /parlance.v1.ExportService/ExportTranslations

... (all endpoints auto-generated from proto definitions)
```

### Example: Using Connect Client (TypeScript)

```typescript
import { createPromiseClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { EntryService } from "~/gen/parlance/v1/entry_connect";

const transport = createConnectTransport({
  baseUrl: "http://localhost:8080",
});

const client = createPromiseClient(EntryService, transport);

// List entries
const response = await client.listEntries({
  pagination: { page: 1, perPage: 20 },
  scopeId: "frontend",
});

// Create entry
const entry = await client.createEntry({
  key: "auth.login",
  scopeId: "frontend",
  description: "Login page title",
});
```

### Example: Using cURL

```bash
# Login
curl -X POST http://localhost:8080/parlance.v1.AuthService/Login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password"}'

# List entries
curl -X POST http://localhost:8080/parlance.v1.EntryService/ListEntries \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"pagination": {"page": 1, "perPage": 20}}'
```

## Project Structure

```
parlance-server/
├── proto/                              # Protocol Buffer definitions
│   └── parlance/v1/
│       ├── entry.proto                # Entry service definitions
│       ├── localization.proto         # Localization service
│       ├── terminology.proto          # Terminology service
│       ├── scope.proto                # Scope service
│       ├── locale.proto               # Locale service
│       ├── auth.proto                 # Authentication service
│       ├── user.proto                 # User management service
│       └── export.proto               # Export service
├── gen/                                # Generated Go code from proto
│   └── parlance/v1/                   # Auto-generated RPC stubs
├── cmd/api/                           # Application entry point
│   └── main.go                        # Server initialization
├── internal/
│   ├── config/                        # Configuration management
│   │   └── config.go                  # Viper configuration
│   ├── database/                      # Database layer
│   │   ├── connection.go              # PostgreSQL connection
│   │   └── migrations/                # SQL migration files
│   ├── models/                        # GORM database models
│   │   ├── entry.go, localization.go, terminology.go
│   │   ├── user.go, role.go, permission.go
│   │   └── ... (16 model files total)
│   ├── server/                        # RPC service implementations
│   │   ├── entry_server.go            # EntryService implementation
│   │   ├── localization_server.go     # LocalizationService
│   │   ├── auth_server.go             # AuthService
│   │   └── ... (8 service files total)
│   ├── services/                      # Business logic
│   │   ├── deepl.go                   # DeepL API integration
│   │   ├── translation.go             # Translation logic
│   │   ├── export.go                  # Export format generation
│   │   ├── history.go                 # History tracking
│   │   └── scope.go                   # Scope management
│   ├── interceptors/                  # Connect middleware
│   │   ├── auth.go                    # JWT authentication
│   │   ├── rbac.go                    # Role-based access control
│   │   └── logging.go                 # Request/response logging
│   └── converter/                     # Type conversion
│       └── converter.go               # Proto ↔ DB model mapping
├── pkg/utils/                         # Shared utilities
│   ├── pagination.go                  # Pagination helpers
│   └── validation.go                  # Validation utilities
├── docker/                            # Docker configuration
│   ├── Dockerfile                     # Multi-stage build
│   └── docker-compose.yml             # Dev environment
├── scripts/                           # Helper scripts
│   ├── proto-gen.sh                   # Proto code generation
│   └── migrate.sh                     # Migration runner
├── buf.yaml                           # Buf linting config
├── buf.gen.yaml                       # Buf code generation
├── Makefile                           # Development commands
├── go.mod                             # Go dependencies
├── .env.example                       # Environment template
└── PLAN.md                            # Implementation plan
```

## Database Schema

The system uses a normalized PostgreSQL database structure:

### Translation System

- **entries** - Translation keys/identifiers with descriptions
- **localizations** - Locale-specific translations (one per locale per entry)
- **localization_history** - Historical versions of translations
- **terminologies** - Custom terminology definitions (glossary terms)
- **definitions** - Locale-specific terminology translations
- **definition_history** - Historical versions of terminology
- **scopes** - Organization categories (frontend, validation, errors, etc.)
- **entry_scopes** - Many-to-many relationship between entries and scopes
- **locales** - Supported languages with default locale support

### Access Control (RBAC)

- **users** - User accounts with email/password
- **roles** - RBAC roles (admin, translator, reviewer, etc.)
- **permissions** - Granular permissions (read, write, delete, etc.)
- **role_permissions** - Many-to-many relationship
- **user_roles** - Many-to-many relationship

### Audit Strategy

- **Soft Deletes** - `deleted_at` field on all core tables
- **Audit Columns** - `created_by`, `updated_by` tracking via JWT context
- **History Tables** - Dedicated tables for localization and terminology changes
- **Constraints** - Unique indexes ensuring data integrity

Full schema, migrations, and GORM models available in `internal/database/` and `internal/models/`

## Development Workflow

### Proto Development

```bash
# 1. Edit proto files in proto/parlance/v1/

# 2. Lint proto files
make proto-lint

# 3. Check for breaking changes
make proto-breaking

# 4. Generate Go code
make proto-gen

# 5. Implement service in internal/server/

# 6. Test
make test
```

### Available Make Commands

```bash
make install-tools    # Install Buf CLI and other development tools
make proto-gen        # Generate Go code from proto files
make proto-lint       # Lint proto files
make proto-breaking   # Check for breaking changes
make dev              # Start development environment (docker-compose up)
make build            # Build Docker images
make run              # Build and run server locally
make test             # Run all tests
make lint             # Run Go linter (golangci-lint)
make migrate-up       # Run database migrations
make migrate-down     # Rollback database migrations
```

## Configuration

Configure via environment variables (see `.env.example`):

```bash
# Server
PORT=8080
ENV=development

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=parlance
DB_PASSWORD=secret
DB_NAME=parlance
DB_SSL_MODE=disable

# JWT
JWT_SECRET=your-secret-key
JWT_EXPIRY=24h

# DeepL
DEEPL_API_KEY=your-deepl-key
DEEPL_API_URL=https://api.deepl.com/v2

# CORS
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080
```

## Client Integration

### TypeScript/Nuxt Client

Install dependencies:

```bash
npm install @connectrpc/connect @connectrpc/connect-web
npm install -D @bufbuild/buf @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
```

Generate TypeScript code from proto files:

```bash
buf generate
```

See [PLAN.md](PLAN.md) for complete client integration examples.

## API Integration

### DeepL Translation

The server integrates with DeepL API for AI-powered translations:

```go
// Translate with terminology
resp := api.localization.translateLocalization({
  entryId: "entry-123",
  targetLocaleId: "de",
  sourceLocaleId: "en",
  applyTerminology: true  // Apply custom glossary
})
```

### Export Formats

Support for multiple export formats:

- **Laravel** - PHP array format
- **Vue i18n** - JavaScript/JSON format
- **JSON** - Standard key-value format

## Implementation Phases

### Phase 1: Foundation (Week 1) ✅

- [x] Project structure and proto definitions
- [x] Database schema design and migrations
- [x] Docker and docker-compose setup
- [x] Buf configuration for code generation

### Phase 2: Core RPC Services (Week 2)

- [ ] CRUD operations for all core entities
- [ ] Proto-to-DB model converters
- [ ] Input validation and error handling
- [ ] Unit and integration tests

### Phase 3: Authentication & Authorization (Week 3)

- [ ] JWT authentication system
- [ ] RBAC implementation with interceptors
- [ ] User, role, and permission management
- [ ] Auth middleware and context propagation

### Phase 4: AI Integration (Week 4)

- [ ] DeepL API integration
- [ ] Custom terminology application
- [ ] Batch translation operations
- [ ] Translation quality validation

### Phase 5: Advanced Features (Week 5)

- [ ] Export formats (Laravel, Vue i18n, JSON)
- [ ] History tracking with GORM hooks
- [ ] Full-text search across translations
- [ ] Audit trail and change logs

### Phase 6: Production Ready (Week 6)

- [ ] Error handling and recovery
- [ ] Caching layer (Redis)
- [ ] Observability (metrics, tracing, logging)
- [ ] Security hardening and rate limiting
- [ ] Performance optimization
- [ ] Documentation and deployment guides

See [PLAN.md](PLAN.md) for detailed task breakdown and architecture decisions.

## Roadmap

### Near-term (Post Phase 6)

- [ ] Redis caching layer for frequently accessed translations
- [ ] gRPC-Gateway for traditional REST API compatibility
- [ ] OpenAPI/Swagger documentation generation
- [ ] Batch operations and bulk import/export
- [ ] Advanced full-text search with Elasticsearch

### Mid-term

- [ ] Real-time collaboration via WebSocket/Server-Sent Events
- [ ] Translation memory and suggestion engine
- [ ] Automated quality checks and validation rules
- [ ] Machine translation post-editing workflow
- [ ] Context screenshots for translators
- [ ] Figma/Sketch plugin integration

### Long-term

- [ ] Multi-tenancy support for SaaS deployment
- [ ] Plugin system for custom integrations
- [ ] AI-powered translation quality scoring
- [ ] Slack/Discord notifications and webhooks
- [ ] Mobile apps (iOS/Android) with offline support
- [ ] Advanced analytics and reporting dashboard

## Why Connect RPC?

Connect RPC provides several advantages over traditional REST:

- ✅ **Type Safety** - Auto-generated TypeScript/Go types from proto files
- ✅ **Single Source of Truth** - Proto definitions shared between server and clients
- ✅ **Better DX** - Method calls instead of manual HTTP requests
- ✅ **Dual Protocol** - JSON for browsers, Protobuf for performance
- ✅ **Streaming Support** - Server/client streaming when needed
- ✅ **No CORS Issues** - Works over standard HTTP
- ✅ **Multi-Client Ready** - Web, Electron, mobile with same proto definitions

See [PLAN.md](PLAN.md) for detailed comparison with REST and architecture decisions.

## Security

The server implements multiple security layers:

- **Input Validation** - Protobuf validation + go-playground/validator on all endpoints
- **SQL Injection Prevention** - Parameterized queries via GORM
- **XSS Prevention** - Sanitized output and Content-Security-Policy headers
- **CSRF Protection** - Token-based protection for state-changing operations
- **Rate Limiting** - Per IP/user limits to prevent abuse
- **Password Security** - bcrypt hashing with salt
- **JWT Security** - Short-lived tokens with refresh mechanism
- **HTTPS Only** - TLS required in production
- **Security Headers** - HSTS, X-Frame-Options, X-Content-Type-Options
- **Database Pooling** - Connection limits to prevent resource exhaustion

See [PLAN.md](PLAN.md) for detailed security considerations and best practices.

## Testing

Comprehensive testing strategy across multiple levels:

- **Unit Tests** - Service logic and business rules
- **Integration Tests** - Database operations and RPC handlers
- **E2E Tests** - Complete API workflows with real database
- **Load Tests** - Performance validation with k6 or vegeta

```bash
# Run all tests
make test

# Run with coverage
go test -v -cover ./...

# Run integration tests only
go test -v -tags=integration ./...
```

## Deployment

### Production Considerations

- **Database**: Managed PostgreSQL (AWS RDS, Google Cloud SQL, or similar)
- **Container Registry**: Docker Hub, AWS ECR, or Google Container Registry
- **Orchestration**: Kubernetes with Helm charts or Docker Swarm
- **Monitoring**: Prometheus + Grafana for metrics and alerting
- **Logging**: Centralized logging with ELK stack or similar
- **CI/CD**: GitHub Actions, GitLab CI, or Jenkins for automated builds/deployments

### Docker Build

```bash
# Build production image
docker build -t parlance-server:latest .

# Run container
docker run -p 8080:8080 --env-file .env parlance-server:latest
```

### Environment Variables

See [Configuration](#configuration) section for required environment variables.

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

This project is licensed under the GNU General Public License - see the [LICENSE](LICENSE) file for details.

## Documentation

- [PLAN.md](PLAN.md) - Detailed implementation plan and architecture
- [Proto Definitions](proto/parlance/v1/) - API service definitions
- [Connect RPC Documentation](https://connectrpc.com/docs/go/getting-started)
- [Buf CLI Documentation](https://buf.build/docs)

## Acknowledgments

- Inspired by modern translation management tools
- Built with Go, Connect RPC, PostgreSQL, and Protocol Buffers
- Frontend client built with Nuxt.js and TypeScript

## Contact

- Project Link: [https://github.com/criticaldevs/parlance-server](https://github.com/criticaldevs/parlance-server)
- Issues: [https://github.com/criticaldevs/parlance-server/issues](https://github.com/criticaldevs/parlance-server/issues)
- Frontend Client: [https://github.com/criticaldevs/parlance](https://github.com/criticaldevs/parlance)

---

Made with ❤️ by developers, for developers
