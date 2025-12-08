BINARY_NAME=huh
INSTALL_PATH=/usr/local/bin

.PHONY: all build install clean run

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/huh

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	sudo install -m 755 $(BINARY_NAME) $(INSTALL_PATH)

clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)

run:
	go run ./cmd/huh
