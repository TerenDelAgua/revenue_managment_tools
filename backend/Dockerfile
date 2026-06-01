# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build API server, migration runner, and seed tool
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o run-migrations cmd/migrations/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o seed cmd/seed/main.go

# Production stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata bash dos2unix postgresql-client

WORKDIR /app

# Copy binaries
COPY --from=builder /app/api .
COPY --from=builder /app/run-migrations .
COPY --from=builder /app/seed .

# Copy entrypoint script
COPY entrypoint.sh .

RUN dos2unix entrypoint.sh && chmod +x /app/api /app/run-migrations /app/seed /app/entrypoint.sh

# Copy SQL migrations and seeds
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/seeds ./seeds

# Expose API port
EXPOSE 8080

CMD ["./entrypoint.sh"]
