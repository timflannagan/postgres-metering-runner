SHELL := /bin/bash

VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
            echo v0)

ROOT_DIR := $(patsubst %/,%,$(dir $(realpath $(lastword $(MAKEFILE_LIST)))))
GO_BUILD_OPTS := --ldflags '-w'

vendor:
	go mod tidy
	go mod vendor
	go mod verify

driver:
	CGO_ENABLED=0 go build $(GO_BUILD_OPTS) -o $(ROOT_DIR)/bin/driver $(ROOT_DIR)/cmd/driver

build:
	docker build -f Dockerfile -t quay.io/tflannag/promscale:latest $(ROOT_DIR) && \
	docker push quay.io/tflannag/promscale:latest
