# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Install build dependencies for SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Build the application with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o bin/notifier ./cmd/notifier

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user and home directory
RUN adduser -D -s /bin/sh notifier

# Create data directory for SQLite with proper permissions
RUN mkdir -p /data && chown notifier:notifier /data && chmod 755 /data

WORKDIR /home/notifier

# Copy the binary from builder stage
COPY --from=builder /app/bin/notifier ./notifier

# Set correct permissions
RUN chmod +x ./notifier && chown notifier:notifier ./notifier

USER notifier

# Expose port for metrics
EXPOSE 9090

# Mount point for persistent data
VOLUME ["/data"]

CMD ["./notifier"]