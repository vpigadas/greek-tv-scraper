.PHONY: all clean build test dev

# Build
all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o greek-tv-scraper ./cmd/server

# Development mode
dev:
	go run ./cmd/server

# Tests
test:
	go test ./...

# Lint
vet:
	go vet ./...

clean:
	rm -f greek-tv-scraper server
