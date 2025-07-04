# Build the manager binary
FROM registry.access.redhat.com/ubi8/go-toolset:latest as builder
ARG TARGETOS
ARG TARGETARCH

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
# TODO No CRDs
# COPY api/ api/
COPY internal/controller/ internal/controller/
COPY internal/common/ internal/common/
COPY internal/webhooks/ internal/webhooks/
COPY pkg/ pkg/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN GOEXPERIMENT=strictfipsruntime \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -o /opt/app-root/manager -tags strictfipsruntime cmd/main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

LABEL com.redhat.component="runtimes-inventory-rhel8-operator-container"
LABEL name="insights-runtimes-tech-preview/runtimes-inventory-rhel8-operator"
LABEL maintainer="Red Hat Insights for Runtimes"
LABEL summary="Operator support for Runtimes Inventory"
LABEL description="A reusable component for Red Hat operators managing Java workloads. \
This component allows these operators to more easily integrate their workloads into \
the Red Hat Insights Runtimes Inventory."
LABEL io.k8s.description="A reusable component for Red Hat operators managing Java workloads. \
This component allows these operators to more easily integrate their workloads into \
the Red Hat Insights Runtimes Inventory."
LABEL io.k8s.display-name="Runtimes Inventory Operator"
LABEL io.openshift.tags="insights,java,openshift"

COPY --from=builder /opt/app-root/manager .

RUN mkdir /licenses
COPY LICENSE /licenses/

USER 1001

ENTRYPOINT ["/manager"]
