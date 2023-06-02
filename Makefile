BIN := ./bin/mii-lama.bin

LAST_COMMIT := $(shell git rev-parse --short HEAD)
LAST_COMMIT_DATE := $(shell git show -s --format=%ci ${LAST_COMMIT})
VERSION := $(shell git describe --tags)
BUILDSTR := ${VERSION} (Commit: ${LAST_COMMIT_DATE} (${LAST_COMMIT}), Build: $(shell date +"%Y-%m-%d% %H:%M:%S %z"))

.PHONY: build
build: ## Build the binary.
	CGO_ENABLED=0 go build -o ${BIN} -ldflags="-X 'main.buildString=${BUILDSTR}'" ./cmd/

.PHONY: run
run: build ## Build and Runs the binary.
	${BIN} --config config.toml

.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out
	go tool cover -func=coverage.out
	rm coverage.out

.PHONY: test
test:
	@go test -v

.PHONY: lint
lint: ## Run all the linters.
	golangci-lint run --config golangci-lint.yml ./...

.PHONY: dist
dist: build
	mkdir -p dist
	cp -R ${BIN} ./dist/
	cp -R ./config.sample.toml ./dist/config.toml

