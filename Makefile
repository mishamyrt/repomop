VERSION := 0.5.0

.PHONY: all
all: build

.PHONY: build
build:
	CGO_ENABLED=0 \
		go build \
			-ldflags "-s -w -X 'main.appVersion=v${VERSION}'" \
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

.PHONY: publish
publish:
	@git add Makefile
	@git commit -m "chore: release ${VERSION} 🔥"
	@git tag "v${VERSION}"
	@git-cliff -o CHANGELOG.md
	@git tag -d "v${VERSION}"
	@git add CHANGELOG.md
	@git commit --amend --no-edit
	@git tag -a "v${VERSION}" -m "release v${VERSION}"
	@git push
	@git push --tags