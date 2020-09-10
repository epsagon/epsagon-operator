# Build the manager binary
# FROM golang:1.13 as builder
FROM registry.access.redhat.com/ubi8/ubi-minimal

LABEL name=epsagon-operator \
      vendor=epsagon \
      description="Epsagon Operator integrated the cluster with Epsagon" \
      summary="Epsagon Operator integrated the cluster with Epsagon"
RUN mkdir /licenses
COPY LICENSE /licenses/LICENSE.txt
ENV OPERATOR=/usr/local/bin/epsagon-operator \
    USER_UID=1001 \
    USER_NAME=epsagon-operator

COPY bin/epsagon-operator ${OPERATOR}
COPY bin/user_setup /usr/local/bin/user_setup
RUN /usr/local/bin/user_setup

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
# FROM registry.redhat.io/ubi7/ubi
# WORKDIR /
# COPY --from=builder /workspace/manager .

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
