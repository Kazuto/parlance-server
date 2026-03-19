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
RUN apk add --no-cache git ca-certificates tzdata

# Copy generated proto files
COPY --from=proto /workspace/gen ./gen

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o api ./cmd/api

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/api .

# Copy migrations (if needed at runtime)
COPY --from=builder /app/internal/database/migrations ./migrations

EXPOSE 8080

# Run the application
CMD ["./api"]
