# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build API server and migration runner
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o migrations cmd/migrations/main.go

# Production stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binaries
COPY --from=builder /app/api .
COPY --from=builder /app/migrations .

# Copy SQL migrations
COPY --from=builder /app/migrations ./migrations

# Expose API port
EXPOSE 8080

CMD ["./api"]
