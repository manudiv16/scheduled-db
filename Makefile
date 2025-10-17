# Makefile for scheduled-db project
.PHONY: help build test clean docker-build docker-push k8s-deploy k8s-delete dev-up dev-down logs

# Variables
APP_NAME := scheduled-db
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_REGISTRY := docker.io
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)
DOCKER_TAG := $(VERSION)
KUBECTL := kubectl
NAMESPACE := default

# Go variables
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
CGO_ENABLED := 0

# Build flags
LDFLAGS := -w -s -extldflags "-static"
BUILD_FLAGS := -a -installsuffix cgo -ldflags="$(LDFLAGS)"

## help: Show this help message
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build & Test:"
	@echo "  build              Build the binary"
	@echo "  build-all          Build binaries for multiple platforms"
	@echo "  test               Run tests with coverage"
	@echo "  test-short         Run short tests"
	@echo "  bench              Run benchmarks"
	@echo "  lint               Run linter"
	@echo "  fmt                Format code"
	@echo "  clean              Clean build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-push        Push Docker image to registry"
	@echo "  docker-run         Run Docker container locally"
	@echo "  dev-up             Start development environment (docker-compose)"
	@echo "  dev-down           Stop development environment"
	@echo "  dev-logs           Show development environment logs"
	@echo ""
	@echo "Kubernetes:"
	@echo "  k8s-deploy         Deploy to Kubernetes"
	@echo "  k8s-delete         Delete from Kubernetes"
	@echo "  k8s-status         Check Kubernetes deployment status"
	@echo "  k8s-logs           Show Kubernetes pod logs"
	@echo "  k8s-shell          Get shell in a pod"
	@echo "  k8s-port-forward   Port forward to access services locally"
	@echo ""
	@echo "Local Cluster:"
	@echo "  cluster-start      Start local cluster using scripts"
	@echo "  cluster-stop       Stop local cluster"
	@echo "  cluster-test       Test cluster functionality"
	@echo ""
	@echo "Testing & Jobs:"
	@echo "  create-jobs        Create test jobs (auto-detects environment)"
	@echo "  create-jobs-local  Create jobs on local cluster"
	@echo "  create-jobs-k8s    Create jobs on Kubernetes cluster"
	@echo "  test-proxy-local   Test proxy functionality locally"
	@echo "  test-proxy-k8s     Test proxy functionality on K8s"
	@echo "  cluster-info       Show cluster information"
	@echo ""
	@echo "Utilities:"
	@echo "  install-deps       Install development dependencies"
	@echo "  security-scan      Run security scans"
	@echo "  release            Create a new release"
	@echo "  version            Show version information"
	@echo "  mod                Tidy and download dependencies"

## build: Build the binary
build:
	@echo "Building $(APP_NAME) v$(VERSION) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o $(APP_NAME) cmd/scheduled-db/main.go
	@echo "Binary built: ./$(APP_NAME)"

## build-all: Build binaries for multiple platforms
build-all:
	@echo "Building $(APP_NAME) for multiple platforms..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/$(APP_NAME)-linux-amd64 cmd/scheduled-db/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/$(APP_NAME)-linux-arm64 cmd/scheduled-db/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/$(APP_NAME)-darwin-amd64 cmd/scheduled-db/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/$(APP_NAME)-darwin-arm64 cmd/scheduled-db/main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/$(APP_NAME)-windows-amd64.exe cmd/scheduled-db/main.go
	@echo "Built binaries in dist/ directory"

## test: Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## test-short: Run short tests
test-short:
	@echo "Running short tests..."
	go test -v -short ./...

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

## mod: Tidy and download dependencies
mod:
	@echo "Tidying modules..."
	go mod tidy
	go mod download

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(APP_NAME)
	rm -rf dist/
	rm -f coverage.out
	rm -rf data*/
	rm -rf logs*/
	rm -f *.log
	rm -f *.pid

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

## docker-push: Push Docker image to registry
docker-push: docker-build
	@echo "Pushing Docker image to registry..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## docker-run: Run Docker container locally
docker-run: docker-build
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 -p 7000:7000 $(DOCKER_IMAGE):$(DOCKER_TAG)

## dev-up: Start development environment with docker-compose
dev-up:
	@echo "Starting development environment..."
	docker-compose up -d
	@echo "Services started. Access points:"
	@echo "  - API: http://localhost:80"
	@echo "  - Node 1: http://localhost:8080"
	@echo "  - Node 2: http://localhost:8081"
	@echo "  - Node 3: http://localhost:8082"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Grafana: http://localhost:3000 (admin/admin)"

## dev-down: Stop development environment
dev-down:
	@echo "Stopping development environment..."
	docker-compose down -v
	@echo "Development environment stopped"

## dev-logs: Show logs from development environment
dev-logs:
	docker-compose logs -f

## k8s-deploy: Deploy to Kubernetes
k8s-deploy:
	@echo "Deploying to Kubernetes namespace: $(NAMESPACE)..."
	$(KUBECTL) apply -k k8s/ -n $(NAMESPACE)
	@echo "Deployment complete. Checking status..."
	$(KUBECTL) get pods -l app=$(APP_NAME) -n $(NAMESPACE)

## k8s-delete: Delete from Kubernetes
k8s-delete:
	@echo "Deleting from Kubernetes namespace: $(NAMESPACE)..."
	$(KUBECTL) delete -k k8s/ -n $(NAMESPACE) --ignore-not-found=true
	@echo "Resources deleted"

## k8s-status: Check Kubernetes deployment status
k8s-status:
	@echo "Checking Kubernetes deployment status..."
	$(KUBECTL) get all -l app=$(APP_NAME) -n $(NAMESPACE)
	@echo ""
	@echo "Pod logs (last 50 lines):"
	$(KUBECTL) logs -l app=$(APP_NAME) -n $(NAMESPACE) --tail=50

## k8s-logs: Show Kubernetes pod logs
k8s-logs:
	$(KUBECTL) logs -f -l app=$(APP_NAME) -n $(NAMESPACE)

## k8s-shell: Get shell in a pod
k8s-shell:
	$(KUBECTL) exec -it $$($(KUBECTL) get pods -l app=$(APP_NAME) -n $(NAMESPACE) -o jsonpath='{.items[0].metadata.name}') -n $(NAMESPACE) -- /bin/sh

## k8s-port-forward: Port forward to access services locally
k8s-port-forward:
	@echo "Port forwarding Kubernetes service..."
	@echo "API will be available at http://localhost:8080"
	$(KUBECTL) port-forward svc/$(APP_NAME)-api 8080:8080 -n $(NAMESPACE)

## cluster-start: Start local cluster using scripts
cluster-start:
	@echo "Starting local cluster..."
	./start-traditional-cluster-fixed.sh

## cluster-stop: Stop local cluster
cluster-stop:
	@echo "Stopping local cluster..."
	pkill -f $(APP_NAME) || true
	./stop-traditional-cluster.sh 2>/dev/null || true

## cluster-test: Test cluster functionality
cluster-test: build
	@echo "Testing cluster functionality..."
	./test-proxy.sh || echo "Proxy test script not found"
	./test-failover.sh || echo "Failover test script not found"

## create-jobs: Create test jobs using script
create-jobs:
	@echo "Creating test jobs..."
	@if kubectl get pods -l app=$(APP_NAME) -n $(NAMESPACE) >/dev/null 2>&1; then \
		echo "Detected Kubernetes environment"; \
		pod_name=$$(kubectl get pods -l app=$(APP_NAME) -n $(NAMESPACE) -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
		if [ -n "$$pod_name" ]; then \
			echo "Using pod: $$pod_name"; \
			kubectl port-forward "$$pod_name" 8080:8080 -n $(NAMESPACE) & \
			sleep 3; \
			./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:8080; \
			pkill -f "kubectl port-forward" || true; \
		else \
			echo "No scheduled-db pods found in Kubernetes"; \
			exit 1; \
		fi; \
	elif curl -s --connect-timeout 2 http://127.0.0.1:12080/health >/dev/null 2>&1; then \
		echo "Detected local cluster on port 12080"; \
		./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:12080; \
	elif curl -s --connect-timeout 2 http://127.0.0.1:80/health >/dev/null 2>&1; then \
		echo "Detected Docker cluster via nginx load balancer on port 80"; \
		./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:80; \
	elif curl -s --connect-timeout 2 http://127.0.0.1:8080/health >/dev/null 2>&1 || \
	     curl -s --connect-timeout 2 http://127.0.0.1:8081/health >/dev/null 2>&1 || \
	     curl -s --connect-timeout 2 http://127.0.0.1:8082/health >/dev/null 2>&1; then \
		echo "Detected Docker cluster, finding leader..."; \
		for port in 8080 8081 8082; do \
			if curl -s --connect-timeout 2 http://127.0.0.1:$$port/health >/dev/null 2>&1; then \
				role=$$(curl -s http://127.0.0.1:$$port/health | grep -o '"role":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "unknown"); \
				is_leader=$$(curl -s http://127.0.0.1:$$port/health | grep -o '"is_leader":[^,}]*' | cut -d':' -f2 2>/dev/null || echo "false"); \
				if [ "$$role" = "leader" ] || [ "$$is_leader" = "true" ]; then \
					echo "Found leader on port $$port"; \
					./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:$$port; \
					exit 0; \
				fi; \
			fi; \
		done; \
		echo "No leader found, trying first available node..."; \
		for port in 8080 8081 8082; do \
			if curl -s --connect-timeout 2 http://127.0.0.1:$$port/health >/dev/null 2>&1; then \
				echo "Using node on port $$port"; \
				./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:$$port; \
				exit 0; \
			fi; \
		done; \
		echo "No Docker nodes responding"; \
		exit 1; \
	else \
		echo "No scheduled-db cluster found. Try:"; \
		echo "  make cluster-start    # For local cluster"; \
		echo "  make k8s-deploy      # For Kubernetes"; \
		echo "  make dev-up          # For Docker environment"; \
		exit 1; \
	fi

## create-jobs-local: Create jobs on local cluster
create-jobs-local:
	@echo "Creating jobs on local cluster..."
	@if curl -s --connect-timeout 2 http://127.0.0.1:12080/health >/dev/null 2>&1; then \
		./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:12080; \
	elif curl -s --connect-timeout 2 http://127.0.0.1:8080/health >/dev/null 2>&1; then \
		./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:8080; \
	else \
		echo "Local cluster not found. Start with: make cluster-start"; \
		exit 1; \
	fi

## create-jobs-k8s: Create jobs on Kubernetes cluster
create-jobs-k8s:
	@echo "Creating jobs on Kubernetes cluster..."
	@pod_name=$$(kubectl get pods -l app=$(APP_NAME) -n $(NAMESPACE) -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	if [ -n "$$pod_name" ]; then \
		echo "Port forwarding to pod: $$pod_name"; \
		kubectl port-forward "$$pod_name" 8080:8080 -n $(NAMESPACE) & \
		sleep 5; \
		./create-test-jobs.sh -u 5 -r 3 -v -s http://127.0.0.1:8080; \
		pkill -f "kubectl port-forward" || true; \
	else \
		echo "No scheduled-db pods found. Deploy with: make k8s-deploy"; \
		exit 1; \
	fi

## test-proxy-local: Test proxy functionality on local cluster
test-proxy-local:
	@echo "Testing proxy on local cluster..."
	@if [ -f "./test-proxy-debug.sh" ]; then \
		./test-proxy-debug.sh; \
	else \
		echo "Creating quick proxy test..."; \
		if curl -s --connect-timeout 2 http://127.0.0.1:12081/health | grep -q follower; then \
			echo "Testing proxy via follower (port 12081)..."; \
			curl -X POST http://127.0.0.1:12081/jobs -H "Content-Type: application/json" \
				-d '{"type":"unico", "timestamp":"'$$(date -u -v+10S +%Y-%m-%dT%H:%M:%SZ)'"}'; \
		else \
			echo "No follower found on port 12081"; \
		fi; \
	fi

## test-proxy-k8s: Test proxy functionality on Kubernetes
test-proxy-k8s:
	@echo "Testing proxy on Kubernetes..."
	@pod_name=$$(kubectl get pods -l app=$(APP_NAME) -n $(NAMESPACE) -o jsonpath='{.items[1].metadata.name}' 2>/dev/null); \
	if [ -n "$$pod_name" ]; then \
		echo "Testing proxy via second pod: $$pod_name"; \
		kubectl port-forward "$$pod_name" 8081:8080 -n $(NAMESPACE) & \
		sleep 3; \
		curl -X POST http://127.0.0.1:8081/jobs -H "Content-Type: application/json" \
			-d '{"type":"unico", "timestamp":"'$$(date -u -v+10S +%Y-%m-%dT%H:%M:%SZ)'"}'; \
		pkill -f "kubectl port-forward.*8081" || true; \
	else \
		echo "Need at least 2 pods for proxy testing"; \
		exit 1; \
	fi

## cluster-info: Show cluster information
cluster-info:
	@echo "=== Cluster Information ==="
	@if kubectl get pods -l app=$(APP_NAME) -n $(NAMESPACE) >/dev/null 2>&1; then \
		echo "Kubernetes cluster:"; \
		kubectl get pods -l app=$(APP_NAME) -n $(NAMESPACE); \
		echo ""; \
		echo "Services:"; \
		kubectl get svc -l app=$(APP_NAME) -n $(NAMESPACE); \
	elif curl -s --connect-timeout 2 http://127.0.0.1:12080/debug/cluster >/dev/null 2>&1; then \
		echo "Local cluster (port 12080):"; \
		curl -s http://127.0.0.1:12080/debug/cluster | jq . || curl -s http://127.0.0.1:12080/debug/cluster; \
	elif curl -s --connect-timeout 2 http://127.0.0.1:8080/debug/cluster >/dev/null 2>&1; then \
		echo "Local cluster (port 8080):"; \
		curl -s http://127.0.0.1:8080/debug/cluster | jq . || curl -s http://127.0.0.1:8080/debug/cluster; \
	else \
		echo "No cluster found"; \
	fi

## install-deps: Install development dependencies
install-deps:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

## security-scan: Run security scans
security-scan:
	@echo "Running security scans..."
	gosec ./...
	go list -json -m all | nancy sleuth

## release: Create a new release
release: clean test lint build-all
	@echo "Creating release $(VERSION)..."
	@echo "Built binaries:"
	@ls -la dist/
	@echo "Ready for release!"

## version: Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Go version: $(shell go version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"
	@echo "Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Default target
.DEFAULT_GOAL := help
