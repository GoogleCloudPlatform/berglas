# Copyright 2022 The webhook Authors
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

FROM golang:1.18 AS builder

ENV GO111MODULE=on \
  GOPROXY=https://proxy.golang.org,direct \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /src
COPY . .

RUN go build \
  -a \
  -trimpath \
  -ldflags "-s -w -extldflags=-static" \
  -o /bin/webhook \
  .

RUN strip -s /bin/webhook


FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
  update-ca-certificates

COPY --from=builder /bin/webhook /bin/webhook
ENTRYPOINT ["/bin/webhook"]
