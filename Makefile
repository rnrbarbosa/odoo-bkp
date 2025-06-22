# Makefile at project root

APP_NAME := odoo-bkp
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
ifeq ($(GOOS),)
	GOOS := linux
endif
ifeq ($(GOARCH),)
	GOARCH := amd64
endif
GO_DIR := .
BUILD_DIR := build

.PHONY: all build test fmt lint clean run

all: build

VERSION := $(shell git describe --tags --always 2> /dev/null || git rev-parse --short HEAD)

build:
	mkdir -p build
	echo "VERSION=$(VERSION)"
	cd $(GO_DIR) && go build -ldflags "-X 'main.version=$(VERSION)'" -o ../../$(BUILD_DIR)/$(APP_NAME)

test:
	cd $(GO_DIR) && go test ./...

fmt:
	cd $(GO_DIR) && go fmt ./...

lint:
	cd $(GO_DIR) && golint ./...

clean:
	rm -rf $(BUILD_DIR)

run:
	cd $(GO_DIR)/cmd/go && go run main.go
