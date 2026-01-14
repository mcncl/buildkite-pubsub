# Build stage
FROM golang:1.25@sha256:8bbd14091f2c61916134fa6aeb8f76b18693fcb29a39ec6d8be9242c0a7e9260 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with explicit architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook cmd/webhook/main.go

# Final stage
FROM alpine:3.23@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62

WORKDIR /app

# Add CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/webhook .

# Run as non-root user
RUN adduser -D -g '' appuser
USER appuser

# Environment variable for the port
ENV PORT=8080

# Expose port
EXPOSE 8080

# Run
CMD ["./webhook"]
