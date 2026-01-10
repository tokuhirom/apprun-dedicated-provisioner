FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/apprun-dedicated-application-provisioner /usr/local/bin/apprun-dedicated-application-provisioner

ENTRYPOINT ["/usr/local/bin/apprun-dedicated-application-provisioner"]
