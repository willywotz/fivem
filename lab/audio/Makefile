.PHONY: build run-server run-client docker-build docker-up docker-down clean

# Build targets
build: build-server build-client

build-server:
	go build -o bin/audio-server ./cmd/server

build-client:
	go build -o bin/audio-client ./cmd/client

# Run targets
run-server:
	go run ./cmd/server -port=8080

run-client:
	go run ./cmd/client -server=http://localhost:8080

# Docker targets
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Development targets
dev-server:
	go run ./cmd/server -port=8080

dev-client:
	go run ./cmd/client -server=http://localhost:8080

# Clean targets
clean:
	rm -rf bin/
	docker-compose down --rmi all --volumes

# Help
help:
	@echo "Available commands:"
	@echo "  build         - Build both server and client"
	@echo "  build-server  - Build server only"
	@echo "  build-client  - Build client only"
	@echo "  run-server    - Run server locally"
	@echo "  run-client    - Run client locally"
	@echo "  docker-build  - Build Docker images"
	@echo "  docker-up     - Start services with Docker Compose"
	@echo "  docker-down   - Stop Docker Compose services"
	@echo "  docker-logs   - View Docker Compose logs"
	@echo "  dev-server    - Run server in development mode"
	@echo "  dev-client    - Run client in development mode"
	@echo "  clean         - Clean build artifacts and Docker"
	@echo "  help          - Show this help message"
