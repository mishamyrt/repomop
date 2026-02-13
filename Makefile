.PHONY: build
build:
	CGO_ENABLED=0 \
		go build \
			-ldflags "-s -w" \
			-o repomop \
			./cmd/repomop

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	revive -config revive.toml -formatter stylish ./...
