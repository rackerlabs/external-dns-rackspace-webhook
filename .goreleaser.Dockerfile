# goreleaser is making the binary dynamically linked so can't use the static container
FROM gcr.io/distroless/base-debian12:nonroot

USER 20000:20000
COPY --chmod=555 external-dns-rackspace-webhook /usr/local/bin/external-dns-rackspace-webhook
ENTRYPOINT ["/usr/local/bin/external-dns-rackspace-webhook"]