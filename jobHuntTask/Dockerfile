# syntax=docker/dockerfile:1.7

# ---- CRM UI (static export embedded at /crm) --------------------------------
FROM node:22-alpine AS crm

WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci 2>/dev/null || npm install
COPY frontend/ .
ENV NEXT_PUBLIC_API_URL=
RUN npm run build

# ---- Go build ---------------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
COPY --from=crm /frontend/out ./internal/web/static/crm

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime ----------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/server /app/server

EXPOSE 8082
USER nonroot:nonroot

ENTRYPOINT ["/app/server"]
