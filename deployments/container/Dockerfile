ARG GOLANG_VERSION=undefined
ARG BASE_IMAGE=undefined
FROM golang:${GOLANG_VERSION} as build

ENV GOPROXY=direct

WORKDIR /build
COPY . .

RUN mkdir /artifacts
RUN make PREFIX=/artifacts cmds

FROM ${BASE_IMAGE}

COPY --from=build /artifacts/dra-kubelet-plugin /usr/bin/dra-kubelet-plugin
COPY --from=build /artifacts/oci-hook           /usr/bin/oci-hook
