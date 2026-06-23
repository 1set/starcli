MAKEFLAGS := --print-directory
SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c

BINARY=starcli

# for CircleCI, GitHub Actions, GitLab CI build number
ifeq ($(origin CIRCLE_BUILD_NUM), environment)
	BUILD_NUM ?= cc$(CIRCLE_BUILD_NUM)
else ifeq ($(origin GITHUB_RUN_NUMBER), environment)
	BUILD_NUM ?= gh$(GITHUB_RUN_NUMBER)
else ifeq ($(origin CI_PIPELINE_IID), environment)
	BUILD_NUM ?= gl$(CI_PIPELINE_IID)
endif

# for go dev
GOCMD=go
GORUN=$(GOCMD) run
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GODOC=$(GOCMD) doc
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# for go build
export CGO_ENABLED=0
export TZ=Asia/Shanghai
export PACK=github.com/1set/starcli/config
export FLAGS="-s -w -X '$(PACK).AppName=$(BINARY)' -X '$(PACK).BuildDate=`date '+%Y-%m-%dT%T%z'`' -X '$(PACK).BuildHost=`hostname`' -X '$(PACK).GoVersion=`go env GOVERSION`' -X '$(PACK).GitBranch=`git symbolic-ref -q --short HEAD`' -X '$(PACK).GitCommit=`git rev-parse --short HEAD`' -X '$(PACK).GitSummary=$${GIT_TAG_NAME:-`git describe --tags --dirty --always`}' -X '$(PACK).CIBuildNum=${BUILD_NUM}'"

# commands
.PHONY: default build build_linux build_mac build_windows snapshot run install ci test bench
default:
	@echo "build target is required for $(BINARY)"
	@exit 1

build:
	$(GOBUILD) -v -ldflags $(FLAGS) -trimpath -o $(BINARY) .

build_linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -v -ldflags $(FLAGS) -trimpath -o $(BINARY) .

build_mac:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -v -ldflags $(FLAGS) -trimpath -o $(BINARY) .

build_windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) -v -ldflags $(FLAGS) -trimpath -o $(BINARY).exe .

# Release-style cross-platform build via GoReleaser (snapshot = local dry-run,
# nothing is published). Inspect the artifacts under dist/. `make build` stays
# the fast single-platform dev build; this is for checking what a release ships.
snapshot:
	GOVERSION=`go env GOVERSION` $(GORUN) github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean

run: build
	./$(BINARY)

preview:
	STAR_HOST_NAME=Aloha ./$(BINARY) --version --log debug

# CI bar consumed by 1set/meta go-ci.yml: race + coverage profile (covcheck reads
# coverage.txt) then bench compile. CGO is enabled for the race detector even
# though release builds set CGO_ENABLED=0 above.
ci:
	CGO_ENABLED=1 $(GOTEST) -v -race -cover -covermode=atomic -coverprofile=coverage.txt -count 1 ./...
	$(GOTEST) -v -parallel=4 -run="none" -benchtime="2s" -benchmem -bench=. ./...

test:
	CGO_ENABLED=1 $(GOTEST) -v -race -cover -covermode=atomic -coverprofile=coverage.txt -count 1 ./...
	$(GOTEST) -parallel=4 -run="none" -benchtime="2s" -benchmem -bench=.

bench:
	$(GOTEST) -parallel=4 -run="none" -benchtime="2s" -benchmem -bench=. ./...

install:
ifndef GOBIN
	$(error GOBIN is not set)
endif
	@if [ ! -d "$(GOBIN)" ]; then echo "Directory $(GOBIN) does not exist"; exit 1; fi
	cp $(BINARY) $(GOBIN)

artifact:
	test -n "$(OSEXT)"
	mkdir -p _upload
	cp -f $(BINARY) _upload/$(BINARY).$(OSEXT)
