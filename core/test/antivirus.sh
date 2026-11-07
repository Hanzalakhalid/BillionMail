#!/bin/sh
set -eu
mkdir -p /src/internal/service
cp /workspace/go.mod /workspace/go.sum /src/
cp -a /workspace/internal/service/antivirus /src/internal/service/
cd /src
go test -count=1 -timeout=120s -run TestLiveClamd ./internal/service/antivirus
