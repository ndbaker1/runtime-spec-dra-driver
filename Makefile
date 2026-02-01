CONTAINER_TOOL ?= docker
TR       ?= tr

export IMAGE_GIT_TAG ?= $(shell git describe --tags --always --dirty --match 'v*')
export CHART_GIT_TAG ?= $(shell git describe --tags --always --dirty --match 'chart/*')

include $(CURDIR)/common.mk
include $(CURDIR)/e2e.mk

CMDS := $(patsubst ./cmd/%/,%,$(sort $(dir $(wildcard ./cmd/*/))))
CMD_TARGETS := $(patsubst %,cmd-%, $(CMDS))

MAKE_TARGETS := binaries build check vendor fmt test examples cmds coverage generate $(CHECK_TARGETS)

TARGETS := $(MAKE_TARGETS) $(CMD_TARGETS)

DOCKER_TARGETS := $(patsubst %,docker-%, $(TARGETS))
.PHONY: $(TARGETS) $(DOCKER_TARGETS)

GOOS ?= linux

binaries: cmds
ifneq ($(PREFIX),)
cmd-%: COMMAND_BUILD_OPTIONS = -o $(PREFIX)/$(*)
endif
cmds: $(CMD_TARGETS)
$(CMD_TARGETS): cmd-%:
	CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' GOOS=$(GOOS) \
		go build -ldflags "-s -w -X main.version=$(VERSION)" $(COMMAND_BUILD_OPTIONS) ./cmd/$(*)

build:
	GOOS=$(GOOS) go build ./...

examples: $(EXAMPLE_TARGETS)
$(EXAMPLE_TARGETS): example-%:
	GOOS=$(GOOS) go build ./examples/$(*)

all: check test build binary
check: $(CHECK_TARGETS)

vendor:
	go mod vendor

fmt:
	go list -f '{{.Dir}}' ./... | xargs gofmt -s -l -w

vet:
	go vet ./...

COVERAGE_FILE := coverage.out
test: build cmds
	go test -v -coverprofile=$(COVERAGE_FILE) ./...

coverage: test
	cat $(COVERAGE_FILE) | grep -v "_mock.go" > $(COVERAGE_FILE).no-mocks
	go tool cover -func=$(COVERAGE_FILE).no-mocks

generate: generate-deepcopy

generate-deepcopy: vendor
	for api in $(APIS); do \
		rm -f $(CURDIR)/api/$${api}/zz_generated.deepcopy.go; \
		controller-gen object \
			paths=$(CURDIR)/api/$${api}/ \
			output:object:dir=$(CURDIR)/api/$${api}; \
	done
