# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/ommr-server ./cmd/server

# Final Runtime Stage
FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/ommr-server /app/ommr-server
COPY --from=builder /app/docs /app/docs
COPY --from=builder /app/ARCHITECTURE_DECISIONS.md /app/ARCHITECTURE_DECISIONS.md

EXPOSE 8080

ENTRYPOINT ["/app/ommr-server"]
