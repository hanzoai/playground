FROM --platform=$BUILDPLATFORM node:20-alpine AS ui-builder
RUN npm install -g pnpm@10
WORKDIR /app/web

COPY web/client/package.json web/client/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/client/ .

# IAM (Hanzo SSO) — baked into the static bundle at build time so
# AuthContext.isIamMode is true in production. Fall back to canonical
# values; CI may override via --build-arg.
ARG VITE_IAM_SERVER_URL=https://hanzo.id
ARG VITE_IAM_CLIENT_ID=hanzo-bot
ARG VITE_IAM_REDIRECT_URI=
ARG VITE_BASE_PATH=/
ENV VITE_IAM_SERVER_URL=${VITE_IAM_SERVER_URL} \
    VITE_IAM_CLIENT_ID=${VITE_IAM_CLIENT_ID} \
    VITE_IAM_REDIRECT_URI=${VITE_IAM_REDIRECT_URI} \
    VITE_BASE_PATH=${VITE_BASE_PATH}

ARG CACHE_BUST=1
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26.0-bookworm AS go-builder
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    gcc-aarch64-linux-gnu \
    libc6-dev-arm64-cross \
    gcc-x86-64-linux-gnu \
    libc6-dev-amd64-cross \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/web/dist ./web/client/dist

RUN if [ "${TARGETARCH}" = "arm64" ]; then export CC=aarch64-linux-gnu-gcc; \
    elif [ "${TARGETARCH}" = "amd64" ]; then export CC=x86_64-linux-gnu-gcc; fi && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags "embedded sqlite_fts5" -o /app/bin/playground-server ./cmd/playground-server

# Pre-create writable data directory for distroless runtime. The nonroot user
# has UID/GID 65532 in the base image, so match that here.
RUN mkdir -p /tmp/playground-data && chown 65532:65532 /tmp/playground-data

FROM gcr.io/distroless/base-debian12

LABEL org.opencontainers.image.source=https://github.com/hanzoai/playground
LABEL org.opencontainers.image.description="Hanzo Playground — open-source agentic harness"
LABEL org.opencontainers.image.licenses=MIT

COPY --from=go-builder /app/bin/playground-server /usr/local/bin/playground-server
COPY --from=go-builder /app/config /etc/playground/config
COPY --from=go-builder --chown=nonroot:nonroot /tmp/playground-data /data
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/playground-server"]
