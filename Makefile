APP_NAME := gopersist
BIN_DIR := bin

.PHONY: all build run test fmt lint clean

all: build

build:
	@echo "> Building $(APP_NAME)"
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)

run:
	@go run ./cmd/$(APP_NAME)

test:
	@go test ./...

fmt:
	@go fmt ./...
	@go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed; skipping lint"

clean:
	@rm -rf $(BIN_DIR)

build-linux-amd64:
	@echo "> Building $(APP_NAME) for linux/amd64"
	@mkdir -p $(BIN_DIR)
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 ./cmd/$(APP_NAME)

build-linux-arm64:
	@echo "> Building $(APP_NAME) for linux/arm64"
	@mkdir -p $(BIN_DIR)
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP_NAME)-linux-arm64 ./cmd/$(APP_NAME)
