module github.com/jittering/vproxy

go 1.15

require (
	github.com/cbednarski/hostess v0.5.2
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/hairyhenderson/go-which v0.2.0
	github.com/icio/mkcert v0.1.2
	github.com/jittering/truststore v0.0.0-20211026155913-8ecd90d10988 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pelletier/go-toml v1.8.1
	github.com/urfave/cli/v2 v2.3.0
)

replace github.com/jittering/truststore => ../truststore
