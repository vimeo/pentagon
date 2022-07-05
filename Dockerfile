FROM golang:1.18.3-alpine as builder

RUN apk add --no-cache ca-certificates libc-dev git make gcc
RUN adduser -D pentagon
USER pentagon

# Enable go modules
ENV GO111MODULE on

# The golang docker images configure GOPATH=/go
RUN mkdir -p /go/src/github.com/vimeo/pentagon /go/pkg/
COPY --chown=pentagon . /go/src/github.com/vimeo/pentagon/

WORKDIR /go/src/github.com/vimeo/pentagon/

RUN make GOMOD_RO_FLAG='-mod=readonly' build/linux/pentagon

FROM alpine
USER root
RUN adduser -D pentagon
RUN apk add --no-cache ca-certificates
RUN mkdir -p /app
COPY --from=builder /go/src/github.com/vimeo/pentagon/build/linux/pentagon /app/pentagon

# drop privileges again
USER pentagon
ENTRYPOINT ["/app/pentagon"]
