# Stage 1: Build binary using multi-stage zero-cgo Go build
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source files
COPY . .

# Compile binary statically
ARG VERSION=1.0.0
ARG GIT_COMMIT=HEAD
ARG BUILD_DATE=2026-09-23

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
      -X github.com/DocHoax/watchdog/cmd.Version=${VERSION} \
      -X github.com/DocHoax/watchdog/cmd.GitCommit=${GIT_COMMIT} \
      -X github.com/DocHoax/watchdog/cmd.BuildDate=${BUILD_DATE} \
      -X github.com/DocHoax/watchdog/cmd.BuiltBy=docker" \
    -o /bin/watchdog .

# Stage 2: Distroless/Minimal Scratch Runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 watchdog && \
    adduser -u 1000 -G watchdog -s /bin/sh -D watchdog && \
    mkdir -p /home/watchdog/.watchdog && \
    chown -R watchdog:watchdog /home/watchdog

COPY --from=builder /bin/watchdog /usr/local/bin/watchdog

USER watchdog
WORKDIR /home/watchdog

EXPOSE 9100 8443

ENTRYPOINT ["/usr/local/bin/watchdog"]
CMD ["serve", "--prometheus", "--port", "9100"]
