# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.25.7-1773851748@sha256:8c5d30cb3c2b44165c332131444c534d259094be7715ac3ead2409f9fb0dbb6d AS builder
ARG TARGETOS
ARG TARGETARCH

ENV GOTOOLCHAIN=auto
WORKDIR /opt/app-root/src
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Coverage instrumentation build argument
ARG ENABLE_COVERAGE=false

# Copy the go source
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

# Build with or without coverage instrumentation
RUN if [ "$ENABLE_COVERAGE" = "true" ]; then \
        echo "Building with coverage instrumentation..."; \
        CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -cover -covermode=atomic -tags=coverage -o manager ./cmd/; \
    else \
        echo "Building production binary..."; \
        CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager ./cmd/; \
    fi


FROM registry.access.redhat.com/ubi9-micro@sha256:2173487b3b72b1a7b11edc908e9bbf1726f9df46a4f78fd6d19a2bab0a701f38
WORKDIR /
COPY --from=builder /opt/app-root/src/manager .
COPY LICENSE /licenses/
USER 65532:65532

# It is mandatory to set these labels
LABEL name="Tekton Kueue Extension"
LABEL description="Tekton Kueue Extension"
LABEL com.redhat.component="Tekton Kueue Extension"
LABEL io.k8s.description="Tekton Kueue Extension"
LABEL io.k8s.display-name="Tekton Kueue Extension"

ENTRYPOINT ["/manager"]
