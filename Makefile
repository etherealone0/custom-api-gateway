.PHONY: all build test test-integration bench docker-build docker-up docker-down chaos-demo clean

BINARY_NAME=gateway
DOCKER_IMAGE=custom-api-gateway:latest

all: build test

build:
	@echo "Building gateway binary..."
	@mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cmd/gateway

test:
	@echo "Running unit tests..."
	go test -short -v ./...

test-integration:
	@echo "Running integration tests (requires docker-compose)..."
	go test -v ./tests/...

bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

docker-up:
	@echo "Starting Docker Compose environment..."
	docker-compose up --build -d

docker-down:
	@echo "Tearing down Docker Compose environment..."
	docker-compose down -v

chaos-demo:
	@echo "Running chaos/failure-injection demo (requires docker-compose and k6)..."
	bash scripts/chaos-demo.sh

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	go clean
