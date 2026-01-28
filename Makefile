SHELL := /bin/bash

.PHONY: bootstrap build install enable-service disable-service status clean

bootstrap:
	@./scripts/bootstrap.sh

build:
	@./scripts/build.sh

install:
	@sudo PREFIX=$${PREFIX:-/usr/local} ./scripts/install.sh

enable-service:
	@sudo install -m 0644 ./scripts/systemd/condenser.service /etc/systemd/system/condenser.service
	@sudo systemctl daemon-reload
	@sudo systemctl enable --now condenser.service

disable-service:
	@sudo systemctl disable --now condenser.service || true
	@sudo rm -f /etc/systemd/system/condenser.service
	@sudo systemctl daemon-reload

status:
	@systemctl status condenser.service --no-pager || true

clean:
	@rm -rf components/*/bin || true
