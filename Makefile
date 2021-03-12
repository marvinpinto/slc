all: help

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

build:  ## Build the slc binary
	@echo "Build for Linux x86_64"
	@GOOS=linux GOARCH=amd64 go build -o ./dist/slc_linux_amd64 -v -ldflags="-X 'main.Version=${APP_VERSION}' -s -w" .

.PHONY: lint
lint:  ## Rudimentary source code linting
	@test -z $$(gofmt -l .)
	@go vet ./...

.PHONY: lintfix
lintfix:  ## Automatic fixes for basic formatting
	@go fmt ./...

.PHONY: clean
clean:  ## Clean out the dev/build environment
	@go clean -cache -modcache -i -r
	@rm -rf dist

.PHONY: update-deps
update-deps:  ## Update all the package dependencies
	@go get -u ./...
	@go mod tidy

.PHONY: test
test:  ## Execute all the unit tests
	@go test -v -race -cover -covermode=atomic -coverprofile coverage.cov ./lib

.PHONY: test-coverage
test-coverage: test  ## Execute all the unit tests (with coverage)
	@go tool cover -func coverage.cov
