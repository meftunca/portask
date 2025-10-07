# Multi-stage build for optimal image size

# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty) -X main.buildTime=$(date +%Y-%m-%dT%H:%M:%S%z)" \
    -a -installsuffix cgo \
    -o portask ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 portask && \
    adduser -D -u 1000 -G portask portask

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/portask .
COPY --from=builder /build/configs ./configs

# Change ownership
RUN chown -R portask:portask /app

# Switch to non-root user
USER portask

# Expose ports
EXPOSE 8080 9092 5672 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables
ENV PORTASK_CONFIG_PATH=/app/configs/config.yaml

# Run the application
ENTRYPOINT ["./portask"]

