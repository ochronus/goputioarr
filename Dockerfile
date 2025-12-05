# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates (needed for fetching dependencies)
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o /goputioarr ./cmd

# Final stage
FROM alpine:3.23

# Install ca-certificates for HTTPS requests and tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 goputioarr && \
    adduser -D -u 1000 -G goputioarr goputioarr

# Create directories
RUN mkdir -p /config /downloads && \
    chown -R goputioarr:goputioarr /config /downloads

# Copy binary from builder
COPY --from=builder /goputioarr /usr/local/bin/goputioarr

# Set environment variables
ENV PUID=1000
ENV PGID=1000
ENV TZ=Etc/UTC

# Expose port
EXPOSE 9091

# Volume for config and downloads
VOLUME ["/config", "/downloads"]

# Switch to non-root user
USER goputioarr

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/goputioarr"]

# Default command
CMD ["run", "-c", "/config/config.toml"]
