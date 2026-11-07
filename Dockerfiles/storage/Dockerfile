# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS go-base

FROM go-base AS minio-builder
# Immutable final upstream Community Edition commit; no third-party binaries.
ARG MINIO_COMMIT=7aac2a2c5b7c882e68c1ce017d8256be2feea27f
RUN --mount=type=secret,id=host_ca --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    if [ -s /run/secrets/host_ca ]; then export SSL_CERT_FILE=/run/secrets/host_ca; fi; \
    GOMAXPROCS=4 CGO_ENABLED=0 go install -p 4 github.com/minio/minio@${MINIO_COMMIT} && \
    mkdir -p /out && cp /go/bin/minio /out/minio && \
    module_dir="$(go list -m -f '{{.Dir}}' github.com/minio/minio@${MINIO_COMMIT})" && \
    cp "$module_dir/LICENSE" /out/LICENSE

FROM go-base AS init-builder
WORKDIR /src
COPY core/go.mod core/go.sum ./
COPY core/internal/service/objectstorage/ ./internal/service/objectstorage/
COPY core/cmd/storage-init/ ./cmd/storage-init/
RUN --mount=type=secret,id=host_ca --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    if [ -s /run/secrets/host_ca ]; then export SSL_CERT_FILE=/run/secrets/host_ca; fi; \
    CGO_ENABLED=0 go build -trimpath -o /out/storage-init ./cmd/storage-init

FROM alpine:3.23 AS runtime
COPY --from=go-base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

FROM runtime AS init
COPY --from=init-builder /out/storage-init /usr/local/bin/storage-init
ENTRYPOINT ["/usr/local/bin/storage-init"]

FROM runtime AS server
LABEL org.opencontainers.image.source="https://github.com/minio/minio" \
      org.opencontainers.image.revision="7aac2a2c5b7c882e68c1ce017d8256be2feea27f" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY --from=minio-builder /out/minio /usr/local/bin/minio
COPY --from=minio-builder /out/LICENSE /usr/share/licenses/minio/LICENSE
EXPOSE 9000 9001
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/minio"]
CMD ["server","/data","--console-address",":9001"]
