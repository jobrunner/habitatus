# syntax=docker/dockerfile:1

# Stage 1: static build. CGO_ENABLED=0 so the binary has no dynamic
# dependency on libc — required for it to run on the distroless base below,
# which carries no shared libraries at all.
FROM golang:1.24 AS build
WORKDIR /src

ARG VERSION=dev
ARG COMMIT=unknown

COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/habitatus ./cmd/habitatus

# Stage 2: distroless, non-root, no shell, no package manager.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/habitatus /habitatus

# The vendored ESy rule file (CC BY 4.0 — attribution required, see
# ATTRIBUTION.md) and its licence/attribution notice, so the image is
# self-contained: `docker run -p 8080:8080 habitatus` works with no volume
# mount.
COPY data/esy/EUNIS-ESy-2025-10-03.txt /data/esy/EUNIS-ESy-2025-10-03.txt
COPY data/esy/ATTRIBUTION.md /data/esy/ATTRIBUTION.md

# The 48 backbone (nomenclature-translation) tables are not vendored yet
# (see data/esy/ATTRIBUTION.md and data/esy/backbones/README.md) — 43 MB,
# added later. Once they are dropped under data/esy/backbones/, add:
#   COPY data/esy/backbones /data/esy/backbones
# -backbones already accepts a directory path; nothing else needs to change.

# habitatus does not write anywhere at runtime — the rule file and backbone
# tables are read once at startup and everything after that is in-memory —
# so the image runs cleanly under --read-only with no tmpfs mount needed.

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/habitatus"]
CMD ["-addr", ":8080", "-rules", "/data/esy/EUNIS-ESy-2025-10-03.txt"]
