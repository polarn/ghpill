DESTDIR ?= $(HOME)/.local/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)

.PHONY: build test clean install

build:
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o ./build/ghpill ./cmd/ghpill

test:
	go vet ./...
	go test ./...

install: build
	@mkdir -p $(DESTDIR)
	rm -f $(DESTDIR)/ghpill
	cp ./build/ghpill $(DESTDIR)/
	pkill -x ghpill 2>/dev/null || true

clean:
	rm -rf ./build ./dist
