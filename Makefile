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

dev:
	@go install .
.PHONY: dev

test:
	@go test -short -parallel=40 ./...
.PHONY: test

test-acc:
	@go test -parallel=20 -count=1 ./...
.PHONY: test-acc

update-go-samples:
	for p in $(shell find examples -name go.mod -type f); do \
		dir=$$(dirname $${p}) ; \
		rm -f $${dir}/go.mod $${dir}/go.sum ; \
		(cd $${dir} && \
			go mod init github.com/GoogleCloudPlatform/berglas/$${dir} && \
			go get -u github.com/GoogleCloudPlatform/berglas@main && \
			go get -u ./... && \
			go mod tidy -go=1.19 -compat=1.19) ; \
	done
.PHONY: update-go-samples
