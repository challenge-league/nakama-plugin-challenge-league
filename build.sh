#!/bin/bash
set -euxo pipefail 
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=1
go build -buildmode=plugin -trimpath
#go build -buildmode=plugin -trimpath -mod=vendor
