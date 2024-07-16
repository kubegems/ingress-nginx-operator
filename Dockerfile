# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot
ARG TARGETOS TARGETARCH
COPY manager-${TARGETOS}-${TARGETARCH} /manager
USER 65532:65532
WORKDIR /
ENTRYPOINT ["/manager"]
