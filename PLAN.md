# Parlance Backend - Go Implementation Plan

## Overview

Standalone Go backend API for Parlance translation management platform, containerized with Docker for multi-client support (web + Electron).

## Technology Stack

### Core
- **Language**: Go 1.23+
- **RPC Framework**: [Connect (Buf)](https://connectrpc.com/) - gRPC-compatible RPC over HTTP
- **Protocol Buffers**: protobuf v3 for service definitions
- **Database**: PostgreSQL 16+
- **ORM**: [GORM](https://gorm.io/) or [sqlc](https://sqlc.dev/)
- **Migration**: [golang-migrate](https://github.com/golang-migrate/migrate)

### Supporting Libraries
- **Connect**: [connectrpc.com/connect](https://connectrpc.com/connect) - Dual JSON/Protobuf support
- **gRPC-Gateway**: (Optional) Traditional REST URLs from proto files
- **Buf CLI**: Code generation and linting for protobuf
- **Validation**: [go-playground/validator](https://github.com/go-playground/validator) + protobuf validation
- **Configuration**: [viper](https://github.com/spf13/viper)
- **Authentication**: JWT ([golang-jwt/jwt](https://github.com/golang-jwt/jwt))
- **HTTP Client**: Standard library `net/http` (for DeepL API)
- **Logging**: [zap](https://github.com/uber-go/zap) or [zerolog](https://github.com/rs/zerolog)

### Containerization
- **Runtime**: Docker
- **Orchestration**: Docker Compose (dev), Kubernetes (prod)
- **Base Image**: `golang:1.23-alpine` (build), `alpine:latest` (runtime)

## Project Structure

```
parlance-server/
├── proto/
│   └── parlance/
│       └── v1/
│           ├── entry.proto          # Entry service definitions
│           ├── localization.proto   # Localization service
│           ├── terminology.proto    # Terminology service
│           ├── scope.proto          # Scope service
│           ├── locale.proto         # Locale service
│           ├── auth.proto           # Authentication service
│           ├── user.proto           # User service
│           ├── export.proto         # Export service
│           └── common.proto         # Shared messages (pagination, etc.)
├── gen/
│   ├── parlance/
│   │   └── v1/                      # Generated Go code from proto
│   │       ├── entry/
│   │       ├── localization/
│   │       └── ...
│   └── openapi/                     # Generated OpenAPI docs (optional)
│       └── parlance/
│           └── v1/
├── cmd/
│   └── api/
│       └── main.go                  # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration management
│   ├── database/
│   │   ├── migrations/             # SQL migrations
│   │   └── db.go                   # Database connection
│   ├── models/
│   │   ├── entry.go                # Database models
│   │   ├── localization.go
│   │   ├── localization_history.go
│   │   ├── terminology.go
│   │   ├── definition.go
│   │   ├── definition_history.go
│   │   ├── scope.go
│   │   ├── entry_scope.go
│   │   ├── locale.go
│   │   ├── user.go
│   │   ├── role.go
│   │   ├── permission.go
│   │   ├── user_role.go
│   │   └── role_permission.go
│   ├── server/
│   │   ├── entry.go                # Entry RPC service implementation
│   │   ├── localization.go         # Localization RPC implementation
│   │   ├── terminology.go          # Terminology RPC implementation
│   │   ├── scope.go                # Scope RPC implementation
│   │   ├── locale.go               # Locale RPC implementation
│   │   ├── auth.go                 # Auth RPC implementation
│   │   ├── user.go                 # User RPC implementation
│   │   └── export.go               # Export RPC implementation
│   ├── services/
│   │   ├── translation.go          # Translation business logic
│   │   ├── deepl.go                # DeepL API integration
│   │   ├── export.go               # Export format generators
│   │   ├── history.go              # History tracking service
│   │   └── scope.go                # Scope management service
│   ├── interceptors/
│   │   ├── auth.go                 # JWT authentication interceptor
│   │   ├── rbac.go                 # Permission checking interceptor
│   │   └── logging.go              # Request logging interceptor
│   └── converter/
│       └── converter.go            # Proto <-> DB model conversion
├── pkg/
│   └── utils/                      # Shared utilities
├── docker/
│   ├── Dockerfile                  # Multi-stage build
│   └── docker-compose.yml          # Development environment
├── scripts/
│   ├── migrate.sh                  # Migration helper
│   └── gen-proto.sh                # Proto code generation
├── buf.yaml                        # Buf configuration
├── buf.gen.yaml                    # Buf code generation config
├── .env.example
├── .dockerignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Database Schema

### Core Tables
```sql
-- Translation System
entries                  # Translation keys (e.g., "auth.login.title")
localizations           # Locale-specific translations for entries
localization_history    # Audit trail for localization changes
terminologies           # Custom terminology/glossary terms
definitions             # Locale-specific terminology translations
definition_history      # Audit trail for definition changes
scopes                  # Organization categories (frontend, backend, etc.)
entry_scopes            # Many-to-many: entries can belong to multiple scopes
locales                 # Supported languages (en, de, fr, etc.)

-- Access Control (RBAC)
users                   # User accounts
roles                   # User roles (admin, translator, viewer)
permissions             # Granular permissions
role_permissions        # Many-to-many: roles to permissions
user_roles              # Many-to-many: users to roles
```

### Schema Details

**Audit Strategy:**
- `created_by`, `updated_by` on: entries, localizations, terminologies, definitions, entry_scopes
- Dedicated history tables: `localization_history`, `definition_history`
- Soft deletes: `deleted_at` on most tables
- History tracking: who changed what and when

**Key Features:**
- ✅ Soft deletes for safe data recovery
- ✅ Audit columns (created_by, updated_by) on core tables
- ✅ Complete history tables for critical data
- ✅ Many-to-many: entries can have multiple scopes (tags)
- ✅ Unique constraint: one translation per locale per entry
- ✅ Default locale support with unique constraint

## API Design

### Connect RPC Services

All services use Connect RPC framework with Protocol Buffers over HTTP/1.1 or HTTP/2.

#### Common Types (proto/parlance/v1/common.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

message PaginationRequest {
  int32 page = 1;
  int32 per_page = 2;
}

message PaginationResponse {
  int32 total = 1;
  int32 page = 2;
  int32 per_page = 3;
  int32 total_pages = 4;
}

message Empty {}
```

#### Authentication Service (proto/parlance/v1/auth.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service AuthService {
  rpc Register(RegisterRequest) returns (AuthResponse);
  rpc Login(LoginRequest) returns (AuthResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (AuthResponse);
  rpc Logout(LogoutRequest) returns (Empty);
  rpc GetCurrentUser(Empty) returns (User);
}

message RegisterRequest {
  string email = 1;
  string password = 2;
  string name = 3;
}

message LoginRequest {
  string email = 1;
  string password = 2;
}

message RefreshTokenRequest {
  string refresh_token = 1;
}

message AuthResponse {
  string access_token = 1;
  string refresh_token = 2;
  User user = 3;
}
```

#### Entry Service (proto/parlance/v1/entry.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service EntryService {
  rpc ListEntries(ListEntriesRequest) returns (ListEntriesResponse);
  rpc GetEntry(GetEntryRequest) returns (Entry);
  rpc CreateEntry(CreateEntryRequest) returns (Entry);
  rpc UpdateEntry(UpdateEntryRequest) returns (Entry);
  rpc DeleteEntry(DeleteEntryRequest) returns (Empty);
  rpc SearchEntries(SearchEntriesRequest) returns (ListEntriesResponse);
  rpc GetEntryHistory(GetEntryHistoryRequest) returns (EntryHistoryResponse);
}

message Entry {
  string id = 1;
  string key = 2;
  string description = 3;
  string created_at = 4;
  string updated_at = 5;
  string created_by = 6;
  string updated_by = 7;
  repeated Localization localizations = 8;
  repeated Scope scopes = 9;  // Many-to-many relationship
}

message ListEntriesRequest {
  PaginationRequest pagination = 1;
  string scope_id = 2;
  string locale_id = 3;
}

message ListEntriesResponse {
  repeated Entry entries = 1;
  PaginationResponse pagination = 2;
}

message GetEntryRequest {
  string id = 1;
}

message CreateEntryRequest {
  string key = 1;
  string scope_id = 2;
  string description = 3;
}

message UpdateEntryRequest {
  string id = 1;
  string key = 2;
  string description = 3;
}

message DeleteEntryRequest {
  string id = 1;
}

message SearchEntriesRequest {
  string query = 1;
  PaginationRequest pagination = 2;
  string scope_id = 3;
}

message GetEntryHistoryRequest {
  string entry_id = 1;
  PaginationRequest pagination = 2;
}

message EntryHistoryResponse {
  repeated LocalizationHistory history = 1;
  PaginationResponse pagination = 2;
}

message LocalizationHistory {
  string id = 1;
  string localization_id = 2;
  string locale_id = 3;
  string entry_id = 4;
  string user_id = 5;
  string translation = 6;
  string action = 7;  // "created", "updated", "deleted"
  string changed_at = 8;
  User user = 9;      // User who made the change
}
```

#### Localization Service (proto/parlance/v1/localization.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service LocalizationService {
  rpc ListLocalizations(ListLocalizationsRequest) returns (ListLocalizationsResponse);
  rpc GetLocalization(GetLocalizationRequest) returns (Localization);
  rpc CreateLocalization(CreateLocalizationRequest) returns (Localization);
  rpc UpdateLocalization(UpdateLocalizationRequest) returns (Localization);
  rpc DeleteLocalization(DeleteLocalizationRequest) returns (Empty);
  rpc TranslateLocalization(TranslateLocalizationRequest) returns (Localization);
  rpc BatchTranslate(BatchTranslateRequest) returns (BatchTranslateResponse);
}

message Localization {
  string id = 1;
  string entry_id = 2;
  string locale_id = 3;
  string translation = 4;  // Changed from "value" to match schema
  string created_at = 5;
  string updated_at = 6;
  string created_by = 7;
  string updated_by = 8;
}

message ListLocalizationsRequest {
  string entry_id = 1;
  PaginationRequest pagination = 2;
}

message ListLocalizationsResponse {
  repeated Localization localizations = 1;
  PaginationResponse pagination = 2;
}

message CreateLocalizationRequest {
  string entry_id = 1;
  string locale_id = 2;
  string translation = 3;  // Changed from "value"
}

message UpdateLocalizationRequest {
  string id = 1;
  string translation = 2;
}

message DeleteLocalizationRequest {
  string id = 1;
}

message GetLocalizationRequest {
  string id = 1;
}

message TranslateLocalizationRequest {
  string entry_id = 1;
  string target_locale_id = 2;
  string source_locale_id = 3;
  bool apply_terminology = 4;
}

message BatchTranslateRequest {
  repeated string entry_ids = 1;
  string target_locale_id = 2;
  string source_locale_id = 3;
  bool apply_terminology = 4;
}
```

#### Terminology Service (proto/parlance/v1/terminology.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service TerminologyService {
  rpc ListTerminologies(ListTerminologiesRequest) returns (ListTerminologiesResponse);
  rpc GetTerminology(GetTerminologyRequest) returns (Terminology);
  rpc CreateTerminology(CreateTerminologyRequest) returns (Terminology);
  rpc UpdateTerminology(UpdateTerminologyRequest) returns (Terminology);
  rpc DeleteTerminology(DeleteTerminologyRequest) returns (Empty);
  rpc GetDefinitionHistory(GetDefinitionHistoryRequest) returns (DefinitionHistoryResponse);
}

message Terminology {
  string id = 1;
  string term = 2;
  string description = 3;
  repeated Definition definitions = 4;
  string created_at = 5;
  string updated_at = 6;
  string created_by = 7;
  string updated_by = 8;
}

message Definition {
  string id = 1;
  string terminology_id = 2;
  string locale_id = 3;
  string translation = 4;
  string created_at = 5;
  string updated_at = 6;
  string created_by = 7;
  string updated_by = 8;
}

message CreateTerminologyRequest {
  string term = 1;
  string description = 2;
}

message UpdateTerminologyRequest {
  string id = 1;
  string term = 2;
  string description = 3;
}

message DeleteTerminologyRequest {
  string id = 1;
}

message GetTerminologyRequest {
  string id = 1;
}

message ListTerminologiesRequest {
  PaginationRequest pagination = 1;
}

message ListTerminologiesResponse {
  repeated Terminology terminologies = 1;
  PaginationResponse pagination = 2;
}

message GetDefinitionHistoryRequest {
  string terminology_id = 1;
  PaginationRequest pagination = 2;
}

message DefinitionHistoryResponse {
  repeated DefinitionHistory history = 1;
  PaginationResponse pagination = 2;
}

message DefinitionHistory {
  string id = 1;
  string definition_id = 2;
  string locale_id = 3;
  string terminology_id = 4;
  string user_id = 5;
  string translation = 6;
  string action = 7;  // "created", "updated", "deleted"
  string changed_at = 8;
  User user = 9;
}
```

#### Scope Service (proto/parlance/v1/scope.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service ScopeService {
  rpc ListScopes(ListScopesRequest) returns (ListScopesResponse);
  rpc GetScope(GetScopeRequest) returns (Scope);
  rpc CreateScope(CreateScopeRequest) returns (Scope);
  rpc UpdateScope(UpdateScopeRequest) returns (Scope);
  rpc DeleteScope(DeleteScopeRequest) returns (Empty);

  // Entry-Scope relationship management
  rpc AddScopeToEntry(AddScopeToEntryRequest) returns (Empty);
  rpc RemoveScopeFromEntry(RemoveScopeFromEntryRequest) returns (Empty);
  rpc GetEntryScopes(GetEntryScopesRequest) returns (EntryScopesResponse);
}

message Scope {
  string id = 1;
  string name = 2;
  string slug = 3;
  string description = 4;
  string color = 5;  // Hex color code (e.g., "#FF5733")
  string created_at = 6;
  string updated_at = 7;
}

message CreateScopeRequest {
  string name = 1;
  string slug = 2;
  string description = 3;
  string color = 4;
}

message UpdateScopeRequest {
  string id = 1;
  string name = 2;
  string slug = 3;
  string description = 4;
  string color = 5;
}

message DeleteScopeRequest {
  string id = 1;
}

message GetScopeRequest {
  string id = 1;
}

message ListScopesRequest {
  PaginationRequest pagination = 1;
}

message ListScopesResponse {
  repeated Scope scopes = 1;
  PaginationResponse pagination = 2;
}

// Entry-Scope relationship management
message AddScopeToEntryRequest {
  string entry_id = 1;
  string scope_id = 2;
}

message RemoveScopeFromEntryRequest {
  string entry_id = 1;
  string scope_id = 2;
}

message GetEntryScopesRequest {
  string entry_id = 1;
}

message EntryScopesResponse {
  repeated Scope scopes = 1;
}
```

#### Locale Service (proto/parlance/v1/locale.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service LocaleService {
  rpc ListLocales(ListLocalesRequest) returns (ListLocalesResponse);
  rpc GetLocale(GetLocaleRequest) returns (Locale);
  rpc CreateLocale(CreateLocaleRequest) returns (Locale);
  rpc UpdateLocale(UpdateLocaleRequest) returns (Locale);
  rpc DeleteLocale(DeleteLocaleRequest) returns (Empty);
  rpc GetDefaultLocale(GetDefaultLocaleRequest) returns (Locale);
  rpc SetDefaultLocale(SetDefaultLocaleRequest) returns (Locale);
}

message Locale {
  string id = 1;
  string code = 2;          // e.g., "en", "de-DE", "fr-FR"
  string name = 3;          // e.g., "English", "German", "French"
  bool is_default = 4;      // Only one locale can be default
  string created_at = 5;
  string updated_at = 6;
}

message CreateLocaleRequest {
  string code = 1;
  string name = 2;
  bool is_default = 3;
}

message UpdateLocaleRequest {
  string id = 1;
  string code = 2;
  string name = 3;
  bool is_default = 4;
}

message DeleteLocaleRequest {
  string id = 1;
}

message GetLocaleRequest {
  string id = 1;
}

message ListLocalesRequest {
  PaginationRequest pagination = 1;
}

message ListLocalesResponse {
  repeated Locale locales = 1;
  PaginationResponse pagination = 2;
}

message GetDefaultLocaleRequest {}

message SetDefaultLocaleRequest {
  string locale_id = 1;
}
```

#### Export Service (proto/parlance/v1/export.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service ExportService {
  rpc ExportLaravel(ExportRequest) returns (ExportResponse);
  rpc ExportVueI18n(ExportRequest) returns (ExportResponse);
  rpc ExportJson(ExportRequest) returns (ExportResponse);
}

message ExportRequest {
  string locale_code = 1;
  string scope_id = 2;
}

message ExportResponse {
  string format = 1;          // "laravel", "vue-i18n", "json"
  string content = 2;         // Exported file content
  string filename = 3;        // Suggested filename
}
```

#### User Service (proto/parlance/v1/user.proto)
```protobuf
syntax = "proto3";
package parlance.v1;

service UserService {
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc GetUser(GetUserRequest) returns (User);
  rpc CreateUser(CreateUserRequest) returns (User);
  rpc UpdateUser(UpdateUserRequest) returns (User);
  rpc DeleteUser(DeleteUserRequest) returns (Empty);
  rpc AssignRole(AssignRoleRequest) returns (User);
}

message User {
  string id = 1;
  string email = 2;
  string name = 3;
  repeated Role roles = 4;
  string created_at = 5;
  string updated_at = 6;
}

message Role {
  string id = 1;
  string name = 2;
  repeated Permission permissions = 3;
}

message Permission {
  string id = 1;
  string name = 2;
  string resource = 3;
  string action = 4;
}
```

### Connect RPC Endpoints

Services are mounted at:
```
POST /parlance.v1.AuthService/Login
POST /parlance.v1.AuthService/Register
POST /parlance.v1.EntryService/ListEntries
POST /parlance.v1.EntryService/CreateEntry
POST /parlance.v1.LocalizationService/TranslateLocalization
... (auto-generated from proto definitions)
```

## Implementation Phases

### Phase 1: Foundation (Week 1)
- [ ] Initialize Go module
- [ ] Set up project structure
- [ ] Install Buf CLI and configure (`buf.yaml`, `buf.gen.yaml`)
- [ ] Define proto files (common, entry, localization, locale, scope)
- [ ] Generate Go code from protos (`buf generate`)
- [ ] Configure PostgreSQL connection
- [ ] Create database migrations for core tables
- [ ] Implement basic database models (Entry, Localization, Locale)
- [ ] Set up Docker & docker-compose
- [ ] Basic Connect server with health check

### Phase 2: Core RPC Services (Week 2)
- [ ] Implement EntryService (CRUD operations)
- [ ] Implement LocalizationService (CRUD operations)
- [ ] Implement LocaleService (management, default locale)
- [ ] Implement ScopeService (management, entry-scope relationships)
- [ ] Add proto <-> DB model converters
- [ ] Add pagination, filtering, sorting
- [ ] Implement audit tracking (created_by, updated_by)
- [ ] Add protobuf validation
- [ ] Write unit tests for services

### Phase 3: Authentication & Authorization (Week 3)
- [ ] Define auth.proto and user.proto
- [ ] Implement AuthService (register, login, refresh, logout)
- [ ] Implement UserService (CRUD, role assignment)
- [ ] JWT token generation and validation
- [ ] Auth interceptor for Connect (middleware)
- [ ] RBAC system (roles, permissions)
- [ ] Permission checking interceptor
- [ ] Database models and migrations for users/roles

### Phase 4: AI Integration (Week 4)
- [ ] Define terminology.proto
- [ ] Implement TerminologyService
- [ ] DeepL API client integration
- [ ] Translation service with terminology application
- [ ] TranslateLocalization RPC method
- [ ] BatchTranslate RPC method
- [ ] Error handling & retry logic
- [ ] Terminology matching during translation

### Phase 5: Advanced Features (Week 5)
- [ ] Define export.proto
- [ ] Implement ExportService (Laravel, Vue i18n, JSON)
- [ ] Export format generators
- [ ] SearchEntries RPC method with full-text search
- [ ] Implement GetEntryHistory RPC method (localization_history)
- [ ] Implement GetDefinitionHistory RPC method (definition_history)
- [ ] History tracking service with automatic recording
- [ ] History middleware/hooks for auto-tracking changes

### Phase 6: Production Ready (Week 6)
- [ ] Comprehensive Connect error handling
- [ ] Rate limiting interceptor
- [ ] Caching (Redis) for read operations
- [ ] API documentation (Buf Schema Registry or generated docs)
- [ ] (Optional) Add gRPC-Gateway for traditional REST URLs
- [ ] (Optional) Generate OpenAPI/Swagger documentation
- [ ] gRPC reflection for debugging
- [ ] Performance optimization (connection pooling, caching)
- [ ] Security hardening (TLS, input validation)
- [ ] Production Docker build with proto compilation
- [ ] Health check and readiness probes
- [ ] Observability (metrics, tracing)

## Docker Setup

### Development (docker-compose.yml)
```yaml
services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=parlance
      - DB_PASSWORD=secret
      - DB_NAME=parlance
    depends_on:
      - postgres
    volumes:
      - .:/app

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=parlance
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=parlance
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

volumes:
  postgres_data:
```

### Multi-Stage Dockerfile
```dockerfile
# Proto generation stage
FROM bufbuild/buf:latest AS proto
WORKDIR /workspace
COPY buf.yaml buf.gen.yaml ./
COPY proto ./proto
RUN buf generate

# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy generated proto files
COPY --from=proto /workspace/gen ./gen

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/api .
EXPOSE 8080
CMD ["./api"]
```

## Configuration

### Environment Variables
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

## Buf Configuration

### buf.yaml (Proto Linting & Breaking Change Detection)
```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

### buf.gen.yaml (Code Generation)
```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/yourusername/parlance-server/gen
plugins:
  # Go code generation
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt:
      - paths=source_relative

  # Connect RPC for Go
  - remote: buf.build/connectrpc/go
    out: gen
    opt:
      - paths=source_relative
```

### Client buf.gen.yaml (TypeScript for Nuxt)
```yaml
version: v2
managed:
  enabled: true
plugins:
  # TypeScript message types
  - remote: buf.build/bufbuild/es
    out: gen
    opt:
      - target=ts

  # Connect RPC for TypeScript
  - remote: buf.build/connectrpc/es
    out: gen
    opt:
      - target=ts
```

## Development Workflow

### Initial Setup
```bash
# Clone server repository
git clone <server-repo-url>
cd parlance-server

# Install development tools
make install-tools

# Copy environment file
cp .env.example .env

# Generate code from proto files
make proto-gen

# Start services
docker-compose up -d

# Run migrations
make migrate-up

# View logs
docker-compose logs -f api
```

### Proto Development Workflow
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

### Makefile Commands
```makefile
.PHONY: dev build test migrate-up migrate-down proto-gen proto-lint

dev:
	docker-compose up

build:
	docker-compose build

test:
	go test -v ./...

proto-gen:
	buf generate

proto-lint:
	buf lint

proto-breaking:
	buf breaking --against '.git#branch=main'

migrate-up:
	migrate -path internal/database/migrations -database "postgresql://parlance:secret@localhost:5432/parlance?sslmode=disable" up

migrate-down:
	migrate -path internal/database/migrations -database "postgresql://parlance:secret@localhost:5432/parlance?sslmode=disable" down

lint:
	golangci-lint run

run:
	buf generate && go run cmd/api/main.go

install-tools:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Client Integration

### Nuxt Frontend Setup

#### 1. Install Dependencies
```bash
npm install @connectrpc/connect @connectrpc/connect-web
npm install -D @bufbuild/buf @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
```

#### 2. Generate TypeScript Code from Proto Files
```bash
# In Nuxt project
# Copy proto files from server or use git submodule
buf generate
```

#### 3. Create Connect Client (composables/useAPI.ts)
```typescript
import { createPromiseClient } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"
import { EntryService } from "~/gen/parlance/v1/entry_connect"
import { AuthService } from "~/gen/parlance/v1/auth_connect"
import { LocalizationService } from "~/gen/parlance/v1/localization_connect"
// ... import other services

export const useAPI = () => {
  const config = useRuntimeConfig()
  const token = useCookie('auth_token')

  const transport = createConnectTransport({
    baseUrl: config.public.apiUrl || 'http://localhost:8080',
    interceptors: [
      // Add auth token to requests
      (next) => async (req) => {
        if (token.value) {
          req.header.set('Authorization', `Bearer ${token.value}`)
        }
        return await next(req)
      }
    ]
  })

  return {
    entry: createPromiseClient(EntryService, transport),
    auth: createPromiseClient(AuthService, transport),
    localization: createPromiseClient(LocalizationService, transport),
    // ... other services
  }
}
```

#### 4. Usage in Components
```vue
<script setup lang="ts">
const api = useAPI()

// List entries with type safety
const { data: entries } = await useAsyncData('entries', async () => {
  const response = await api.entry.listEntries({
    pagination: { page: 1, perPage: 20 },
    scopeId: 'frontend'
  })
  return response.entries
})

// Create entry
const createEntry = async (key: string, description: string) => {
  const entry = await api.entry.createEntry({
    key,
    scopeId: 'frontend',
    description
  })
  console.log('Created:', entry)
}

// Translate localization
const translate = async (entryId: string, targetLocale: string) => {
  const result = await api.localization.translateLocalization({
    entryId,
    targetLocaleId: targetLocale,
    sourceLocaleId: 'en',
    applyTerminology: true
  })
  return result
}
</script>
```

#### 5. Authentication Flow
```typescript
// composables/useAuth.ts
export const useAuth = () => {
  const api = useAPI()
  const token = useCookie('auth_token')
  const user = useState('user')

  const login = async (email: string, password: string) => {
    const response = await api.auth.login({ email, password })
    token.value = response.accessToken
    user.value = response.user
  }

  const logout = async () => {
    await api.auth.logout({})
    token.value = null
    user.value = null
  }

  return { login, logout, user }
}
```

### Electron App

Same setup as Nuxt, but with different base URL:

```typescript
const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_URL || 'https://api.parlance.app',
  // ... same interceptors
})
```

### Benefits of Connect RPC

1. **Type Safety**: Auto-generated TypeScript types from proto files
2. **Single Source of Truth**: Proto definitions shared between server and client
3. **Better DX**: Method calls instead of HTTP endpoints
4. **Error Handling**: Structured error responses
5. **Streaming Support**: Server/client streaming when needed (future feature)
6. **No CORS Issues**: Works over standard HTTP
7. **Works Everywhere**: Web browsers, Node.js, Electron, React Native

## History Tracking Implementation

### Automatic History Recording with GORM Hooks

The database schema includes dedicated history tables (`localization_history`, `definition_history`) that automatically track all changes.

#### GORM Model with Audit Tracking

**internal/models/localization.go:**
```go
package models

import (
  "time"
  "gorm.io/gorm"
)

type Localization struct {
  ID          string         `gorm:"primaryKey"`
  LocaleID    string         `gorm:"not null;index"`
  EntryID     string         `gorm:"not null;index"`
  Translation string         `gorm:"type:text;not null"`
  CreatedAt   time.Time
  UpdatedAt   time.Time
  DeletedAt   gorm.DeletedAt `gorm:"index"`
  CreatedBy   *string        `gorm:"index"`
  UpdatedBy   *string        `gorm:"index"`

  // Relationships
  Locale Locale `gorm:"foreignKey:LocaleID"`
  Entry  Entry  `gorm:"foreignKey:EntryID"`
}

type LocalizationHistory struct {
  ID              string    `gorm:"primaryKey"`
  LocalizationID  string    `gorm:"not null;index"`
  LocaleID        string    `gorm:"not null"`
  EntryID         string    `gorm:"not null;index"`
  UserID          *string
  Translation     string    `gorm:"type:text;not null"`
  Action          string    `gorm:"type:varchar(20);not null"` // created, updated, deleted
  ChangedAt       time.Time `gorm:"not null;index"`

  // Relationships
  Localization Localization `gorm:"foreignKey:LocalizationID"`
  User         User         `gorm:"foreignKey:UserID"`
}

// GORM Hooks for automatic history tracking
func (l *Localization) AfterCreate(tx *gorm.DB) error {
  return recordHistory(tx, l, "created")
}

func (l *Localization) AfterUpdate(tx *gorm.DB) error {
  return recordHistory(tx, l, "updated")
}

func (l *Localization) AfterDelete(tx *gorm.DB) error {
  return recordHistory(tx, l, "deleted")
}

func recordHistory(tx *gorm.DB, l *Localization, action string) error {
  history := LocalizationHistory{
    LocalizationID: l.ID,
    LocaleID:       l.LocaleID,
    EntryID:        l.EntryID,
    UserID:         l.UpdatedBy, // or CreatedBy depending on action
    Translation:    l.Translation,
    Action:         action,
    ChangedAt:      time.Now(),
  }
  return tx.Create(&history).Error
}
```

#### History Service Implementation

**internal/services/history.go:**
```go
package services

import (
  "context"
  "github.com/yourusername/parlance-server/internal/models"
  "gorm.io/gorm"
)

type HistoryService struct {
  db *gorm.DB
}

func NewHistoryService(db *gorm.DB) *HistoryService {
  return &HistoryService{db: db}
}

func (s *HistoryService) GetLocalizationHistory(ctx context.Context, entryID string, page, perPage int) ([]models.LocalizationHistory, int64, error) {
  var history []models.LocalizationHistory
  var total int64

  query := s.db.Where("entry_id = ?", entryID).
    Preload("User").
    Order("changed_at DESC")

  query.Model(&models.LocalizationHistory{}).Count(&total)

  offset := (page - 1) * perPage
  err := query.Offset(offset).Limit(perPage).Find(&history).Error

  return history, total, err
}

func (s *HistoryService) GetDefinitionHistory(ctx context.Context, terminologyID string, page, perPage int) ([]models.DefinitionHistory, int64, error) {
  var history []models.DefinitionHistory
  var total int64

  query := s.db.Where("terminology_id = ?", terminologyID).
    Preload("User").
    Order("changed_at DESC")

  query.Model(&models.DefinitionHistory{}).Count(&total)

  offset := (page - 1) * perPage
  err := query.Offset(offset).Limit(perPage).Find(&history).Error

  return history, total, err
}
```

#### Context-Aware User Tracking

**internal/interceptors/auth.go:**
```go
func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
  return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
    // ... validate JWT ...

    userID, err := validateJWT(token, i.jwtSecret)
    if err != nil {
      return nil, connect.NewError(connect.CodeUnauthenticated, err)
    }

    // Add user ID to context for audit tracking
    ctx = context.WithValue(ctx, "user_id", userID)

    return next(ctx, req)
  }
}
```

**Usage in service:**
```go
func (s *LocalizationServer) UpdateLocalization(
  ctx context.Context,
  req *connect.Request[localizationv1.UpdateLocalizationRequest],
) (*connect.Response[localizationv1.Localization], error) {
  userID := ctx.Value("user_id").(string)

  localization.Translation = req.Msg.Translation
  localization.UpdatedBy = &userID  // Audit tracking

  s.db.Save(&localization)  // Triggers AfterUpdate hook -> history recorded

  return connect.NewResponse(localizationToProto(localization)), nil
}
```

### History Query Examples

**Get all changes to an entry:**
```bash
# Connect RPC
curl -X POST http://localhost:8080/parlance.v1.EntryService/GetEntryHistory \
  -H "Content-Type: application/json" \
  -d '{"entry_id": "123", "pagination": {"page": 1, "per_page": 20}}'

# REST (with gRPC-Gateway)
curl "http://localhost:8080/api/v1/entries/123/history?page=1&per_page=20"
```

**Response:**
```json
{
  "history": [
    {
      "id": "hist_001",
      "localization_id": "loc_123",
      "locale_id": "de",
      "entry_id": "123",
      "user_id": "user_456",
      "translation": "Anmelden",
      "action": "updated",
      "changed_at": "2024-03-09T10:30:00Z",
      "user": {
        "id": "user_456",
        "name": "John Doe",
        "email": "john@example.com"
      }
    }
  ],
  "pagination": {
    "total": 15,
    "page": 1,
    "per_page": 20,
    "total_pages": 1
  }
}
```

## Connect RPC Implementation Example

### Service Implementation (internal/server/entry.go)
```go
package server

import (
  "context"
  "connectrpc.com/connect"

  entryv1 "github.com/yourusername/parlance-server/gen/parlance/v1"
  "github.com/yourusername/parlance-server/internal/models"
  "github.com/yourusername/parlance-server/internal/database"
)

type EntryServer struct {
  db *database.DB
}

func NewEntryServer(db *database.DB) *EntryServer {
  return &EntryServer{db: db}
}

// ListEntries implements EntryService.ListEntries
func (s *EntryServer) ListEntries(
  ctx context.Context,
  req *connect.Request[entryv1.ListEntriesRequest],
) (*connect.Response[entryv1.ListEntriesResponse], error) {
  // Get pagination params
  page := req.Msg.Pagination.Page
  perPage := req.Msg.Pagination.PerPage

  // Query database
  var entries []models.Entry
  var total int64

  query := s.db.Model(&models.Entry{})
  if req.Msg.ScopeId != "" {
    query = query.Where("scope_id = ?", req.Msg.ScopeId)
  }

  query.Count(&total)
  query.Offset(int((page - 1) * perPage)).Limit(int(perPage)).Find(&entries)

  // Convert to proto messages
  protoEntries := make([]*entryv1.Entry, len(entries))
  for i, entry := range entries {
    protoEntries[i] = entryToProto(&entry)
  }

  // Return response
  return connect.NewResponse(&entryv1.ListEntriesResponse{
    Entries: protoEntries,
    Pagination: &entryv1.PaginationResponse{
      Total: int32(total),
      Page: page,
      PerPage: perPage,
      TotalPages: int32((total + int64(perPage) - 1) / int64(perPage)),
    },
  }), nil
}

// CreateEntry implements EntryService.CreateEntry
func (s *EntryServer) CreateEntry(
  ctx context.Context,
  req *connect.Request[entryv1.CreateEntryRequest],
) (*connect.Response[entryv1.Entry], error) {
  entry := &models.Entry{
    Key: req.Msg.Key,
    ScopeID: req.Msg.ScopeId,
    Description: req.Msg.Description,
  }

  if err := s.db.Create(entry).Error; err != nil {
    return nil, connect.NewError(connect.CodeInternal, err)
  }

  return connect.NewResponse(entryToProto(entry)), nil
}

// Helper function to convert DB model to proto
func entryToProto(entry *models.Entry) *entryv1.Entry {
  return &entryv1.Entry{
    Id: entry.ID,
    Key: entry.Key,
    ScopeId: entry.ScopeID,
    Description: entry.Description,
    CreatedAt: entry.CreatedAt.String(),
    UpdatedAt: entry.UpdatedAt.String(),
  }
}
```

### Main Server Setup (cmd/api/main.go)
```go
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

  entryv1connect "github.com/yourusername/parlance-server/gen/parlance/v1/entryconnect"
  authv1connect "github.com/yourusername/parlance-server/gen/parlance/v1/authconnect"

  "github.com/yourusername/parlance-server/internal/config"
  "github.com/yourusername/parlance-server/internal/database"
  "github.com/yourusername/parlance-server/internal/server"
  "github.com/yourusername/parlance-server/internal/interceptors"
)

func main() {
  // Load configuration
  cfg := config.Load()

  // Connect to database
  db, err := database.Connect(cfg.Database)
  if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
  }

  // Create servers
  entryServer := server.NewEntryServer(db)
  authServer := server.NewAuthServer(db, cfg.JWT)

  // Create interceptors
  authInterceptor := interceptors.NewAuthInterceptor(cfg.JWT)
  loggingInterceptor := interceptors.NewLoggingInterceptor()

  // Create mux
  mux := http.NewServeMux()

  // Register services with interceptors
  mux.Handle(entryv1connect.NewEntryServiceHandler(
    entryServer,
    connect.WithInterceptors(loggingInterceptor, authInterceptor),
  ))

  mux.Handle(authv1connect.NewAuthServiceHandler(
    authServer,
    connect.WithInterceptors(loggingInterceptor),
  ))

  // Health check
  mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
  })

  // Create server with h2c (HTTP/2 without TLS for development)
  addr := fmt.Sprintf(":%s", cfg.Port)
  srv := &http.Server{
    Addr: addr,
    Handler: h2c.NewHandler(mux, &http2.Server{}),
  }

  // Graceful shutdown
  go func() {
    log.Printf("Server starting on %s", addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
      log.Fatalf("Server failed: %v", err)
    }
  }()

  // Wait for interrupt signal
  quit := make(chan os.Signal, 1)
  signal.Notify(quit, os.Interrupt)
  <-quit

  log.Println("Shutting down server...")
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  if err := srv.Shutdown(ctx); err != nil {
    log.Fatalf("Server forced to shutdown: %v", err)
  }

  log.Println("Server exited")
}
```

### Auth Interceptor (internal/interceptors/auth.go)
```go
package interceptors

import (
  "context"
  "strings"

  "connectrpc.com/connect"
)

type AuthInterceptor struct {
  jwtSecret string
}

func NewAuthInterceptor(jwtSecret string) *AuthInterceptor {
  return &AuthInterceptor{jwtSecret: jwtSecret}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
  return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
    // Skip auth for certain methods
    if isPublicMethod(req.Spec().Procedure) {
      return next(ctx, req)
    }

    // Get token from header
    token := req.Header().Get("Authorization")
    if token == "" {
      return nil, connect.NewError(connect.CodeUnauthenticated, nil)
    }

    // Remove "Bearer " prefix
    token = strings.TrimPrefix(token, "Bearer ")

    // Validate token (implement JWT validation)
    userID, err := validateJWT(token, i.jwtSecret)
    if err != nil {
      return nil, connect.NewError(connect.CodeUnauthenticated, err)
    }

    // Add user ID to context
    ctx = context.WithValue(ctx, "user_id", userID)

    return next(ctx, req)
  }
}

func isPublicMethod(procedure string) bool {
  publicMethods := []string{
    "/parlance.v1.AuthService/Login",
    "/parlance.v1.AuthService/Register",
  }

  for _, method := range publicMethods {
    if procedure == method {
      return true
    }
  }
  return false
}
```

## Testing Strategy

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test API endpoints with test database
3. **E2E Tests**: Test complete workflows
4. **Load Tests**: Performance testing with k6 or vegeta

## Security Considerations

- [ ] Input validation on all endpoints
- [ ] SQL injection prevention (use parameterized queries)
- [ ] XSS prevention (sanitize output)
- [ ] CSRF protection
- [ ] Rate limiting per IP/user
- [ ] Secure password hashing (bcrypt)
- [ ] Environment variable secrets
- [ ] HTTPS in production
- [ ] Security headers (helmet equivalent)
- [ ] Database connection pooling

## Deployment

### Production Considerations
- Use managed PostgreSQL (AWS RDS, DigitalOcean)
- Container registry (Docker Hub, GHCR, ECR)
- Orchestration (Kubernetes, Docker Swarm)
- Reverse proxy (Nginx, Traefik)
- Monitoring (Prometheus, Grafana)
- Logging (ELK stack, Loki)
- CI/CD (GitHub Actions, GitLab CI)

## Why Connect RPC over REST?

| Aspect | REST | Connect RPC |
|--------|------|-------------|
| **Type Safety** | Manual (OpenAPI/TypeScript) | Auto-generated from proto |
| **API Contract** | Documentation (Swagger) | Proto files (source of truth) |
| **Client Code** | Manual axios/fetch calls | Type-safe method calls |
| **Validation** | Manual validators | Built-in proto validation |
| **Breaking Changes** | Manual checking | Automated (`buf breaking`) |
| **Multi-Client** | Need to maintain consistency | Single proto, multiple clients |
| **Streaming** | WebSocket/SSE | Native bidirectional streaming |
| **Error Handling** | HTTP status codes | Structured error responses |
| **Browser Support** | ✅ Native | ✅ Native (HTTP/1.1 compatible) |
| **Learning Curve** | Low | Medium |

**Decision**: Connect RPC provides better type safety and developer experience for multi-client (web + Electron) architecture, with minimal additional complexity.

## Dual Protocol Support: RPC + REST

### Connect's Built-in JSON/HTTP Support

**Good News**: Connect automatically supports both protocols with zero extra code!

#### What You Get Out of the Box

1. **JSON over HTTP/1.1** (REST-like)
2. **Binary Protobuf over HTTP/1.1**
3. **Binary Protobuf over HTTP/2** (gRPC-compatible)

The same Go handler serves all three formats - just change the `Content-Type` header!

#### Example: Same Endpoint, Multiple Formats

```bash
# JSON request (REST-like)
curl -X POST http://localhost:8080/parlance.v1.EntryService/GetEntry \
  -H "Content-Type: application/json" \
  -d '{"id": "123"}'

# Binary Protobuf request
curl -X POST http://localhost:8080/parlance.v1.EntryService/GetEntry \
  -H "Content-Type: application/proto" \
  --data-binary @request.bin

# Both hit the same Go handler!
```

#### Client Configuration

**TypeScript/Nuxt (JSON):**
```typescript
const transport = createConnectTransport({
  baseUrl: "http://localhost:8080",
  useBinaryFormat: false,  // Use JSON
})
```

**Go Client (Protobuf):**
```go
client := entryv1connect.NewEntryServiceClient(
  http.DefaultClient,
  "http://localhost:8080",
  connect.WithGRPC(),  // Use binary protobuf
)
```

### Optional: Traditional REST URLs with gRPC-Gateway

If you need **traditional RESTful paths** like:
```
GET    /api/v1/entries
GET    /api/v1/entries/123
POST   /api/v1/entries
PUT    /api/v1/entries/123
DELETE /api/v1/entries/123
```

Instead of Connect's RPC-style:
```
POST /parlance.v1.EntryService/ListEntries
POST /parlance.v1.EntryService/GetEntry
POST /parlance.v1.EntryService/CreateEntry
```

You can add **gRPC-Gateway** to serve **both** simultaneously.

#### When to Add gRPC-Gateway

**Add it if you need:**
- ✅ Traditional REST URLs for public API
- ✅ HTTP method semantics (GET/POST/PUT/DELETE)
- ✅ Third-party integrations expecting REST
- ✅ Auto-generated OpenAPI/Swagger documentation
- ✅ SEO-friendly URLs with GET requests
- ✅ Better HTTP caching (GET requests)

**Skip it if:**
- ❌ Only internal API (Nuxt + Electron clients)
- ❌ Type-safe clients are sufficient
- ❌ Don't need OpenAPI docs
- ❌ Want simpler setup

#### Implementation: Connect + gRPC-Gateway

**Step 1: Add HTTP Annotations to Proto Files**

```protobuf
syntax = "proto3";
package parlance.v1;

import "google/api/annotations.proto";

service EntryService {
  rpc ListEntries(ListEntriesRequest) returns (ListEntriesResponse) {
    option (google.api.http) = {
      get: "/api/v1/entries"
    };
  }

  rpc GetEntry(GetEntryRequest) returns (Entry) {
    option (google.api.http) = {
      get: "/api/v1/entries/{id}"
    };
  }

  rpc CreateEntry(CreateEntryRequest) returns (Entry) {
    option (google.api.http) = {
      post: "/api/v1/entries"
      body: "*"
    };
  }

  rpc UpdateEntry(UpdateEntryRequest) returns (Entry) {
    option (google.api.http) = {
      put: "/api/v1/entries/{id}"
      body: "*"
    };
  }

  rpc DeleteEntry(DeleteEntryRequest) returns (Empty) {
    option (google.api.http) = {
      delete: "/api/v1/entries/{id}"
    };
  }

  rpc SearchEntries(SearchEntriesRequest) returns (ListEntriesResponse) {
    option (google.api.http) = {
      get: "/api/v1/entries/search"
    };
  }
}

message GetEntryRequest {
  string id = 1;  // Path parameter from URL
}

message UpdateEntryRequest {
  string id = 1;         // Path parameter
  string key = 2;        // Body fields
  string scope_id = 3;
  string description = 4;
}
```

**Step 2: Update buf.gen.yaml**

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/yourusername/parlance-server/gen
plugins:
  # Go protobuf
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt:
      - paths=source_relative

  # Connect RPC
  - remote: buf.build/connectrpc/go
    out: gen
    opt:
      - paths=source_relative

  # gRPC-Gateway (REST endpoints) - OPTIONAL
  - remote: buf.build/grpc-ecosystem/gateway
    out: gen
    opt:
      - paths=source_relative
      - generate_unbound_methods=true

  # OpenAPI/Swagger docs - OPTIONAL
  - remote: buf.build/grpc-ecosystem/openapiv2
    out: gen/openapi
```

**Step 3: Update main.go to Serve Both**

```go
package main

import (
  "context"
  "log"
  "net/http"

  "connectrpc.com/connect"
  "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
  "golang.org/x/net/http2"
  "golang.org/x/net/http2/h2c"

  entryv1 "github.com/yourusername/parlance-server/gen/parlance/v1"
  entryv1connect "github.com/yourusername/parlance-server/gen/parlance/v1/entryconnect"

  "github.com/yourusername/parlance-server/internal/config"
  "github.com/yourusername/parlance-server/internal/database"
  "github.com/yourusername/parlance-server/internal/server"
  "github.com/yourusername/parlance-server/internal/interceptors"
)

func main() {
  cfg := config.Load()
  db, err := database.Connect(cfg.Database)
  if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
  }

  // Create servers
  entryServer := server.NewEntryServer(db)
  authServer := server.NewAuthServer(db, cfg.JWT)

  // Create interceptors
  authInterceptor := interceptors.NewAuthInterceptor(cfg.JWT)
  loggingInterceptor := interceptors.NewLoggingInterceptor()

  mux := http.NewServeMux()

  // ===========================================
  // 1. Connect RPC Endpoints (JSON + Protobuf)
  // ===========================================
  // Routes: POST /parlance.v1.EntryService/*
  path, handler := entryv1connect.NewEntryServiceHandler(
    entryServer,
    connect.WithInterceptors(loggingInterceptor, authInterceptor),
  )
  mux.Handle(path, handler)

  // Register other Connect services...
  // authPath, authHandler := authv1connect.NewAuthServiceHandler(authServer)
  // mux.Handle(authPath, authHandler)

  // ===========================================
  // 2. gRPC-Gateway REST Endpoints (OPTIONAL)
  // ===========================================
  // Routes: GET/POST/PUT/DELETE /api/v1/*
  gwMux := runtime.NewServeMux(
    runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
      // Forward auth headers
      md := metadata.MD{}
      if auth := req.Header.Get("Authorization"); auth != "" {
        md.Set("authorization", auth)
      }
      return md
    }),
  )

  // Register REST handlers
  err = entryv1.RegisterEntryServiceHandlerServer(context.Background(), gwMux, entryServer)
  if err != nil {
    log.Fatalf("Failed to register gateway: %v", err)
  }

  // Mount REST API under /api/
  mux.Handle("/api/", http.StripPrefix("/api", gwMux))

  // Health check
  mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
  })

  // Serve with h2c (HTTP/2 without TLS)
  addr := fmt.Sprintf(":%s", cfg.Port)
  srv := &http.Server{
    Addr:    addr,
    Handler: h2c.NewHandler(mux, &http2.Server{}),
  }

  log.Printf("Server starting on %s", addr)
  log.Printf("Connect RPC: POST /parlance.v1.*")
  log.Printf("REST API:    GET/POST/PUT/DELETE /api/v1/*")

  if err := srv.ListenAndServe(); err != nil {
    log.Fatalf("Server failed: %v", err)
  }
}
```

**Step 4: Now You Support Both!**

```bash
# Connect RPC (JSON) - Type-safe client
curl -X POST http://localhost:8080/parlance.v1.EntryService/GetEntry \
  -H "Content-Type: application/json" \
  -d '{"id": "123"}'

# Traditional REST
curl http://localhost:8080/api/v1/entries/123

# REST with query params
curl "http://localhost:8080/api/v1/entries?page=1&per_page=20"

# REST create
curl -X POST http://localhost:8080/api/v1/entries \
  -H "Content-Type: application/json" \
  -d '{"key": "auth.login", "description": "Login page"}'

# All hit the same Go handlers!
```

#### Auto-Generated OpenAPI Documentation

With gRPC-Gateway, you get Swagger docs automatically:

```bash
# After buf generate
ls gen/openapi/
# parlance/v1/entry.swagger.json
# parlance/v1/auth.swagger.json
```

Serve with Swagger UI:
```go
import "github.com/go-chi/chi/v5"
import httpSwagger "github.com/swaggo/http-swagger"

router := chi.NewRouter()
router.Get("/swagger/*", httpSwagger.Handler(
  httpSwagger.URL("/openapi/parlance/v1/entry.swagger.json"),
))
```

### Comparison: Connect Only vs Connect + gRPC-Gateway

| Feature | Connect Only | Connect + Gateway |
|---------|--------------|------------------|
| **JSON Support** | ✅ Built-in | ✅ Built-in |
| **Protobuf Support** | ✅ Built-in | ✅ Built-in |
| **RESTful URLs** | ❌ RPC-style | ✅ `/api/v1/entries/123` |
| **HTTP Methods** | POST only | ✅ GET/POST/PUT/DELETE |
| **Setup Complexity** | Low | Medium |
| **Code Generation** | 2 plugins | 4 plugins |
| **OpenAPI Docs** | Manual | ✅ Auto-generated |
| **Public API** | Good | Better |
| **Maintenance** | Lower | Higher |

### Recommendation

**Phase 1-5 (MVP)**: Use **Connect Only**
- Simpler setup
- JSON support for web clients
- Protobuf support for performance
- Sufficient for Nuxt + Electron

**Phase 6 (Production)**: Add **gRPC-Gateway** if needed
- Public API requirements emerge
- Third-party integrations needed
- OpenAPI documentation required
- Traditional REST expectations

## Next Steps

1. Create `parlance-server` repository
2. Initialize Go module (`go mod init`)
3. Set up basic project structure
4. Install Buf CLI and create proto files
5. Configure `buf.yaml` and `buf.gen.yaml`
6. Define initial proto services (entry, auth, locale)
7. Generate Go code (`buf generate`)
8. Implement Phase 1 (Foundation)
9. Set up Nuxt client with Connect-Web
10. Generate TypeScript code from protos

## Resources

### Go & Connect RPC
- [Go Best Practices](https://go.dev/doc/effective_go)
- [Connect RPC Documentation](https://connectrpc.com/docs/go/getting-started)
- [gRPC-Gateway Documentation](https://grpc-ecosystem.github.io/grpc-gateway/)
- [Google API HTTP Annotations](https://github.com/googleapis/googleapis/blob/master/google/api/http.proto)
- [Buf CLI Documentation](https://buf.build/docs)
- [Protocol Buffers Guide](https://protobuf.dev/programming-guides/proto3/)
- [GORM Documentation](https://gorm.io/docs/)

### Frontend Integration
- [Connect-Web for TypeScript](https://connectrpc.com/docs/web/getting-started)
- [Nuxt 3 Documentation](https://nuxt.com/docs)
- [Protocol Buffers in TypeScript](https://github.com/bufbuild/protobuf-es)

### Infrastructure
- [PostgreSQL Best Practices](https://wiki.postgresql.org/wiki/Don%27t_Do_This)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [golang-migrate](https://github.com/golang-migrate/migrate)

### Example Projects
- [Connect Examples](https://github.com/connectrpc/examples-go)
- [Buf Examples](https://github.com/bufbuild/buf-examples)

---

**Estimated Timeline**: 6 weeks for MVP
**Team Size**: 1-2 developers
**Complexity**: Medium
