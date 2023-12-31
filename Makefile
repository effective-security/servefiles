include .project/gomod-project.mk
export GO111MODULE=on
BUILD_FLAGS=

.PHONY: *

.SILENT:

default: help

all: clean tools generate covtest

#
# clean produced files
#
clean:
	go clean ./...
	rm -rf \
		${COVPATH} \
		${PROJ_BIN}

tools:
	go install github.com/go-phorce/cov-report/cmd/cov-report@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54
	go install github.com/mattn/goveralls@latest

build:
	echo "nothing to build yet"

coveralls-github:
	echo "Running coveralls"
	goveralls -v -coverprofile=coverage.out -service=github -package ./...
