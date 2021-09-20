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

VETTERS = "asmdecl,assign,atomic,bools,buildtag,cgocall,composites,copylocks,errorsas,httpresponse,loopclosure,lostcancel,nilfunc,printf,shift,stdmethods,structtag,tests,unmarshal,unreachable,unsafeptr,unusedresult"
GOFMT_FILES = $(shell go list -f '{{.Dir}}' ./...)

export GO111MODULE = on
export CGO_ENABLED = 0

builders:
	@cd cloudbuild/builders/go-gcloud-make && \
		gcloud builds submit \
		  --project berglas-test \
		  .
.PHONY: builders

deps:
	@go get -u -t ./...
	@go mod tidy
.PHONY: deps

dev:
	@go install .
.PHONY: dev

docker-push:
	@./bin/docker-push
.PHONY: docker-push

fmtcheck:
	@go install golang.org/x/tools/cmd/goimports
	@CHANGES="$$(goimports -d $(GOFMT_FILES))"; \
		if [ -n "$${CHANGES}" ]; then \
			echo "Unformatted (run goimports -w .):\n\n$${CHANGES}\n\n"; \
			exit 1; \
		fi
	@# Annoyingly, goimports does not support the simplify flag.
	@CHANGES="$$(gofmt -s -d $(GOFMT_FILES))"; \
		if [ -n "$${CHANGES}" ]; then \
			echo "Unformatted (run gofmt -s -w .):\n\n$${CHANGES}\n\n"; \
			exit 1; \
		fi
.PHONY: fmtcheck

publish:
	@GOOSES="${GOOSES}" GOARCHES="${GOARCHES}" ./bin/publish
.PHONY: publish

spellcheck:
	@go install github.com/client9/misspell/cmd/misspell
	@misspell -locale="US" -error -source="text" **/*
.PHONY: spellcheck

staticcheck:
	@go install honnef.co/go/tools/cmd/staticcheck
	@staticcheck -checks="all" -tests $(GOFMT_FILES)
.PHONY: staticcheck

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
			go get github.com/GoogleCloudPlatform/berglas@v0.6.0 && \
			go get ./... && \
			go mod tidy) ; \
	done
.PHONY: update-go-samples
