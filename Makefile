
build: build-linux build-mac

build-linux:
	GOOS=linux go build -o vproxy-linux-x64 ./bin/vproxy/

build-mac:
	 GOOS=darwin go build -o vproxy-macos-x64 ./bin/vproxy/

release: clean
	goreleaser release --rm-dist

build-brew:
	 go build -o vproxy ./bin/vproxy/
	 sudo mv vproxy /usr/local/opt/vproxy/bin/vproxy
	 sudo killall vproxy

clean:
	rm -f ./vproxy*
	rm -rf ./dist/

vproxy:
	go build ./bin/vproxy

install: vproxy
	sudo cp -a ./vproxy /usr/local/bin/vproxy
