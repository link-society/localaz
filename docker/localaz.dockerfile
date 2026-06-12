# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/localaz ./cmd/localaz

# Pre-create the data directory so it can be copied with the correct ownership.
RUN mkdir -p /data

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /

COPY --from=build /out/localaz /usr/local/bin/localaz

# Persisted emulator state, owned by the nonroot user (uid 65532) so the
# emulator can write to it. A named volume mounted here inherits this ownership.
COPY --from=build --chown=65532:65532 /data /data

# Persisted emulator state. Mount a volume here to keep data across restarts.
VOLUME ["/data"]

# Azure Blob service. This matches Azurite's default blob port so existing
# tooling and connection strings work unchanged.
EXPOSE 10000

ENV LOCALAZ_BLOB_ADDR=":10000" \
    LOCALAZ_DATA_DIR="/data"

ENTRYPOINT ["/usr/local/bin/localaz"]
