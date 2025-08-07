# NOTE: This Dockerfile is used by the pipeline, but not for the 'make image' command, which uses the Dockerfile template in hack/common instead.
# Use distroless as minimal base image to package the component binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:cdf4daaf154e3e27cfffc799c16f343a384228f38646928a1513d925f473cb46
ARG TARGETOS
ARG TARGETARCH
ARG COMPONENT
WORKDIR /
COPY bin/$COMPONENT-$TARGETOS.$TARGETARCH /$COMPONENT
USER nonroot:nonroot

ENTRYPOINT ["/$COMPONENT"]
