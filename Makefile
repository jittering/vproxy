
SHELL := bash

# Build the vproxy binary (using go build)
build: clean
	go build ./bin/vproxy
	echo "built ./vproxy"

# Build a snapshot (using goreleaser)
snapshot: clean
	goreleaser release --snapshot --clean

# Install the generated homebrew formula into local homebrew tap
install-formula: snapshot
	[ -z "$$HOMEBREW_PREFIX" ] && echo "HOMEBREW_PREFIX is not set" && exit 1
	cp -a dist/homebrew/Formula/*.rb $${HOMEBREW_PREFIX}/Library/Taps/jittering/homebrew-kegs/Formula/

# Build for linux (x64) (for testing only, release uses goreleaser)
build-linux:
	GOOS=linux go build -o vproxy-linux-x64 ./bin/vproxy/

# Build for mac (x64) (for testing only, release uses goreleaser)
build-mac:
	 GOOS=darwin go build -o vproxy-macos-x64 ./bin/vproxy/

# Build for windows (x64) (for testing only, release uses goreleaser)
build-windows:
	GOOS=windows go build -o vproxy-windows-x64 ./bin/vproxy/

# Release using goreleaser
release: clean
	goreleaser release --clean

# Check goreleaser config and generated homebrew formula style for errors
check-style:
	goreleaser check
	goreleaser --snapshot --skip-validate --clean
	# get cops
	cops=$$(cat /usr/local/Homebrew/Library/Taps/jittering/homebrew-kegs/.rubocop.yml \
		| grep -v Enabled | grep -v '#' | grep -v '^$$' | tr ':\n' ','); \
		brew style --display-cop-names --except-cops="$${cops}" ./dist/*.rb;

## Build and install into homebrew bin path, restart service
build-brew:
	 go build -ldflags \
	 	"-X main.version=snapshot \
			-X main.commit=$$(git rev-parse HEAD) \
			-X main.date=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
			-X main.builtBy=$$(whoami)" \
		-o vproxy ./bin/vproxy/

	 sudo mv vproxy $${HOMEBREW_PREFIX}/opt/vproxy/bin/vproxy
	 sudo pkill -f 'vproxy daemon'

# Clean build artifacts
clean:
	rm -f ./vproxy*
	rm -rf ./dist/

# Build and install to /usr/local/bin
install: build
	sudo cp -a ./vproxy /usr/local/bin/vproxy

help: ## Show make target help
	@(grep -A1 '^#' $(MAKEFILE_LIST) \
		| grep -v '^--$$' \
		| while IFS= read -r line1 && IFS= read -r line2; do \
				if [[ "$$line2" =~ ^[a-zA-Z_-]+:.*$$ ]]; then \
					echo "$$line2 #$$line1"; \
				fi; \
			done; \
		grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST)) \
			| sort \
			| awk '!seen[$$1]++' \
			| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
