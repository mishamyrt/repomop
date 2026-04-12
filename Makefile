VERSION := 0.7.1

.PHONY: all
all: build

.PHONY: build
build:
	cargo build --profile release

.PHONY: test
test:
	cargo test

.PHONY: lint
lint:
	cargo clippy

.PHONY: install
install: build
	rm -f "$(HOME)/.local/bin/repomop"
	cp target/release/repomop "$(HOME)/.local/bin/repomop"

.PHONY: clean
clean:
	rm -rf target

.PHONY: publish
publish:
	@sed -E 's/^version = "[^"]+"/version = "${VERSION}"/' Cargo.toml > Cargo.toml.tmp
	@mv Cargo.toml.tmp Cargo.toml
	@cargo update -p repomop
	@git add Makefile Cargo.toml Cargo.lock
	@git commit -m "chore: release ${VERSION} 🔥"
	@git tag "v${VERSION}"
	@git-cliff -o CHANGELOG.md
	@git tag -d "v${VERSION}"
	@git add CHANGELOG.md
	@git commit --amend --no-edit
	@git tag -a "v${VERSION}" -m "release v${VERSION}"
	@git push
	@git push --tags
