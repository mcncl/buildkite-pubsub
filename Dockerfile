# Development stage - includes linting and testing tools
FROM golang:1.25-alpine@sha256:98e6cffc31ccc44c7c15d83df1d69891efee8115a5bb7ede2bf30a38af3e3c92 AS dev

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
FROM golang:1.25-alpine@sha256:98e6cffc31ccc44c7c15d83df1d69891efee8115a5bb7ede2bf30a38af3e3c92 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o webhook cmd/webhook/main.go

# Production stage
FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS production

WORKDIR /app

RUN apk add --no-cache ca-certificates && \
    adduser -D -g '' appuser

COPY --from=builder /app/webhook .

USER appuser

ENV PORT=8080
EXPOSE 8080

CMD ["./webhook"]
