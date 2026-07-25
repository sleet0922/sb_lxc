PACKAGE_NAME ?= sb-lxc
BIN_NAME ?= sb_lxc
VERSION ?= $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' main.go)
GOOS ?= linux
GOARCH ?= $(shell go env GOARCH)
DIST_DIR ?= dist
LDFLAGS ?= -s -w

ifeq ($(strip $(VERSION)),)
VERSION := 0.0.0
endif

ifeq ($(GOARCH),amd64)
DEB_ARCH ?= amd64
else ifeq ($(GOARCH),arm64)
DEB_ARCH ?= arm64
else
DEB_ARCH ?= $(GOARCH)
endif

BUILD_ENV := CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)

.DEFAULT_GOAL := help

.PHONY: help build test vet check deb clean

help:
	@printf 'Targets:\n'
	@printf '  make build     Build $(BIN_NAME) into $(DIST_DIR)/\n'
	@printf '  make check     Run go test and go vet\n'
	@printf '  make deb       Build .deb package with nFPM into $(DIST_DIR)/\n'
	@printf '  make clean     Remove build artifacts under $(DIST_DIR)/\n'

build:
	@mkdir -p '$(DIST_DIR)'
	$(BUILD_ENV) go build -trimpath -ldflags '$(LDFLAGS)' -o '$(DIST_DIR)/$(BIN_NAME)' .
	@printf 'Built %s\n' '$(DIST_DIR)/$(BIN_NAME)'

test:
	go test ./...

vet:
	go vet ./...

check: test vet

deb: check build
	@command -v nfpm >/dev/null 2>&1 || { echo '✘ nfpm is required. Run: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest'; exit 1; }
	VERSION=$(VERSION) DEB_ARCH=$(DEB_ARCH) nfpm package --packager deb --target '$(DIST_DIR)/'
	@printf '✔ Built deb package in %s/\n' '$(DIST_DIR)'

clean:
	rm -rf '$(DIST_DIR)'
