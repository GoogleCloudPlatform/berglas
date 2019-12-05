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

NAME = berglas
GOOSES = darwin linux windows
GOARCHES = amd64

export GO111MODULE = on
export GOFLAGS = -mod=vendor
export CGO_ENABLED = 0

builders:
	@cd cloudbuild/builders/go-gcloud-make && \
		gcloud builds submit \
		  --project berglas-test \
		  .
.PHONY: builders

cleanup-gcr:
	@./bin/cleanup-gcr
.PHONY: cleanup-gcr

deps:
	@go get -mod="" -u ./...
	@go mod tidy
	@go mod vendor
.PHONY: deps

dev:
	@go install -i .
.PHONY: dev

docker-push:
	@./bin/docker-push
.PHONY: docker-push

publish:
	@GOOSES="${GOOSES}" GOARCHES="${GOARCHES}" ./bin/publish
.PHONY: publish

test:
	@go test -short -parallel=40 ./...
.PHONY: test

test-acc:
	@go test -parallel=40 -count=1 ./...
.PHONY: test-acc
