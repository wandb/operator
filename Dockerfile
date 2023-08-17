ARG KUBECTL_VERSION=1.27.3
ARG PNPM_VERSION=8.6.6

# Build the manager binary
FROM golang:1.20 as manager-builder

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
COPY main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY controllers/ controllers/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

FROM gcr.io/distroless/static-debian11

COPY --from=manager-builder /workspace/manager .

ENV OPERATOR_MODE=production
ENV DEPLOYER_API_URL=https://deploy.wandb.ai/api

ENTRYPOINT ["/manager"]
