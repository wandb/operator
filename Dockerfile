# Watchtower ships in this image as a second entrypoint: the operator synthesizes
# an Application that runs the same image with `command: ["/watchtower"]`, and
# finds this image by name through OPERATOR_IMAGE. Its binary embeds a Next.js
# static export, so it cannot be rebuilt from Go source here — download the
# published binary instead.
#
# Watchtower no longer publishes a container image; it attaches per-arch Linux
# binaries to its GitHub Release. wandb/watchtower is a PRIVATE repository, so the
# fetch below needs a token with read access, passed as a BuildKit secret:
#
#   GH_TOKEN=$(gh auth token) \
#     docker build --secret id=gh_token,env=GH_TOKEN .
#
# The tag carries its leading "v" — it is the release tag verbatim, not the de-v'ed
# form the old image tag used.
ARG WATCHTOWER_VERSION=v0.12.0-rc.1

# Build the manager binary
FROM golang:1.26 AS manager-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY pkg/ pkg/
COPY internal/ internal/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager ./cmd/manager
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o crd-installer ./cmd/crd-installer

# A private repo's assets are not reachable by their browser download URL — that
# 404s even with a token. The releases API has to resolve the asset id first, then
# serve the bytes with Accept: application/octet-stream.
FROM alpine:3.21 AS watchtower
ARG WATCHTOWER_VERSION
ARG TARGETARCH
RUN apk add --no-cache ca-certificates curl jq tar
RUN --mount=type=secret,id=gh_token \
    set -eu; \
    token="$(cat /run/secrets/gh_token)"; \
    api="https://api.github.com/repos/wandb/watchtower"; \
    asset="watchtower-linux-${TARGETARCH}.tar.gz"; \
    release="$(curl -fsSL -H "Authorization: Bearer ${token}" \
                 "${api}/releases/tags/${WATCHTOWER_VERSION}")"; \
    fetch() { \
      id="$(printf '%s' "${release}" | jq -r --arg n "$1" '.assets[] | select(.name == $n) | .id')"; \
      if [ -z "${id}" ] || [ "${id}" = "null" ]; then \
        echo "release ${WATCHTOWER_VERSION} has no asset named $1" >&2; \
        return 1; \
      fi; \
      curl -fsSL -H "Authorization: Bearer ${token}" -H "Accept: application/octet-stream" \
        -o "/tmp/$1" "${api}/releases/assets/${id}"; \
    }; \
    fetch "${asset}"; \
    fetch watchtower-linux-checksums.txt; \
    cd /tmp && grep "${asset}" watchtower-linux-checksums.txt | sha256sum -c -; \
    tar -xzf "/tmp/${asset}" -C /

FROM registry.access.redhat.com/ubi9/ubi-minimal

WORKDIR /
COPY --from=manager-builder /workspace/manager .
COPY --from=manager-builder /workspace/crd-installer .
# Released CGO-free and statically linked, so it runs unmodified on this glibc
# base despite having been unpacked in an Alpine stage.
COPY --from=watchtower /watchtower .

RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm && \
    chown -R 65532:65532 /helm

USER 65532:65532

ENV HELM_CACHE_HOME=/helm/.cache/helm
ENV HELM_CONFIG_HOME=/helm/.config/helm
ENV HELM_DATA_HOME=/helm/.local/share/helm
ENV OPERATOR_MODE=production

ENTRYPOINT ["/manager"]
