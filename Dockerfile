# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.2

FROM golang:${GO_VERSION} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 \
	GOOS=${TARGETOS} \
	GOARCH=${TARGETARCH:-$(go env GOARCH)} \
	go build -trimpath -ldflags="-s -w" -o /out/revenueleakageengine ./cmd/revenueleakageengine

RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build --chown=nonroot:nonroot /out/revenueleakageengine /app/revenueleakageengine
COPY --from=build --chown=nonroot:nonroot /src/internal/platform/sqlite/migrations /app/internal/platform/sqlite/migrations
COPY --from=build --chown=nonroot:nonroot /src/internal/app/gateway/http/static /app/internal/app/gateway/http/static
COPY --from=build --chown=nonroot:nonroot /out/data /data

ENV RLE_HTTP_ADDR=:8080 \
	RLE_GRPC_ADDR=:9090 \
	RLE_SQLITE_PATH=/data/local.db \
	RLE_MIGRATIONS_PATH=/app/internal/platform/sqlite/migrations \
	RLE_LOG_JSON=true

EXPOSE 8080 9090
VOLUME ["/data"]

USER nonroot:nonroot

ENTRYPOINT ["/app/revenueleakageengine"]
