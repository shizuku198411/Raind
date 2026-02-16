#!/bin/bash

BINDIR=./bin
MAINDIR=./cmd/raind
BINNAME=raind

go build -o $BINDIR/$BINNAME $MAINDIR