SHELL := /bin/bash

SCRIPT := ./scripts/build.sh

.PHONY: bootstrap build install enable-service enable-ui-gateway-service test test-droplet test-droplet-e2e test-condenser-e2e test-raind-e2e e2e all

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

test:
	@go test ./...

test-droplet:
	@./scripts/test/droplet.sh

test-droplet-e2e:
	@workshop run raind-dev -- test-droplet-e2e

test-condenser-e2e:
	@workshop run raind-dev -- test-condenser-e2e

test-raind-e2e:
	@workshop run raind-dev -- test-raind-e2e

e2e:
	@workshop run raind-dev -- e2e

all:
	@$(SCRIPT) all
