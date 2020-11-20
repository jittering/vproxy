
build: build-linux build-mac

build-linux:
	GOOS=linux go build -o vproxy-linux-x64 ./bin/vproxy/

build-mac:
	 GOOS=darwin go build -o vproxy-macos-x64 ./bin/vproxy/

release:
	goreleaser release --rm-dist

