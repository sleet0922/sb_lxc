PACKAGE_NAME ?= sb-lxc
BIN_NAME ?= sb_lxc
VERSION ?= $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' main.go)
GOOS ?= linux
GOARCH ?= $(shell go env GOARCH)
PREFIX ?= /usr
DIST_DIR ?= dist
PKG_DIR := $(DIST_DIR)/pkg
DEB_FILE = $(DIST_DIR)/$(PACKAGE_NAME)_$(VERSION)_$(DEB_ARCH).deb
CONTROL_FILE := $(PKG_DIR)/DEBIAN/control
STAGED_BIN := $(PKG_DIR)$(PREFIX)/bin/$(BIN_NAME)
BUILD_FLAGS ?= -trimpath
LDFLAGS ?= -s -w
MAINTAINER ?= root <root@localhost>
DESCRIPTION ?= Simple Incus container management CLI

ifeq ($(strip $(VERSION)),)
VERSION := 0.0.0
endif

ifeq ($(GOARCH),amd64)
DEB_ARCH ?= amd64
else ifeq ($(GOARCH),arm64)
DEB_ARCH ?= arm64
else ifeq ($(GOARCH),arm)
DEB_ARCH ?= armhf
else
DEB_ARCH ?= $(GOARCH)
endif

BUILD_ENV := CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)

.DEFAULT_GOAL := help

.PHONY: help build test vet check deb deb-fpm clean distclean

help:
	@printf 'Targets:\n'
	@printf '  make build     Build $(BIN_NAME) into $(DIST_DIR)/\n'
	@printf '  make check     Run go test and go vet\n'
	@printf '  make deb       Build $(DEB_FILE) with dpkg-deb\n'
	@printf '  make deb-fpm   Build a deb with fpm, if fpm is installed\n'
	@printf '  make clean     Remove staging files and standalone binary\n'
	@printf '  make distclean Remove all files under $(DIST_DIR)/\n'

build:
	@mkdir -p '$(DIST_DIR)'
	$(BUILD_ENV) go build $(BUILD_FLAGS) -ldflags '$(LDFLAGS)' -o '$(DIST_DIR)/$(BIN_NAME)' .
	@printf 'Built %s\n' '$(DIST_DIR)/$(BIN_NAME)'

test:
	go test ./...

vet:
	go vet ./...

check: test vet

deb: check
	@command -v dpkg-deb >/dev/null || { echo 'dpkg-deb is required'; exit 1; }
	@rm -rf '$(PKG_DIR)'
	@mkdir -p '$(PKG_DIR)/DEBIAN' '$(PKG_DIR)$(PREFIX)/bin'
	$(BUILD_ENV) go build $(BUILD_FLAGS) -ldflags '$(LDFLAGS)' -o '$(STAGED_BIN)' .
	@chmod 0755 '$(STAGED_BIN)'
	@{ \
		printf 'Package: %s\n' '$(PACKAGE_NAME)'; \
		printf 'Version: %s\n' '$(VERSION)'; \
		printf 'Section: utils\n'; \
		printf 'Priority: optional\n'; \
		printf 'Architecture: %s\n' '$(DEB_ARCH)'; \
		printf 'Depends: incus\n'; \
		printf 'Maintainer: %s\n' '$(MAINTAINER)'; \
		printf 'Description: %s\n' '$(DESCRIPTION)'; \
		printf ' %s is an interactive command-line helper for managing Incus containers.\n' '$(BIN_NAME)'; \
	} > '$(CONTROL_FILE)'
	@chmod 0644 '$(CONTROL_FILE)'
	dpkg-deb --root-owner-group --build '$(PKG_DIR)' '$(DEB_FILE)'
	@printf 'Built %s\n' '$(DEB_FILE)'

deb-fpm: check build
	@command -v fpm >/dev/null || { echo 'fpm is required'; exit 1; }
	fpm -s dir -t deb \
		-n '$(PACKAGE_NAME)' \
		-v '$(VERSION)' \
		-a '$(DEB_ARCH)' \
		--depends incus \
		--description '$(DESCRIPTION)' \
		-p '$(DEB_FILE)' \
		'$(DIST_DIR)/$(BIN_NAME)=$(PREFIX)/bin/$(BIN_NAME)'
	@printf 'Built %s\n' '$(DEB_FILE)'

clean:
	rm -rf '$(PKG_DIR)' '$(DIST_DIR)/$(BIN_NAME)'

distclean:
	rm -rf '$(DIST_DIR)'
