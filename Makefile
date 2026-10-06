BINARY_NAME := eunomia
BUILD_DIR := build
BINARY := $(BUILD_DIR)/$(BINARY_NAME)
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: all build install test vet check clean

all: test vet build

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY_NAME)

test:
	go test ./...

vet:
	go vet ./...

check: test vet

clean:
	rm -rf $(BUILD_DIR)
