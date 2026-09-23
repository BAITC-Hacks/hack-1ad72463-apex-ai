export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod
export GOTMPDIR := $(CURDIR)/.tmp

.PHONY: prepare deps fmt check test test-integration build run migrate import bootstrap catalog-check benchmark

prepare:
	@mkdir -p .cache/go-build .cache/go-mod .tmp bin

deps: prepare
	go mod download

fmt:
	gofmt -w cmd internal catalogadmin

check: prepare
	@test -z "$$(gofmt -l cmd internal catalogadmin)" || (gofmt -l cmd internal catalogadmin; exit 1)
	go vet ./...

test: prepare
	go test -race -cover ./...

test-integration: prepare
	@test -n "$(TEST_DATABASE_URL)" || (echo 'Set TEST_DATABASE_URL to a test PostgreSQL database'; exit 1)
	go test -race -count=1 -v ./internal/postgres

build: prepare
	go build -trimpath -o bin/server ./cmd/server
	go build -trimpath -o bin/catalog ./cmd/catalog

run: prepare
	go run ./cmd/server

migrate import bootstrap: prepare
	go run ./cmd/catalog $@

catalog-check: prepare
	go run ./cmd/catalog check

benchmark: prepare
	go test ./internal/matching -run '^$$' -bench BenchmarkRecommendation -benchmem
