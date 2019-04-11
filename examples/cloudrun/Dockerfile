# Copyright 2019 The Berglas Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.12 AS builder

WORKDIR /src
COPY . .

RUN env \
  GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build \
    -a \
    -ldflags "-s -w -extldflags '-static'" \
    -mod vendor \
    -installsuffix cgo \
    -tags netgo \
    -o /bin/app \
    .

FROM alpine:latest AS certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=builder /bin/app /bin/app
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/bin/app"]


# FROM alpine:latest
# RUN apk --no-cache add ca-certificates && \
#   update-ca-certificates
#
# RUN addgroup -g 1001 appgroup && \
#   adduser -H -D -s /bin/false -G appgroup -u 1001 appuser
#
# USER 1001:1001
# COPY --from=builder /bin/app /bin/app
# ENTRYPOINT ["/bin/app"]
