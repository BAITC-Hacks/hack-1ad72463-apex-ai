export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/../backend/.cache/go-mod
export GOTMPDIR := $(CURDIR)/.tmp

.PHONY: prepare test check build init run
prepare:
	@mkdir -p .cache/go-build .tmp bin

test: prepare
	go test -race -cover ./...

check: prepare
	@test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	node --check internal/app/web/app.js

build: prepare
	go build -trimpath -o bin/admin ./cmd/admin

init: prepare
	go run ./cmd/admin init

run: prepare
	go run ./cmd/admin serve
