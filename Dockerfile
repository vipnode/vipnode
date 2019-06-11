# Vipnode binary-builder image using Alpine.
#
# To run a vipnode agent, you'll need to volume-mount the IPC socket or expose
# the appropriate RPC ports to connect to your node.
#
# Example:
#   docker build . --tag "vipnode"
#   docker run --rm -it vipnode --help

# Builder environment
FROM golang:alpine AS builder
RUN apk update && apk --no-cache add git gcc build-base

# Build the source
COPY . /go/src/vipnode
WORKDIR /go/src/vipnode
RUN make

# Run environment
FROM alpine
RUN apk update && apk --no-cache add ca-certificates

# Copy binary to a fresh container, to avoid including build artifact.
COPY --from=builder /go/src/vipnode/vipnode /usr/bin/vipnode
USER nobody
ENTRYPOINT [ "/usr/bin/vipnode" ]
