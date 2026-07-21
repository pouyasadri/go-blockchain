# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Build static binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/node ./cmd/node
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/indexer ./cmd/indexer
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/mcp-daemon ./cmd/mcp-daemon
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/sign-policy ./cmd/sign-policy

# Final Minimal Stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy compiled binaries and assets from builder stage
COPY --from=builder /app/bin/* /usr/local/bin/
COPY --from=builder /app/internal/dashboard/assets /app/assets

EXPOSE 5000 50051 8080

ENTRYPOINT ["/usr/local/bin/node"]
