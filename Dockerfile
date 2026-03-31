FROM --platform=$BUILDPLATFORM alpine:latest AS builder

RUN apk --no-cache add ca-certificates && \
  update-ca-certificates

RUN apk update && apk upgrade --no-cache

COPY berglas /bin/berglas
ENTRYPOINT ["/bin/berglas"]
