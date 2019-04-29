DEPS := $(shell go list -f '{{$$dir := .Dir}}{{range .GoFiles }}{{$$dir}}/{{.}} {{end}}' ./...)
BUILD = $(shell git rev-parse --short HEAD 2>/dev/null)
VERSION = $(shell git describe --tags)
LDFLAGS := "-X main.BUILD=$(BUILD) -X main.VERSION=$(VERSION)"
RepoTag = $(shell git describe --abbrev=0 --tags 2>/dev/null || (echo '0.0.0'))

GOMOD_RO_FLAG ?=

build/linux/pentagon: $(DEPS)
	mkdir -p build/linux
	GOOS=linux go build $(GOMOD_RO_FLAG) -v -ldflags=$(LDFLAGS) -o ./build/linux/pentagon ./pentagon

build/darwin/pentagon: $(DEPS)
	mkdir -p build/darwin
	GOOS=darwin go build $(GOMOD_RO_FLAG) -v -ldflags=$(LDFLAGS) -o ./build/darwin/pentagon ./pentagon

.PHONY: go_mod_cache
go_mod_cache: go.mod go.sum
	GO111MODULE=on go mod download

vendor/gomod_deps: go.mod go.sum
	mkdir -p vendor/gomod_deps
	readonly GOPATH=$$(mktemp -d); GO111MODULE=on go mod download && \
			   chmod -R 'u+wr' "vendor/gomod_deps" && \
			   rm -rf vendor/gomod_deps/mod; \
			   mkdir vendor/gomod_deps/mod && \
			   cp -r $${GOPATH}/pkg/mod/cache vendor/gomod_deps/mod/cache; \
			   chmod -R 'u+wr' "vendor/gomod_deps/mod" "$${GOPATH}"; \
			   rm -rf "$${GOPATH}"

.PHONY: docker
docker: Dockerfile $(DEPS) vendor/gomod_deps
	docker build . -t pentagon:${RepoTag}

.PHONY: test
test:
	@go test -v ./...

.PHONY: clean
clean:
	@-rm -rf ./build ./vendor
