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
