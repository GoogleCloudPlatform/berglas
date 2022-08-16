FROM --platform=$BUILDPLATFORM alpine AS builder

RUN apk --no-cache add ca-certificates && \
  update-ca-certificates

COPY berglas /berglas
ENTRYPOINT ["/berglas"]
