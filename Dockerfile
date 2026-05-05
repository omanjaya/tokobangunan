# syntax=docker/dockerfile:1.7
# Multi-stage production build for tokobangunan.

# ---------- Stage 1: builder ----------
FROM golang:1.24-alpine AS builder

ENV GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux

RUN apk add --no-cache git curl ca-certificates tzdata

WORKDIR /app

# Tailwind CSS standalone CLI (no Node.js required)
RUN curl -sLo /usr/local/bin/tailwindcss \
        https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    && chmod +x /usr/local/bin/tailwindcss

# Cache Go modules first for better layer reuse
COPY go.mod go.sum ./
RUN go mod download

# Install templ generator (must match go.mod version of a-h/templ when possible)
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy the rest of the source
COPY . .

# Generate templ files
RUN templ generate

# Build CSS (input -> compiled output under web/static/css/)
RUN if [ -f web/static/css/input.css ]; then \
        tailwindcss -i web/static/css/input.css -o web/static/css/output.css --minify; \
    fi

# Compile static, stripped binary
RUN go build -trimpath -ldflags '-s -w' -o /server ./cmd/server

# ---------- Stage 2: runtime ----------
FROM alpine:3.20

ENV TZ=Asia/Jakarta \
    APP_PORT=8080

RUN apk add --no-cache ca-certificates tzdata wget \
    && cp /usr/share/zoneinfo/Asia/Jakarta /etc/localtime \
    && echo "Asia/Jakarta" > /etc/timezone \
    && addgroup -S -g 65532 app \
    && adduser  -S -u 65532 -G app app

WORKDIR /

COPY --from=builder /server /server
COPY --from=builder /app/web/static /web/static
COPY --from=builder /app/db/migrations /db/migrations

USER 65532:65532

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q -O- http://127.0.0.1:8080/livez >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/server"]
