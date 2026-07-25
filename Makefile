PACKAGE_NAME ?= sb-lxc
BIN_NAME ?= sb_lxc
VERSION ?= $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' main.go 2>nul)
ifeq ($(strip $(VERSION)),)
VERSION := 1.0.0
endif
TAG := v$(VERSION)

GOOS ?= linux
GOARCH ?= $(shell go env GOARCH)
DIST_DIR ?= dist
LDFLAGS ?= -s -w

ifeq ($(GOARCH),amd64)
DEB_ARCH ?= amd64
else ifeq ($(GOARCH),arm64)
DEB_ARCH ?= arm64
else
DEB_ARCH ?= $(GOARCH)
endif

DEB_FILE := $(DIST_DIR)/$(PACKAGE_NAME)_$(VERSION)_$(DEB_ARCH).deb

export CGO_ENABLED := 0
export GOOS := $(GOOS)
export GOARCH := $(GOARCH)
export VERSION := $(VERSION)
export DEB_ARCH := $(DEB_ARCH)

.DEFAULT_GOAL := help

.PHONY: help build test vet check deb release clean

help:
	@echo Targets:
	@echo   make build     Build $(BIN_NAME) into $(DIST_DIR)/
	@echo   make check     Run go test and go vet
	@echo   make deb       Build .deb package with nFPM into $(DIST_DIR)/
	@echo   make release   Build deb and publish/upload release via gh
	@echo   make clean     Remove build artifacts under $(DIST_DIR)/

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o '$(DIST_DIR)/$(BIN_NAME)' .
	@echo Built $(DIST_DIR)/$(BIN_NAME)

test:
	go test ./...

vet:
	go vet ./...

check: test vet

deb: check build
	nfpm package --packager deb --target '$(DIST_DIR)/'
	@echo ✔ Built deb package in $(DIST_DIR)/

release: deb
	@gh release view $(TAG) >nul 2>&1 && \
		gh release upload $(TAG) $(DIST_DIR)/$(BIN_NAME) $(DEB_FILE) --clobber || \
		gh release create $(TAG) $(DIST_DIR)/$(BIN_NAME) $(DEB_FILE) --title $(TAG) --generate-notes
	@echo ✔ Released $(TAG) to GitHub

clean:
	rm -rf '$(DIST_DIR)'
