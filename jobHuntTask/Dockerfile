# syntax=docker/dockerfile:1.7

# ---- build stage ------------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache go modules independently of source.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Static binary, no CGO — runs anywhere, including distroless/static.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime stage ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/server /app/server

EXPOSE 8082
USER nonroot:nonroot

ENTRYPOINT ["/app/server"]
