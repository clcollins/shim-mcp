FROM docker.io/library/golang:1.26 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-X github.com/clcollins/shim-mcp/internal/cli.Version=${VERSION} \
              -X github.com/clcollins/shim-mcp/internal/cli.Commit=${COMMIT}" \
    -o shim-mcp ./cmd/shim-mcp

FROM registry.fedoraproject.org/fedora-minimal:42

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="shim-mcp" \
      org.opencontainers.image.description="Lightweight MCP server for authenticated HTTP request proxying" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/clcollins/shim-mcp"

COPY --from=builder /build/shim-mcp /usr/local/bin/shim-mcp

USER 1000

ENTRYPOINT ["/usr/local/bin/shim-mcp"]
CMD ["serve"]
