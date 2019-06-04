FROM golang:latest AS builder

COPY . /go/src/vipnode
WORKDIR /go/src/vipnode
RUN make

# Copy binary to a fresh container, to avoid including build artifact.

# FIXME: Would be nice to use alpine (reduces the image size from >700mb to
# <30mb, but we'd need to statically link which is tricky with the Keccak256 C
# lib sub-dependency (via geth).
FROM golang:latest
COPY --from=builder /go/src/vipnode/vipnode /vipnode

EXPOSE 8080
ENTRYPOINT [ "/vipnode" ]
