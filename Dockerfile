# Build Stage
FROM golang:alpine AS build

# Install gcc and musl-dev because go-sqlite3 requires CGO
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o core-api .

# Production Stage
FROM alpine:latest

# Install tzdata for timezones and ca-certificates for HTTPS
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# Copy the binary from the build stage
COPY --from=build /app/core-api .

# Expose the API port
EXPOSE 3001

CMD ["./core-api"]
