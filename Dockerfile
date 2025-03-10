# Build stage
FROM golang:1.24@sha256:c5adecdb7b3f8c5ca3c88648a861882849cc8b02fed68ece31e25de88ad13418 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with explicit architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook cmd/webhook/main.go

# Final stage
FROM alpine:3.21@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c

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
