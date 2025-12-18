# Build stage
FROM golang:1.25@sha256:36b4f45d2874905b9e8573b783292629bcb346d0a70d8d7150b6df545234818f AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with explicit architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook cmd/webhook/main.go

# Final stage
FROM alpine:3.23@sha256:be171b562d67532ea8b3c9d1fc0904288818bb36fc8359f954a7b7f1f9130fb2

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
