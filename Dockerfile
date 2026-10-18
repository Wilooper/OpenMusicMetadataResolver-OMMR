# Build Stage
FROM golang:1.26-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/ommr-server ./cmd/server

# Final Runtime Stage
FROM alpine:3.21 AS runtime

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/ommr-server /app/ommr-server
COPY --from=builder /app/docs /app/docs
COPY --from=builder /app/ARCHITECTURE_DECISIONS.md /app/ARCHITECTURE_DECISIONS.md

EXPOSE 8080

ENTRYPOINT ["/app/ommr-server"]

# Optional: docker build --target with-python -t ommr-python .
FROM runtime AS with-python
RUN apk add --no-cache python3 py3-pip && python3 -m venv /opt/ytmusic
COPY workers/ytmusic/requirements.txt /app/workers/ytmusic/requirements.txt
RUN /opt/ytmusic/bin/pip install --no-cache-dir -r /app/workers/ytmusic/requirements.txt
COPY workers/ytmusic/worker.py /app/workers/ytmusic/worker.py
ENV OMMR_EXTRACTOR_PYTHON=/opt/ytmusic/bin/python

# The default target keeps the small Go-only runtime.
FROM runtime AS final
