FROM --platform=$BUILDPLATFORM alpine:latest AS builder

RUN apk --no-cache add ca-certificates && \
  update-ca-certificates

COPY berglas /bin/berglas
ENTRYPOINT ["/bin/berglas"]
