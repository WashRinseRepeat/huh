BINARY_NAME=huh
PREFIX?=/usr/local
INSTALL_PATH=$(PREFIX)/bin

VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: all build install uninstall clean run test vet

all: build

build:
	@echo "Building $(BINARY_NAME) ($(VERSION))..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/huh

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	install -d $(INSTALL_PATH)
	install -m 755 $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

uninstall:
	@echo "Removing $(INSTALL_PATH)/$(BINARY_NAME)..."
	rm -f $(INSTALL_PATH)/$(BINARY_NAME)

clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)

run:
	go run ./cmd/huh

test:
	go test -race ./...

vet:
	go vet ./...
