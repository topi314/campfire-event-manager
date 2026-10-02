# syntax=docker/dockerfile:1

# Static frontend is arch-independent — build once on the runner, not under QEMU.
FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS frontend
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY frontend/ ./
ENV NUXT_PUBLIC_API_BASE=
RUN npm run generate

# Cross-compile Go from the native runner for each TARGETARCH.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
COPY --from=frontend /app/.output/public ./frontend/dist

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    GOARM=${TARGETVARIANT#v} \
    go build -o campfire-event-manager .

FROM alpine

COPY --from=build /build/campfire-event-manager /bin/campfire-event-manager

ENTRYPOINT ["/bin/campfire-event-manager"]

CMD ["-config", "/var/lib/config.toml"]
