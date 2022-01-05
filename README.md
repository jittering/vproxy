# vproxy

__A zero-config service mesh for local development__

Stop trying to remember port numbers and create secure endpoints for all of your
services. Automatically updates your hosts file, generates TLS certificates,
(via
[mkcert](https://github.com/FiloSottile/mkcert)/[truststore](https://github.com/jittering/truststore)),
and binds a reverse proxy with a single command.

No third-party dependencies. No Docker. No Kubernetes.

## Quickstart

```sh
brew install jittering/kegs/vproxy
vproxy caroot --create
sudo brew services start vproxy
vproxy connect hello.local:8888 -- vproxy hello
```

Now open https://hello.local in your browser!

## Installation

via homebrew (mac or linux):

```sh
brew install jittering/kegs/vproxy
```

or manually:

Download a [pre-built binary](https://github.com/jittering/vproxy/releases) or build it from source:

```sh
go get github.com/jittering/vproxy/...
```

### Initialize local root CA

Create and install a new local root CA in your system:

```sh
vproxy caroot --create
```

## Usage

vproxy consists of two processes: daemon and client.

* The __daemon__ serves as the primary host of the HTTP & HTTPS endpoints for
  your various applications.
* The __client__ registers a service with the daemon and relays all access logs
  to the current terminal. It can also optionally run the service for you.

A single daemon is required per-host, while clients can be run multiple times.

### daemon

If installed via homebrew on macOS, running it as a service is easy:

```sh
sudo brew services start vproxy
```

> Note that you must run as __root__ to bind to privileged ports (hence the use of
> sudo above).

Or run it manually:

```sh
$ vproxy daemon
[*] rerunning with sudo
Password:
[*] starting proxy: http://127.0.0.1:80
[*] starting proxy: https://127.0.0.1:443
```

### client

Use the connect command to bind a hostname to a local port:

```sh
$ vproxy connect --bind foo.local.com:5000
[*] registering vhost: foo.local.com:5000
```

The daemon will automatically:

* Issue a TLS cert for `foo.local.com`
* Add `foo.local.com` to your hosts file (e.g., /etc/hosts)
* Add a reverse proxy vhost connecting `foo.local.com` to port 5000

You can even run the underlying service with one command, for ease of use:

```sh
$ vproxy connect --bind foo.local.com:5000 -- flask run
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
on http://127.0.0.1:5000

When you stop the client process (i.e., by pressing `^C`), vproxy will deregister the vhost with the daemon and send a TERM signal to it's child process.

## License

MIT, (c) 2021, Pixelcop Research, Inc.

Originally based on [simpleproxy](https://github.com/ybrs/simpleproxy) - MIT (c) 2016 aybars badur.
