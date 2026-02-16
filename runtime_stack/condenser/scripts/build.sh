#!/bin/bash

BINDIR=./bin
MAINDIR=./cmd/condenser
BINNAME=condenser

swag init -g cmd/condenser/main.go

# condenser
go build -o $BINDIR/$BINNAME $MAINDIR

HOOKMAINDIR=./cmd/condenser-hook
HOOKBINNAME=condenser-hook-agent

# hook
go build -o $BINDIR/$HOOKBINNAME $HOOKMAINDIR
