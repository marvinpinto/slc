all: help

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

PLATFORMS := linux/amd64 windows/amd64 darwin/amd64
temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))

$(PLATFORMS):
	@GOOS=$(os) GOARCH=$(arch) go build -o './dist/slc_$(os)_$(arch)' -v -ldflags="-X 'github.com/marvinpinto/slc/cmd.Version=${APP_VERSION}' -s -w" .
	@mv -f ./dist/slc_windows_amd64 ./dist/slc_windows_amd64.exe > /dev/null 2>&1 || true

.PHONY: build-all $(PLATFORMS)
build-all: $(PLATFORMS)  ## Build the slc binaries

.PHONY: build
build: ## Build the slc binary
	@GOOS=linux GOARCH=amd64 go build -o './dist/slc' -v -ldflags="-X 'github.com/marvinpinto/slc/cmd.Version=${APP_VERSION}' -s -w" .

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
	@go test -race -cover -covermode=atomic -coverprofile coverage.cov ./lib

.PHONY: test-coverage
test-coverage: test  ## Execute all the unit tests (with coverage)
	@go tool cover -func coverage.cov

.PHONY: release
release: lint test build-all ## Generate a new GitHub tagged release
ifndef tag
	@echo 'tag not specified - try: make tag=0.0.1 release'
	@exit 1
endif
	@echo "Creating a new release for: $(tag)"
	@git tag "$(tag)"
	@git push origin --tags
