.PHONY: dev build test migrate-up migrate-down proto-gen proto-lint proto-breaking install-tools run lint clean

# Development
dev:
	docker-compose up

build:
	docker-compose build

run:
	buf generate && go run cmd/api/main.go

# Testing
test:
	go test -v ./...

# Proto
proto-gen:
	buf generate

proto-lint:
	buf lint

proto-breaking:
	buf breaking --against '.git#branch=main'

# Database migrations
migrate-up:
	migrate -path internal/database/migrations -database "postgresql://parlance:secret@localhost:5432/parlance?sslmode=disable" up

migrate-down:
	migrate -path internal/database/migrations -database "postgresql://parlance:secret@localhost:5432/parlance?sslmode=disable" down

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir internal/database/migrations -seq $$name

# Linting
lint:
	golangci-lint run

# Tools installation
install-tools:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Cleanup
clean:
	rm -rf gen/parlance
	go clean
