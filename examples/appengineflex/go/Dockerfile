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

ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /src
COPY . .

RUN go build \
  -a \
  -ldflags "-s -w -extldflags 'static'" \
  -installsuffix cgo \
  -tags netgo \
  -o /bin/app \
  .



FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
  update-ca-certificates

COPY --from=builder /bin/app /bin/app
ENTRYPOINT ["/bin/app"]
