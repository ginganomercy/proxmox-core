# Build Stage
FROM golang:1.23-bookworm AS build

# base debian bookworm image includes build-essential and gcc which is perfect for CGO
WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o core-api ./cmd/api

# Production Stage
FROM debian:bookworm-slim

# Install tzdata for timezones and ca-certificates for HTTPS
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from the build stage
COPY --from=build /app/core-api .

# Expose the API port
EXPOSE 3001

CMD ["./core-api"]
