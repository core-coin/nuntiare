# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=nuntiare
BINARY_UNIX=$(BINARY_NAME)_unix

# Default target executed when no arguments are given to make
default: build

# Build the project
build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/nuntiare

# Run the project
run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/nuntiare
	./$(BINARY_NAME)

# Test the project
test:	
	$(GOTEST) -v ./...

# Clean the project
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Format the code
fmt:
	$(GOCMD) fmt ./...

# Lint the code
lint:
	golangci-lint run

# Run the project with environment variables
run-env:
	CONFIG=config.yaml POSTGRES_USER=user POSTGRES_PASSWORD=password POSTGRES_HOST=localhost POSTGRES_PORT=5432 POSTGRES_DB=db BLOCKCHAIN_SERVICE_URL=http://localhost:8545 SMART_CONTRACT_ADDRESS=0x1234567890abcdef DEVELOPMENT=true $(GOBUILD) -o $(BINARY_NAME) -v ./cmd/nuntiare
	./$(BINARY_NAME)

# Load environment variables from .env and run docker-compose
docker-run:
	test -f .env && docker-compose --env-file .env up -d || echo ".env file not found"

docker-down:
	docker-compose down

docker-build:
	docker-compose build
.PHONY: build run test clean fmt lint run-env docker-run