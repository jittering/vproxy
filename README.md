# vproxy

> Zero-config virtual proxies with TLS

Automatically create and manage hosts files and TLS certificates for any
hostname using a locally-trusted CA (via
[mkcert](https://github.com/FiloSottile/mkcert/)).

## Installation

via homebrew (mac or linux):

```sh
brew tap jittering/kegs
brew install vproxy
```

or manually:

1. Install [mkcert](https://github.com/FiloSottile/mkcert/#installation)
2. Build it:

```sh
go get github.com/jittering/vproxy/...
```

## Usage

vproxy consists of two processes: daemon and client.

* The __daemon__ serves as the primary host of the HTTP & HTTPS endpoints for
  your various applications.
* The __client__ registers a service with the daemon and relays all access logs
  to the current terminal.

### daemon

A single daemon is required per-host:

```sh
$ vproxy daemon
[*] rerunning with sudo
Password:
[*] starting proxy: http://127.0.0.1:80
[*] starting proxy: https://127.0.0.1:443
```

Note that *sudo* is required to bind to privileged ports.

Alternatively, run it as a homebrew service (macOS only), so it will start
automatically:

```sh
sudo brew services start vproxy
```

### client

```sh
$ vproxy client --bind foo.local.com:5000
[*] registering vhost: foo.local.com:5000
```

You can even run the underlying service directly, for ease of use:

```sh
$ vproxy client --bind foo.local.com:5000 -- flask run
[*] running command: /usr/local/bin/flask run
[*] registering vhost: foo.local.com:5000
 * Serving Flask app "app/main.py"
 * Environment: production
   WARNING: This is a development server. Do not use it in a production deployment.
   Use a production WSGI server instead.
 * Debug mode: off
 * Running on http://127.0.0.1:5000/ (Press CTRL+C to quit)
```

Now visit https://foo.local.com/ to access your application originally running
on http://127.0.0.1:8080

## License

MIT, (c) 2020, Pixelcop Research, Inc.

Originally based on [simpleproxy](https://github.com/ybrs/simpleproxy) - MIT (c) 2016 aybars badur.
