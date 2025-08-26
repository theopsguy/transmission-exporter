GO ?= GO111MODULE=on CGO_ENABLED=0 go
PACKAGES = $(shell go list ./... | grep -v /vendor/)
VERSION ?= $(shell git describe --tags --abbrev=0)
REVISION ?= $(shell git rev-parse --short HEAD)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILD_USER ?= $(shell whoami)@$(shell hostname)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%d-%H:%M:%S")

.PHONY: all
all: install

.PHONY: clean
clean:
	$(GO) clean -i ./...

.PHONY: install
install:
	$(GO) install -v ./cmd/transmission-exporter

.PHONY: build
build:
	$(GO) build -v -ldflags "\
        -X main.Version=${VERSION} \
        -X main.Revision=${REVISION} \
        -X main.Branch=${BRANCH} \
        -X main.BuildUser=${BUILD_USER} \
        -X main.BuildDate=${BUILD_DATE}" \
        -o transmission-exporter ./cmd/transmission-exporter

.PHONY: fmt
fmt:
	$(GO) fmt $(PACKAGES)

.PHONY: vet
vet:
	$(GO) vet $(PACKAGES)

.PHONY: lint
lint:
	@which golint > /dev/null; if [ $$? -ne 0 ]; then \
		$(GO) get -u golang.org/x/lint/golint; \
	fi
	for PKG in $(PACKAGES); do golint -set_exit_status $$PKG || exit 1; done;

.PHONY: dashboards
dashboards:
	jsonnet fmt -i dashboards/transmission.jsonnet
	jsonnet -J dashboards/vendor -m dashboards -e "(import 'dashboards/transmission.jsonnet').grafanaDashboards"
