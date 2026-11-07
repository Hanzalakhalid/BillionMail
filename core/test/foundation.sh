#!/bin/sh
set -eu
mkdir -p /src
mkdir -p /src/public/dist /src/public/html
# Test a fixed snapshot, even while the developer continues editing the host.
cp /workspace/go.mod /workspace/go.sum /workspace/main.go /src/
for directory in api cmd internal utility manifest languages resource template; do
    cp -a "/workspace/$directory" "/src/$directory"
done
cd /src
if [ -s /test-ca/ca-bundle.pem ]; then
    export SSL_CERT_FILE=/test-ca/ca-bundle.pem
fi
echo "Running backend tests"
if [ "${BM_TEST_FULL:-0}" = 1 ]; then
    go test -short -count=1 -timeout=180s ./...
else
    go test -count=1 -timeout=180s ./internal/migrations ./internal/service/projects ./internal/service/products ./internal/service/business ./internal/service/scheduler ./internal/service/queue ./internal/service/integrations ./internal/service/objectstorage ./internal/service/antivirus ./internal/service/assets ./internal/service/secrets ./internal/service/tenancy ./internal/service/platform ./internal/service/identity ./internal/service/notifications ./internal/service/whatsappchannel ./internal/service/whatsappwebhook
fi
echo "Compiling API"
go build -o /tmp/billionmail ./main.go
echo "Compiling marketing worker"
go build -o /tmp/saas-worker ./cmd/saas-worker
echo "Compiling scheduler"
go build -o /tmp/scheduler ./cmd/scheduler
echo "Compiling media worker"
go build -o /tmp/media-worker ./cmd/media-worker
