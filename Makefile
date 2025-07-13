# Portask Makefile
# High-performance queue manager

# Variables
BINARY_NAME=portask
BINARY_PATH=./cmd/portask
BUILD_DIR=./build
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -s -w"

# Go settings
export CGO_ENABLED=1
export GOOS=$(shell go env GOOS)
export GOARCH=$(shell go env GOARCH)

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
.PHONY: help
help:
	@echo "🚀 Portask Build System"
	@echo "======================"
	@echo ""
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
	@echo ""

## deps: Install dependencies
.PHONY: deps
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

## build: Build the binary
.PHONY: build
build: deps
	@echo "🔨 Building Portask..."
	@mkdir -p ${BUILD_DIR}
	go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ${BINARY_PATH}
	@echo "✅ Build complete: ${BUILD_DIR}/${BINARY_NAME}"

## build-release: Build optimized release binary
.PHONY: build-release
build-release: deps
	@echo "🔨 Building Portask (release)..."
	@mkdir -p ${BUILD_DIR}
	go build ${LDFLAGS} -tags release -o ${BUILD_DIR}/${BINARY_NAME} ${BINARY_PATH}
	@echo "✅ Release build complete: ${BUILD_DIR}/${BINARY_NAME}"

## build-all: Build for all platforms
.PHONY: build-all
build-all: deps
	@echo "🔨 Building Portask for all platforms..."
	@mkdir -p ${BUILD_DIR}
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-linux-amd64 ${BINARY_PATH}
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-linux-arm64 ${BINARY_PATH}
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-amd64 ${BINARY_PATH}
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-arm64 ${BINARY_PATH}
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-windows-amd64.exe ${BINARY_PATH}
	
	@echo "✅ Multi-platform build complete"
	@ls -la ${BUILD_DIR}/

## test: Run all tests
.PHONY: test
test:
	@echo "🧪 Running tests..."
	go test -v -race -timeout=30s ./...
	@echo "✅ Tests completed"

## test-coverage: Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	@mkdir -p ${BUILD_DIR}
	go test -v -race -timeout=30s -coverprofile=${BUILD_DIR}/coverage.out ./...
	go tool cover -html=${BUILD_DIR}/coverage.out -o ${BUILD_DIR}/coverage.html
	@echo "✅ Coverage report: ${BUILD_DIR}/coverage.html"

## benchmark: Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "⚡ Running benchmarks..."
	@mkdir -p ${BUILD_DIR}
	go test -v -bench=. -benchmem -timeout=10m ./... | tee ${BUILD_DIR}/benchmark.txt
	@echo "✅ Benchmark results: ${BUILD_DIR}/benchmark.txt"

## profile: Run CPU and memory profiling
.PHONY: profile
profile:
	@echo "📊 Running profiling..."
	@mkdir -p ${BUILD_DIR}/profiles
	go test -v -bench=. -benchmem -cpuprofile=${BUILD_DIR}/profiles/cpu.prof -memprofile=${BUILD_DIR}/profiles/mem.prof ./pkg/...
	@echo "✅ Profiles generated in ${BUILD_DIR}/profiles/"
	@echo "   View CPU profile: go tool pprof ${BUILD_DIR}/profiles/cpu.prof"
	@echo "   View memory profile: go tool pprof ${BUILD_DIR}/profiles/mem.prof"

## lint: Run linting
.PHONY: lint
lint:
	@echo "🔍 Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run --timeout=5m ./...
	@echo "✅ Linting completed"

## format: Format code
.PHONY: format
format:
	@echo "🎨 Formatting code..."
	go fmt ./...
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w ${GO_FILES}
	@echo "✅ Code formatted"

## clean: Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf ${BUILD_DIR}
	go clean -cache -testcache -modcache
	@echo "✅ Clean completed"

## run: Run the application
.PHONY: run
run: build
	@echo "🚀 Running Portask..."
	${BUILD_DIR}/${BINARY_NAME} --config=configs/config.dev.yaml

## run-prod: Run with production config
.PHONY: run-prod
run-prod: build-release
	@echo "🚀 Running Portask (production)..."
	${BUILD_DIR}/${BINARY_NAME} --config=configs/config.prod.yaml

## demo: Run the core components demo
.PHONY: demo
demo: deps
	@echo "🎭 Running core components demo..."
	go run examples/core_demo.go

## docker-build: Build Docker image
.PHONY: docker-build
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t portask:latest -f Dockerfile .
	docker build -t portask:${VERSION} -f Dockerfile .
	@echo "✅ Docker image built: portask:latest, portask:${VERSION}"

## docker-run: Run Docker container
.PHONY: docker-run
docker-run:
	@echo "🐳 Running Docker container..."
	docker run --rm -p 8080:8080 -p 9092:9092 -p 5672:5672 -p 9090:9090 portask:latest

## install: Install the binary
.PHONY: install
install: build-release
	@echo "📦 Installing Portask..."
	sudo cp ${BUILD_DIR}/${BINARY_NAME} /usr/local/bin/
	@echo "✅ Portask installed to /usr/local/bin/${BINARY_NAME}"

## dev: Start development environment
.PHONY: dev
dev:
	@echo "🔧 Starting development environment..."
	@which modd > /dev/null || (echo "Installing modd..." && go install github.com/cortesi/modd/cmd/modd@latest)
	modd

## generate: Generate code (mocks, etc.)
.PHONY: generate
generate:
	@echo "⚙️ Generating code..."
	go generate ./...
	@echo "✅ Code generation completed"

## security: Run security checks
.PHONY: security
security:
	@echo "🔒 Running security checks..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec -quiet ./...
	@echo "✅ Security checks completed"

## mod-update: Update all dependencies
.PHONY: mod-update
mod-update:
	@echo "📦 Updating dependencies..."
	go get -u ./...
	go mod tidy
	@echo "✅ Dependencies updated"

## size: Show binary size analysis
.PHONY: size
size: build-release
	@echo "📏 Binary size analysis..."
	@ls -lh ${BUILD_DIR}/${BINARY_NAME}
	@echo ""
	@echo "📊 Size breakdown:"
	@which go-size-analyzer > /dev/null || (echo "Installing go-size-analyzer..." && go install github.com/Zxilly/go-size-analyzer@latest)
	go-size-analyzer ${BUILD_DIR}/${BINARY_NAME}

## performance: Run full performance test suite
.PHONY: performance
performance: benchmark profile
	@echo "⚡ Running performance analysis..."
	@mkdir -p ${BUILD_DIR}/performance
	
	# Memory usage test
	@echo "📊 Memory usage test..."
	go test -v -timeout=5m -memprofile=${BUILD_DIR}/performance/heap.prof ./pkg/... -bench=BenchmarkMessage
	
	# CPU usage test  
	@echo "🔥 CPU usage test..."
	go test -v -timeout=5m -cpuprofile=${BUILD_DIR}/performance/cpu.prof ./pkg/... -bench=BenchmarkSerialization
	
	# Generate performance report
	@echo "📋 Generating performance report..."
	@echo "Performance Test Results - $(shell date)" > ${BUILD_DIR}/performance/report.txt
	@echo "============================================" >> ${BUILD_DIR}/performance/report.txt
	@echo "" >> ${BUILD_DIR}/performance/report.txt
	@cat ${BUILD_DIR}/benchmark.txt >> ${BUILD_DIR}/performance/report.txt
	
	@echo "✅ Performance analysis completed"
	@echo "   Report: ${BUILD_DIR}/performance/report.txt"
	@echo "   CPU profile: go tool pprof ${BUILD_DIR}/performance/cpu.prof"
	@echo "   Memory profile: go tool pprof ${BUILD_DIR}/performance/heap.prof"

## docs: Generate documentation
.PHONY: docs
docs:
	@echo "📚 Generating documentation..."
	@mkdir -p ${BUILD_DIR}/docs
	@which godoc > /dev/null || (echo "Installing godoc..." && go install golang.org/x/tools/cmd/godoc@latest)
	
	# Generate package documentation
	go doc -all ./pkg/... > ${BUILD_DIR}/docs/api.txt
	
	# Generate README from templates if available
	@if [ -f "docs/README.template.md" ]; then \
		echo "📝 Generating README from template..."; \
		cp docs/README.template.md README.md; \
	fi
	
	@echo "✅ Documentation generated in ${BUILD_DIR}/docs/"

## all: Run full CI pipeline
.PHONY: all
all: clean deps format lint security test test-coverage build-release
	@echo "🎉 Full CI pipeline completed successfully!"

# Development workflow targets
## quick: Quick development build and test
.PHONY: quick
quick: format test build
	@echo "⚡ Quick development cycle completed"

## check: Pre-commit checks
.PHONY: check
check: format lint test security
	@echo "✅ Pre-commit checks passed"

# Information targets
## version: Show version information
.PHONY: version
version:
	@echo "Version: ${VERSION}"
	@echo "Build Time: ${BUILD_TIME}"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: ${GOOS}/${GOARCH}"

## info: Show project information
.PHONY: info
info:
	@echo "🚀 Portask - High-Performance Queue Manager"
	@echo "=========================================="
	@echo "Version: ${VERSION}"
	@echo "Build Time: ${BUILD_TIME}"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: ${GOOS}/${GOARCH}"
	@echo ""
	@echo "📁 Project Structure:"
	@tree -L 2 -I 'vendor|build|.git' .
	@echo ""
	@echo "📊 Code Statistics:"
	@echo "  Go files: $(shell find . -name "*.go" -type f -not -path "./vendor/*" | wc -l)"
	@echo "  Lines of code: $(shell find . -name "*.go" -type f -not -path "./vendor/*" -exec cat {} \; | wc -l)"
	@echo "  Test files: $(shell find . -name "*_test.go" -type f | wc -l)"
