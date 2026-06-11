SHELL := /bin/bash

SCRIPT := ./scripts/build.sh

.PHONY: bootstrap build install enable-service enable-ui-gateway-service all

bootstrap:
	@go mod download

build:
	@$(SCRIPT) build

install:
	@$(SCRIPT) install

enable-service:
	@$(SCRIPT) enable-service

enable-ui-gateway-service:
	@$(SCRIPT) enable-ui-gateway-service

all:
	@$(SCRIPT) all
