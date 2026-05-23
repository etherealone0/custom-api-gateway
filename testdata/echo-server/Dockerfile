# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /echo-server main.go

# Stage 2: Create a minimal production image
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /echo-server /echo-server

USER 65532:65532
EXPOSE 9000
CMD ["/echo-server"]
