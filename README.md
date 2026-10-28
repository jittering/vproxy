# vproxy

__A zero-config service mesh for local development__

Stop trying to remember port numbers and create secure endpoints for your
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

or manually (macOS, linux, windows):

- Download a [pre-built binary](https://github.com/jittering/vproxy/releases)
- Install in proper location, e.g., `/usr/local/bin/` or `%SYSTEMROOT%\System32\`

 or build it from source (Go 1.25 or newer):

```sh
go install github.com/jittering/vproxy/bin/vproxy@master
```

### Initialize local root CA

Create and install a new local root CA in your system:

```sh
vproxy caroot --create
```

## Usage

vproxy can run in two modes, depending on the use case: single-process for proxying a single service or with a standalone daemon for proxying more than one service.

vproxy consists of two processes: daemon and client.

- The __daemon__ serves as the primary host of the HTTP & HTTPS endpoints for
  your applications.
- The __client__ registers a service with the daemon and relays all access logs
  to the current terminal. It can also optionally run the service for you.

A single daemon is required per-host, while clients can be run multiple times.

In single-process mode, skip the daemon section.

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

### MCP server

The daemon exposes an MCP server at `http://127.0.0.1/_vproxy/mcp` (or
`https://vproxy.local/_vproxy/mcp` when HTTPS is enabled). It uses Streamable HTTP
with stateless JSON responses on the daemon's existing ports.

Tools:

| Tool | Arguments | Action |
| --- | --- | --- |
| `list` | `{}` | List proxies with hostname, upstream host, and port. |
| `add` | `{"binding":"app.local:3000"}` | Add or replace a proxy to a local port. |
| `remove` | `{"host":"app.local"}` | Remove a proxy by its exact hostname. |

Changes are saved to `vhosts.json`, just like CLI changes. Adding a proxy also
updates the hosts file and generates a certificate when TLS is enabled.
Hosts-file update warnings are included in the tool response. Removing a proxy
also removes its hostname from the hosts file, but retains certificates. This
applies to MCP, CLI, and CLI remove-all. If cleanup fails, the proxy is still
removed and a warning is returned.

Authentication is disabled by default. Set a token in your TOML config to require
the `Authorization: Bearer <token>` header on every MCP request:

```toml
[mcp]
token = "replace-with-your-secret-token"
```

Start or restart the daemon with that config:

```sh
vproxy --config /path/to/vproxy.toml daemon
```

Configure your MCP client's Streamable HTTP URL as
`http://127.0.0.1/_vproxy/mcp` and its `Authorization` header as
`Bearer replace-with-your-secret-token`. Use your configured port if different
from 80. Omit the token setting (or use an empty string) to disable authentication.
This token protects the MCP endpoint only; the existing CLI management endpoints
retain their current behavior.

### client

Use the connect command to bind a hostname to a local port:

```sh
$ vproxy connect --bind foo.local.com:5000
[*] registering vhost: foo.local.com:5000
```

The daemon will automatically:

- Issue a TLS cert for `foo.local.com`
- Add `foo.local.com` to your hosts file (e.g., /etc/hosts)
- Add a reverse proxy vhost connecting `foo.local.com` to port 5000

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

Now visit https://foo.local.com to access your application originally running
on http://127.0.0.1:5000

When you stop the client process (i.e., by pressing `^C`), vproxy will deregister the vhost with the daemon and send a TERM signal to it's child process.

### Permissions

A couple of notes on permissions. The vproxy *daemon* must be run with elevated privileges for the following reasons:

- macOS or Linux: binding on privileged ports 80/443 and modifying `/etc/hosts`
- Windows: `modifying %SYSTEMROOT%\System32\drivers\etc\hosts`
- All: installing our local CA into the system trust stores (system prompt)

On mac or linux this means running as root (or via sudo).

On Windows this means running via an elevated command prompt.

## License

MIT, (c) 2022, Pixelcop Research, Inc.

Originally based on [simpleproxy](https://github.com/ybrs/simpleproxy) - MIT (c) 2016 aybars badur.
