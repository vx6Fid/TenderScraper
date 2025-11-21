# Stage 1 — build
FROM golang:1.25.4 AS builder

WORKDIR /app

# Copy only go.mod first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest (excluding TenderDocs via .dockerignore)
COPY . .

# Build the HTTP service
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./http

# Stage 2 — run
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/server .

# adjust if needed
EXPOSE 8080  

CMD ["./server"]

