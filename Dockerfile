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

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build \
  -a \
  -ldflags "-s -w -extldflags 'static'" \
  -installsuffix cgo \
  -tags netgo \
  -o /bin/berglas \
  .



FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
  update-ca-certificates

RUN addgroup -g 1001 berglasgroup && \
  adduser -H -D -s /bin/false -G berglasgroup -u 1001 berglasuser

USER 1001:1001
COPY --from=builder /bin/berglas /bin/berglas
ENTRYPOINT ["/bin/berglas"]
