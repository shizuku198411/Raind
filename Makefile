SHELL := /bin/bash

SCRIPT := ./scripts/build.sh
COMPONENTS := runtime_stack/raind-cli runtime_stack/condenser runtime_stack/droplet
COMPONENTS += runtime_stack/raind-ui-gateway

.PHONY: bootstrap build install enable-service enable-ui-gateway-service all

bootstrap:
	@set -e; \
	for c in $(COMPONENTS); do \
		echo "==> bootstrap $$c"; \
		(cd $$c && go mod download); \
	done

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
