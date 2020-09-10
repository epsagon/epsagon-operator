# Build the manager binary
# FROM golang:1.13 as builder
FROM registry.redhat.io/ubi7/go-toolset

LABEL name=epsagon-operator \
      vendor=epsagon \
      description="Epsagon Operator integrated the cluster with Epsagon" \
      summary="Epsagon Operator integrated the cluster with Epsagon"
COPY LICENSE /licenses

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN bash -c "go mod download"

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN bash -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go"

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
# FROM registry.redhat.io/ubi7/ubi
# WORKDIR /
# COPY --from=builder /workspace/manager .

USER nonroot:nonroot

ENTRYPOINT ["/workspace/manager"]
