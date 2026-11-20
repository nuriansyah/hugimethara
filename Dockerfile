# syntax=docker/dockerfile:1.7
# One image, four binaries; the command selects the role.
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    LDFLAGS="-s -w -X github.com/nuriansyah/hugimethara/internal/platform/buildinfo.Version=${VERSION} \
             -X github.com/nuriansyah/hugimethara/internal/platform/buildinfo.Commit=${COMMIT}" && \
    go build -ldflags "$LDFLAGS" -o /out/hugi-api ./cmd/api && \
    go build -ldflags "$LDFLAGS" -o /out/hugi-worker ./cmd/worker && \
    go build -ldflags "$LDFLAGS" -o /out/hugi-migrate ./cmd/migrate && \
    go build -ldflags "$LDFLAGS" -o /out/hugi-admin ./cmd/admin

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT []
CMD ["hugi-api"]
