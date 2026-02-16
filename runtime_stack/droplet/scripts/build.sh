#!/bin/bash

BINDIR=./bin
MAINDIR=./cmd/droplet
BINNAME=droplet

go build -o $BINDIR/$BINNAME $MAINDIR