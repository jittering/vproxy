
build: clean
	goreleaser release --snapshot --rm-dist

install-formula: build
	cp -a dist/vproxy.rb dist/vproxy-head.rb /usr/local/Homebrew/Library/Taps/jittering/homebrew-kegs/Formula/

build-linux:
	GOOS=linux go build -o vproxy-linux-x64 ./bin/vproxy/

build-mac:
	 GOOS=darwin go build -o vproxy-macos-x64 ./bin/vproxy/

release: clean
	goreleaser release --rm-dist

build-brew:
	 go build -o vproxy ./bin/vproxy/
	 sudo mv vproxy /usr/local/opt/vproxy/bin/vproxy
	 sudo pkill -f 'vproxy daemon'

clean:
	rm -f ./vproxy*
	rm -rf ./dist/

vproxy:
	go build ./bin/vproxy

install: vproxy
	sudo cp -a ./vproxy /usr/local/bin/vproxy
