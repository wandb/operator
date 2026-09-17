# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:f44f6e88636cfb311f9ebace870ded69d943f227bb3cb27d32ffd84ea18c43ea AS manager-builder

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
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY pkg/ pkg/
COPY internal/ internal/

# Cross-compile for the requested image platform using the native build platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:7b8e25a1b56ca4d00219198f3b5b51a3e1693a5c4f5369c5e190d7d6cb3f980e
# Include errata published since the base image was built.
RUN microdnf upgrade --refresh -y && microdnf clean all
WORKDIR /
COPY --from=manager-builder /workspace/manager .

# Create a helm cache directory and set ownership to the non-root user
RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm && chown -R 65532:65532 /helm

USER 65532:65532

ENV HELM_CACHE_HOME=/helm/.cache/helm
ENV HELM_CONFIG_HOME=/helm/.config/helm
ENV HELM_DATA_HOME=/helm/.local/share/helm

ENV OPERATOR_MODE=production
ENV DEPLOYER_API_URL=https://deploy.wandb.ai/api

ENTRYPOINT ["/manager"]
