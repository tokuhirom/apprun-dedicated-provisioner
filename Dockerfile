FROM gcr.io/distroless/static-debian12:nonroot

COPY apprun-dedicated-application-provisioner /usr/local/bin/apprun-dedicated-application-provisioner

ENTRYPOINT ["/usr/local/bin/apprun-dedicated-application-provisioner"]
