
SHELL := bash

build: clean
	go build ./bin/vproxy

snapshot: clean
	goreleaser release --snapshot --clean

install-formula: snapshot
	cp -a dist/homebrew/Formula/*.rb /usr/local/Homebrew/Library/Taps/jittering/homebrew-kegs/Formula/

build-linux:
	GOOS=linux go build -o vproxy-linux-x64 ./bin/vproxy/

build-mac:
	 GOOS=darwin go build -o vproxy-macos-x64 ./bin/vproxy/

build-windows:
	GOOS=windows go build -o vproxy-windows-x64 ./bin/vproxy/

release: clean
	goreleaser release --clean

check-style:
	goreleaser check
	goreleaser --snapshot --skip-validate --clean
	# get cops
	cops=$$(cat /usr/local/Homebrew/Library/Taps/jittering/homebrew-kegs/.rubocop.yml \
		| grep -v Enabled | grep -v '#' | grep -v '^$$' | tr ':\n' ','); \
		brew style --display-cop-names --except-cops="$${cops}" ./dist/*.rb;

build-brew:
	 go build -o vproxy ./bin/vproxy/
	 sudo mv vproxy /usr/local/opt/vproxy/bin/vproxy
	 sudo pkill -f 'vproxy daemon'

clean:
	rm -f ./vproxy*
	rm -rf ./dist/

install: build
	sudo cp -a ./vproxy /usr/local/bin/vproxy
