# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /bin/isthmus ./cmd/isthmus

FROM gcr.io/distroless/base-debian12
COPY --from=builder /bin/isthmus /usr/local/bin/isthmus
USER nonroot:nonroot
ENTRYPOINT ["isthmus"]
