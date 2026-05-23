# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Download Go modules (caching step)
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build a statically linked binary. 
# CGO_ENABLED=0 ensures it runs on a distroless static image.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /api-gateway ./cmd/gateway

# Stage 2: Create a minimal production image
FROM gcr.io/distroless/static:nonroot

# Set the working directory
WORKDIR /

# Copy the binary from the builder stage
COPY --from=builder /api-gateway /api-gateway

# Copy the configuration file
COPY config.yaml /config.yaml

# Run as non-root user for security (provided by distroless)
USER 65532:65532

# Expose the default gateway port
EXPOSE 8080

# Command to run the executable
CMD ["/api-gateway"]
