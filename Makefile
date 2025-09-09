.PHONY: all build clean dashboards fmt install lint vet

PROJECTNAME ?= transmission-exporter

GO ?= GO111MODULE=on CGO_ENABLED=0 go
GOHOSTOS ?= $(shell $(GO) env GOHOSTOS)
GOHOSTARCH ?= $(shell $(GO) env GOHOSTARCH)
GO_BUILD_PLATFORM ?= $(GOHOSTOS)-$(GOHOSTARCH)
FIRST_GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PREFIX ?= $(shell pwd)
BIN_DIR ?= $(shell pwd)

PACKAGES = $(shell go list ./... | grep -v /vendor/)

PROMU_VERSION ?= 0.17.0
PROMU_URL := https://github.com/prometheus/promu/releases/download/v$(PROMU_VERSION)/promu-$(PROMU_VERSION).$(GO_BUILD_PLATFORM).tar.gz
PROMU := $(FIRST_GOPATH)/bin/promu

GOLANGCI_LINT_OPTS ?=
GOLANGCI_LINT_VERSION ?= v2.4.0
GOLANGCI_LINT_URL := https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCI_LINT_VERSION)/install.sh
GOLANGCI_LINT := $(FIRST_GOPATH)/bin/golangci-lint

all: install

install:
	$(GO) install -v ./cmd/transmission-exporter

lint: golangci-lint
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT) run --timeout=5m

build: promu
	$(PROMU) build --prefix $(PREFIX)

promu-crossbuild: promu
	$(PROMU) crossbuild

promu-crossbuild-tarballs: promu-crossbuild
	$(PROMU) crossbuild tarballs

release: promu-crossbuild-tarballs
	$(PROMU) release .tarballs

clean:
	@echo "Cleaning build artifacts..."
	$(GO) clean -i ./...
	rm -rf .build/
	rm -rf .tarballs/
	rm -f $(PROJECTNAME)
	rm -f $(PROJECTNAME)-*.tar.gz

promu:
	@if [ ! -f $(PROMU) ]; then \
		echo "Downloading promu..."; \
		PROMU_TMP=$$(mktemp -d); \
		if curl -fsSL $(PROMU_URL) | tar -xz -C "$$PROMU_TMP"; then \
			mkdir -p "$(FIRST_GOPATH)/bin"; \
			cp "$$PROMU_TMP/promu-$(PROMU_VERSION).$(GO_BUILD_PLATFORM)/promu" "$(FIRST_GOPATH)/bin/promu"; \
			chmod +x "$(PROMU)"; \
			rm -r "$$PROMU_TMP"; \
			echo "promu downloaded to $(FIRST_GOPATH)/bin/promu"; \
		else \
			echo "Failed to download promu"; \
			rm -r "$$PROMU_TMP"; \
			exit 1; \
		fi; \
	fi

golangci-lint:
	@if [ ! -f $(GOLANGCI_LINT) ]; then \
		echo "Downloading golangci-lint..."; \
		curl -sfL $(GOLANGCI_LINT_URL) \
		| sed -e '/install -d/d' \
		| sh -s -- -b $(FIRST_GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
		if [ ! -f $(GOLANGCI_LINT) ]; then \
			echo "Failed to download golangci-lint"; \
			exit 1; \
		else \
			echo "golangci-lint downloaded to $(GOLANGCI_LINT)"; \
		fi; \
	fi

dashboards:
	jsonnet fmt -i dashboards/transmission.jsonnet
	jsonnet -J dashboards/vendor -m dashboards -e "(import 'dashboards/transmission.jsonnet').grafanaDashboards"
