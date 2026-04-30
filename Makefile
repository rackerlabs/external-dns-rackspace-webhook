# Variables
BINARY_NAME=external-dns-rackspace-webhook
IMAGE_NAME=ghcr.io/rackerlabs/external-dns-rackspace-webhook
VERSION?=latest
PLATFORMS=linux/amd64,linux/arm64

# Go parameters
GOCMD=go
GOFLAGS?=-mod=mod
GOBUILD=$(GOCMD) build $(GOFLAGS)
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test $(GOFLAGS)
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

.PHONY: all build clean test e2e deps docker-build docker-push help

all: test build

# Build the binary
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo -o $(BINARY_NAME) ./cmd/webhook

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) verify

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Build Docker image
docker-build:
	docker build -t $(IMAGE_NAME):$(VERSION) .

# Build multi-platform Docker image
docker-buildx:
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE_NAME):$(VERSION) .

# Push Docker image
docker-push:
	docker push $(IMAGE_NAME):$(VERSION)

# Build and push multi-platform image
docker-release:
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE_NAME):$(VERSION) --push .

# Run locally for development
run:
	$(GOCMD) run ./cmd/webhook

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint code
lint:
	GOFLAGS=$(GOFLAGS) golangci-lint run

# Security scan
security:
	gosec ./...

# Install development dependencies
dev-deps:
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint
	$(GOGET) -u github.com/securecodewarrior/gosec/cmd/gosec

# Deploy to Kubernetes (assumes kubectl is configured)
deploy:
	kubectl apply -f manifests/

# Remove from Kubernetes
undeploy:
	kubectl delete -f manifests