# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY core/go.mod core/go.sum ./
RUN --mount=type=secret,id=host_ca --mount=type=cache,target=/go/pkg/mod \
    if [ -s /run/secrets/host_ca ]; then export SSL_CERT_FILE=/run/secrets/host_ca; fi; go mod download
COPY core/ ./
RUN --mount=type=secret,id=host_ca --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    if [ -s /run/secrets/host_ca ]; then export SSL_CERT_FILE=/run/secrets/host_ca; fi; \
    CGO_ENABLED=0 go build -trimpath -o /out/media-worker ./cmd/media-worker

FROM mcr.microsoft.com/playwright:v1.64.1-noble
USER root
RUN --mount=type=secret,id=host_ca \
    if [ -s /run/secrets/host_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/host_ca; fi; \
    apt-get update && apt-get install -y --no-install-recommends ffmpeg imagemagick ca-certificates curl && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /opt/media-tools /opt/billionmail/core && \
    cd /opt/media-tools && npm install --omit=dev --ignore-scripts playwright@1.64.1 && \
    chown -R pwuser:pwuser /opt/media-tools /opt/billionmail
COPY --from=builder /out/media-worker /opt/billionmail/core/media-worker
COPY core/manifest/ /opt/billionmail/core/manifest/
USER pwuser
WORKDIR /opt/billionmail/core
ENV NODE_PATH=/opt/media-tools/node_modules
ENTRYPOINT ["/opt/billionmail/core/media-worker"]
