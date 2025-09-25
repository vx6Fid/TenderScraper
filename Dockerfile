# ---- Build Stage ----
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go.mod and go.sum first for caching dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy necessary source code
COPY http ./http
COPY utils ./utils
COPY docDownloads ./docDownloads
COPY scraper/captcha ./scraper/captcha
COPY session ./session

# Build the Go server
RUN go build -o tender-server ./http

# ---- Final Stage ----
FROM debian:bookworm-slim

WORKDIR /app

# Install CA certificates
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy the built binary from builder
COPY --from=builder /app/tender-server .

# Expose the port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./tender-server"]
