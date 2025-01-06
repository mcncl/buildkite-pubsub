# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook cmd/webhook/main.go

# Final stage
FROM alpine:3.18

WORKDIR /app

# Add CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/webhook .

# Run as non-root user
RUN adduser -D -g '' appuser
USER appuser

# Expose port
EXPOSE 8080

# Run
CMD ["./webhook"]
