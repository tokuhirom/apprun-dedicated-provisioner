FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/apprun-dedicated-provisioner /usr/local/bin/apprun-dedicated-provisioner

ENTRYPOINT ["/usr/local/bin/apprun-dedicated-provisioner"]
