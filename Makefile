APPS := store-mcp

BIN_DIR := bin
BUILD_FLAGS := -trimpath

.PHONY: $(APPS) build test vet run clean

build: $(APPS)

$(APPS):
	go build $(BUILD_FLAGS) -o $(BIN_DIR)/$@ ./cmd/$@

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/store-mcp

clean:
	rm -rf $(BIN_DIR)