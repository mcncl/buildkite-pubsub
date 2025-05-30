# Build stage
FROM golang:1.24@sha256:4c0a1814a7c6c65ece28b3bfea14ee3cf83b5e80b81418453f0e9d5255a5d7b8 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with explicit architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook cmd/webhook/main.go

# Final stage
FROM alpine:3.22@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715

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
