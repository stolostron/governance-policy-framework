# Stage 1: Use image builder to build the target binaries
# Copyright Contributors to the Open Cluster Management project

FROM golang:1.19 AS builder

ENV COMPONENT=governance-policy-framework
ENV REPO_PATH=/go/src/open-cluster-management.io/${COMPONENT}
WORKDIR ${REPO_PATH}
COPY . .
RUN make build

# Stage 2: Copy the binaries from the image builder to the base image
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV COMPONENT=governance-policy-framework
ENV REPO_PATH=/go/src/open-cluster-management.io/${COMPONENT}
ENV OPERATOR=/usr/local/bin/${COMPONENT} \
    USER_UID=1001 \
    USER_NAME=${COMPONENT}

# install operator binary
COPY --from=builder ${REPO_PATH}/build/_output/bin/${COMPONENT} ${OPERATOR}

COPY --from=builder ${REPO_PATH}/build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

RUN microdnf update && \
    microdnf clean all

USER ${USER_UID}
