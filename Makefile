test:
	@go test \
		-count=1 \
		-short \
		-shuffle=on \
		./...
.PHONY: test

test-acc:
	@go test \
		-count=1 \
		-race \
		-shuffle=on \
		./...
.PHONY: test-acc

update-go-samples:
	for p in $(shell find examples -name go.mod -type f); do \
		dir=$$(dirname $${p}) ; \
		rm -f $${dir}/go.mod $${dir}/go.sum ; \
		(cd $${dir} && \
			go mod init github.com/GoogleCloudPlatform/berglas/$${dir} && \
			go get -u github.com/GoogleCloudPlatform/berglas@main && \
			go get -u ./... && \
			go mod tidy -go=1.21 -compat=1.21) ; \
	done
.PHONY: update-go-samples
