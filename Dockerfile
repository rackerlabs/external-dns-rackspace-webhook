FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o external-dns-rackspace-webhook ./cmd/webhook

# final image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/external-dns-rackspace-webhook /opt/external-dns-rackspace-webhook
ENTRYPOINT ["/opt/external-dns-rackspace-webhook"]

