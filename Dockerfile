# Development stage - includes linting and testing tools
FROM golang:1.25-alpine@sha256:d9b2e14101f27ec8d09674cd01186798d227bb0daec90e032aeb1cd22ac0f029 AS dev

RUN apk add --no-cache git gcc musl-dev

RUN go install mvdan.cc/gofumpt@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install github.com/securego/gosec/v2/cmd/gosec@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["go", "test", "-v", "./..."]

# Build stage
FROM golang:1.25-alpine@sha256:d9b2e14101f27ec8d09674cd01186798d227bb0daec90e032aeb1cd22ac0f029 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o webhook cmd/webhook/main.go

# Production stage
FROM alpine:3.23@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62 AS production

WORKDIR /app

RUN apk add --no-cache ca-certificates && \
    adduser -D -g '' appuser

COPY --from=builder /app/webhook .

USER appuser

ENV PORT=8080
EXPOSE 8080

CMD ["./webhook"]
