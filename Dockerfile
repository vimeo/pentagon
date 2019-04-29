FROM golang:1.12-alpine as builder

RUN apk add --no-cache ca-certificates libc-dev git make gcc
RUN adduser -D pentagon
USER pentagon

# Enable go modules
ENV GO111MODULE on

# The golang docker images configure GOPATH=/go
RUN mkdir -p /go/src/github.com/vimeo/pentagon /go/pkg/
COPY --chown=pentagon . /go/src/github.com/vimeo/pentagon/

# Copy the vendored gomod_deps directly to /go/pkg/mod
COPY --chown=pentagon ./vendor/gomod_deps/mod /go/pkg/mod

WORKDIR /go/src/github.com/vimeo/pentagon/

RUN make GOMOD_RO_FLAG='-mod=readonly' build/linux/pentagon

FROM alpine
USER root
RUN apk add --no-cache ca-certificates
RUN mkdir -p /app
COPY --from=builder /go/src/github.com/vimeo/pentagon/build/linux/pentagon /app/pentagon
ENTRYPOINT ["/app/pentagon"]
