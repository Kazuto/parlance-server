.PHONY: dev dev-stop dev-down dev-logs build test test-coverage migrate-up migrate-down migrate-create proto-gen proto-lint proto-breaking install-tools run lint fmt fmt-check vet ci seed seed-fresh clean

# Development
dev:
	docker-compose -f docker/docker-compose.yml up -d

dev-stop:
	docker-compose -f docker/docker-compose.yml stop

dev-down:
	docker-compose -f docker/docker-compose.yml down

dev-logs:
	docker-compose -f docker/docker-compose.yml logs -f

build:
	docker-compose -f docker/docker-compose.yml build

run:
	buf generate && go run cmd/api/main.go

# Testing
test:
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test
	go tool cover -html=coverage.out

# Proto
proto-gen:
	buf generate

proto-lint:
	buf lint

proto-breaking:
	buf breaking --against '.git#branch=main'

# Database seeding
seed:
	@echo "Stopping API container..."
	docker-compose -f docker/docker-compose.yml stop api 2>/dev/null || true
	@echo "Dropping and recreating database..."
	PGPASSWORD=secret psql -h localhost -U parlance -d postgres -c "DROP DATABASE IF EXISTS parlance WITH (FORCE);" || true
	PGPASSWORD=secret psql -h localhost -U parlance -d postgres -c "CREATE DATABASE parlance;"
	@echo "Running migrations and seeding..."
	go run cmd/seed/main.go
	@echo "Restarting API container..."
	docker-compose -f docker/docker-compose.yml start api 2>/dev/null || true

# Formatting
fmt:
	gofmt -w .

fmt-check:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	fi

# Vetting
vet:
	go vet ./...

# Linting
lint:
	golangci-lint run

# CI - Run all checks
ci: fmt-check vet test

# Tools installation
install-tools:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Cleanup
clean:
	rm -rf gen/parlance
	go clean
