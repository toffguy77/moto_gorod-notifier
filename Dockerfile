# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/notifier ./cmd/notifier

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user and home directory
RUN adduser -D -s /bin/sh notifier

WORKDIR /home/notifier

# Copy the binary from builder stage
COPY --from=builder /app/bin/notifier ./notifier

# Set correct permissions
RUN chmod +x ./notifier && chown notifier:notifier ./notifier

USER notifier

# Expose port (if needed for health checks)
EXPOSE 8080

CMD ["./notifier"]