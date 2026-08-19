# Watchtower ships in this image as a second entrypoint: the operator synthesizes
# an Application that runs the same image with `command: ["/watchtower"]`, and
# finds this image by name through OPERATOR_IMAGE. Its binary embeds a Next.js
# static export, so it cannot be rebuilt from Go source here — lift the binary out
# of the published Watchtower image instead.
ARG WATCHTOWER_IMAGE=us-docker.pkg.dev/wandb-production/public/wandb/watchtower
ARG WATCHTOWER_VERSION=0.11.0

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

FROM ${WATCHTOWER_IMAGE}:${WATCHTOWER_VERSION} AS watchtower

FROM registry.access.redhat.com/ubi9/ubi-minimal

WORKDIR /
COPY --from=manager-builder /workspace/manager .
COPY --from=manager-builder /workspace/crd-installer .
# Built CGO-free on golang:alpine, so it runs unmodified on this glibc base.
COPY --from=watchtower /watchtower .

RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm && \
    chown -R 65532:65532 /helm

USER 65532:65532

ENV HELM_CACHE_HOME=/helm/.cache/helm
ENV HELM_CONFIG_HOME=/helm/.config/helm
ENV HELM_DATA_HOME=/helm/.local/share/helm
ENV OPERATOR_MODE=production

ENTRYPOINT ["/manager"]
