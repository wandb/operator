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

# Prepare writable Helm directories before entering the shell-free runtime image.
RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm

# The CGO-disabled manager needs CA certificates and runtime data, not a C library,
# shell, or package manager. Pin the multi-platform index for reproducible builds.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
WORKDIR /
COPY --from=manager-builder /workspace/manager .

COPY --from=manager-builder --chown=65532:65532 /helm /helm

USER 65532:65532

ENV HELM_CACHE_HOME=/helm/.cache/helm
ENV HELM_CONFIG_HOME=/helm/.config/helm
ENV HELM_DATA_HOME=/helm/.local/share/helm

ENV OPERATOR_MODE=production
ENV DEPLOYER_API_URL=https://deploy.wandb.ai/api

ENTRYPOINT ["/manager"]
