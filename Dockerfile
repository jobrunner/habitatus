# syntax=docker/dockerfile:1

# Stage 1: static build. CGO_ENABLED=0 so the binary has no dynamic
# dependency on libc — required for it to run on the distroless base below,
# which carries no shared libraries at all.
# Pinned to a patch level, not a floating minor tag. The 1.24 line is out of
# security support: govulncheck reports standard-library vulnerabilities
# against 1.24.13, its last release, that are fixed only from 1.25.13 on.
FROM golang:1.26.8 AS build
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

# The software's own licence travels with the binary: an image is a
# distribution, and MIT requires the notice to accompany it.
COPY LICENSE /LICENSE

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

# Defaults live in the environment, not in CMD. Docker replaces the whole
# CMD array as soon as a caller passes any argument, so a CMD of
# ["-addr", ":8080", "-rules", "/data/esy/..."] would silently vanish behind
# `docker run habitatus -mode faithful` -- the container would then exit 2
# on "missing -rules", the most obvious invocation being the one that
# breaks. cmd/habitatus reads HABITATUS_ADDR/HABITATUS_RULES/
# HABITATUS_BACKBONES/HABITATUS_MODE as defaults, with a flag of the same
# name overriding when given, so both `docker run habitatus -mode faithful`
# and `docker run -e HABITATUS_MODE=faithful habitatus` work. CMD stays
# empty; there is nothing left for it to carry.
# :8080, overriding the binary's loopback default: inside a container the
# port is reachable only through an explicit -p mapping, and a process bound
# to 127.0.0.1 there would answer nobody but itself.
ENV HABITATUS_ADDR=:8080
ENV HABITATUS_RULES=/data/esy/EUNIS-ESy-2025-10-03.txt

EXPOSE 8080
# Runs as uid 65532 (nonroot). A rule file or backbone directory mounted in
# at runtime -- to override HABITATUS_RULES/HABITATUS_BACKBONES -- must be
# readable by that uid, not just by the host user who owns it; e.g.
# `docker run -v $PWD/other.txt:/rules.txt:ro,z --read-only ...` needs the
# file world- or 65532-readable, or `--user "$(id -u):$(id -g)"` to match.
USER nonroot:nonroot
ENTRYPOINT ["/habitatus"]
