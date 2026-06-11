#!/bin/bash

# execute test
go test ./... -coverprofile=coverage/coverage.out

# generate html file
go tool cover -html=coverage/coverage.out -o coverage/coverage.html

# open web server
python3 -m http.server 8777 -d coverage
