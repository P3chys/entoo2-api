# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Copy source code first to get all dependencies
COPY . .

# Download dependencies and tidy
RUN go mod download && go mod tidy

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -u 1001 appuser

WORKDIR /home/appuser

# Copy binary from builder
COPY --from=builder /app/main .

# Copy email templates
COPY --from=builder /app/templates ./templates

# Entrypoint: fix ownership of the volume mount point (runs as root), then exec app as appuser
COPY --from=builder /bin/sh /bin/sh
RUN printf '#!/bin/sh\nmkdir -p "${STORAGE_PATH:-/home/appuser/uploads}"\nchown appuser:appuser "${STORAGE_PATH:-/home/appuser/uploads}"\nexec su-exec appuser ./main\n' > /entrypoint.sh && \
    chmod +x /entrypoint.sh

RUN apk --no-cache add su-exec && chown appuser:appuser /home/appuser

EXPOSE 8000

CMD ["/entrypoint.sh"]
