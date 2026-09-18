# Development stage - includes linting and testing tools
FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS dev

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
FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS builder

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
