SHELL := /bin/bash

SCRIPT := ./scripts/build.sh
COMPONENTS := runtime_stack/raind-cli runtime_stack/condenser runtime_stack/droplet

.PHONY: bootstrap build install enable-service all

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

all:
	@$(SCRIPT) all
