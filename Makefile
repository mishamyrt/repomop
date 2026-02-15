.PHONY: all
all: build

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

.PHONY: install
install: build
	rm -f "$(HOME)/.local/bin/repomop"
	cp repomop "$(HOME)/.local/bin/repomop"
	chmod +x "$(HOME)/.local/bin/repomop"

.PHONY: clean
clean:
	rm -f repomop