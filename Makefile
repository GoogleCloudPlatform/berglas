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

VERSION = $(shell go run main.go version)

dev:
	@go install ./...

docker-push:
	@gcloud builds submit \
		--project berglas \
		--tag gcr.io/berglas/berglas:$(VERSION) \
		.
	@gcloud container images add-tag \
		--project berglas \
		--quiet \
		gcr.io/berglas/berglas:$(VERSION) \
		gcr.io/berglas/berglas:latest

test:
	@go test -short -parallel=40 ./...

test-acc:
	@go test -parallel=40 ./...
