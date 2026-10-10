<img src="./data/dinosaur.svg" width="360" />

# Dinosaur DNS

A DNS caching proxy for local networks. Supports UDP, DNS-over-TLS (DoT),
and DNS-over-HTTPS (DoH) upstreams, an in-memory cache, qtype-aware
blocklists, local authoritative entries, ACLs, and an optional HTTP API.

See [ARCHITECTURE.md](ARCHITECTURE.md) for a component overview.

## Build

Requires Go 1.25 or later.

```
git clone https://github.com/paulc/dinosaur-dns
cd dinosaur-dns
go build ./cmd/dinosaur
```

## Quick start

Listen on localhost port 53, use Cloudflare DoT as upstream:

```
sudo ./dinosaur -listen 127.0.0.1:53 -upstream tls://1.1.1.1:853
```

Without flags, the server listens on `lo0:8053` (all addresses on the `lo0`
interface) and uses `tls://1.1.1.1:853` and `tls://1.0.0.1:853`.

## Upstream formats

| Format | Protocol |
|--------|----------|
| `1.1.1.1:53` | UDP (port optional, default 53) |
| `tls://1.1.1.1:853` | DNS-over-TLS |
| `https://cloudflare-dns.com/dns-query` | DNS-over-HTTPS |

Multiple `-upstream` flags are tried in order. If the active (first) upstream
fails more than `-upstream-error-threshold` consecutive times (default 3) it is
demoted to the end of the list. The active upstream can also be changed from
the Config tab of the web dashboard (or `api.SetUpstream`).

### Bootstrapping DoT/DoH upstream hostnames

DoT/DoH upstreams given by hostname (e.g. `https://dns.google/dns-query`)
must be resolved before dinosaur can connect to them. This happens whenever a
new upstream connection is opened, in this order:

1. If any `-bootstrap` resolvers (`ip[:port]`, default port 53) are given,
   A/AAAA queries are sent directly to each one in turn and the first to
   return addresses is used. The system resolver (including `/etc/hosts`) is
   **not** used.
2. Otherwise the system resolver is used.
3. If that lookup fails (e.g. the network is down at startup, or the system
   resolver points back at dinosaur) the static `-bootstrap-host` mappings
   are used.

```
./dinosaur -upstream https://dns.google/dns-query \
           -bootstrap 9.9.9.9 -bootstrap 149.112.112.112:53 \
           -bootstrap-host dns.google=8.8.8.8,8.8.4.4
```

TLS certificates are always verified against the upstream hostname.
Upstreams given as IP addresses don't need bootstrapping, and plain UDP
upstreams given by hostname always use the system resolver.

## Listen address formats

| Format | Meaning |
|--------|---------|
| `127.0.0.1:53` | specific IP and port |
| `lo0:8053` | all addresses on interface `lo0`, port 8053 |
| `lo0` | all addresses on interface `lo0`, port 53 |

Multiple `-listen` flags are accepted. Listen addresses which can't be bound
(e.g. a missing interface) are logged and skipped rather than being fatal.

## Blocklist

Block a domain for all query types:

```
./dinosaur -block ads.example.com
```

Block only AAAA lookups (useful for IPv6 tunnel workarounds):

```
./dinosaur -block ads.example.com:AAAA
```

Load a blocklist from a file or URL (one domain per line):

```
./dinosaur -blocklist /etc/dns/blocklist.txt
./dinosaur -blocklist https://example.com/blocklist.txt
```

Load a blocklist in `/etc/hosts` format (e.g. Steven Black's list):

```
./dinosaur -blocklist-from-hosts https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
```

Block all AAAA for a list of domains (useful for Netflix CDN with IPv6
tunnels -- see https://openconnect.netflix.com/mobiledeliverydomains.txt):

```
./dinosaur -blocklist-aaaa https://openconnect.netflix.com/mobiledeliverydomains.txt
```

Auto-refresh blocklists every 6 hours:

```
./dinosaur -blocklist /etc/dns/blocklist.txt -refresh -refresh-interval 6h
```

Block rules added or deleted via the web UI/API are re-applied after each
refresh.

## Local entries

Add a static DNS record:

```
./dinosaur -localrr "host.local. 3600 IN A 192.168.1.10"
```

Add a static record and automatically create the reverse PTR:

```
./dinosaur -localrr-ptr "host.local. 3600 IN A 192.168.1.10"
```

Load a zone file:

```
./dinosaur -localzone /etc/dns/local.zone
```

## ACL

Restrict which clients may query the server:

```
./dinosaur -listen 0.0.0.0:53 -acl 192.168.1.0/24 -acl 127.0.0.1/32
```

With no `-acl` flags all clients are permitted.

## DNS64

Synthesise AAAA records when no upstream AAAA answer exists, using the well-known prefix (applies to all clients):

```
./dinosaur -dns64
```

Use a custom prefix:

```
./dinosaur -dns64 -dns64-prefix 2001:db8::/96
```

## JSON config

All flags can be specified in a JSON file:

```json
{
  "listen":   ["127.0.0.1:53"],
  "upstream": ["tls://1.1.1.1:853", "tls://1.0.0.1:853"],
  "upstream-error-threshold": 3,
  "bootstrap": ["9.9.9.9"],
  "bootstrap-hosts": {"dns.google": ["8.8.8.8", "8.8.4.4"]},
  "blocklist": ["/etc/dns/blocklist.txt"],
  "localrr":  ["host.local. 3600 IN A 192.168.1.10"],
  "acl":      ["192.168.1.0/24"],
  "api":      true,
  "api-bind": "https://192.168.1.1:8553",
  "api-auth": ["admin:secret"],
  "ssl-cert": "/etc/ssl/dns.crt",
  "ssl-key":  "/etc/ssl/dns.key",
  "refresh":  true,
  "refresh-interval": "6h",
  "debug":    false
}
```

```
./dinosaur -config /etc/dns/dinosaur.json
```

Command-line flags take precedence over the config file and are appended to
any list-valued fields. An empty config file is treated as `{}`; unknown keys
are errors.

### Checking a config file

`-config-test <file>` checks a config file without starting the server. It
parses the file (as `-config` does, unknown keys and trailing data are
errors), then validates it in the same way as at startup (blocklist and zone
files/URLs are loaded, ACLs, durations and bootstrap entries are parsed, and
DoH certificate/key files are loaded if a DoH listener is configured).
`setuid` is not checked (it requires root, and the user/group may only exist
on the target host) but is included in the output. Any other flags given are
merged in as usual.

On success the normalised config, with defaults filled in (as shown by
"Generate merged config" in the web UI), is printed to stdout and the exit
status is 0. On error the message (with line:column for JSON errors) is
printed to stderr and the exit status is 1:

```
$ ./dinosaur -config-test /etc/dns/dinosaur.json
Config Error: /etc/dns/dinosaur.json:3:10: invalid character 'y' looking for beginning of value
```

Use an empty file to print the default config:

```
./dinosaur -config-test /dev/null > dinosaur.json
```

`-config-test` and `-config` can't be used together.

## API

Enable the HTTP API (default bind `http://127.0.0.1:8553`):

```
./dinosaur -api
./dinosaur -api -api-bind http://127.0.0.1:9000
./dinosaur -api -api-bind /run/dinosaur.sock

# HTTPS (auto-generated self-signed certificate)
./dinosaur -api -api-bind https://192.168.1.1:8553

# HTTPS with a real certificate and basic auth
./dinosaur -api -api-bind https://dns.example.com:8553 \
  -ssl-cert /etc/ssl/dns.crt -ssl-key /etc/ssl/dns.key \
  -api-auth admin:secret -api-auth readonly:password
```

`-api-bind` is a URL (`http://host[:port]` or `https://host[:port]`, default
port 8553), a plain `host[:port]` (HTTP) or an absolute UNIX socket path
(plain HTTP). IPv6 addresses must be in brackets (`[::1]:8553`). Use
`0.0.0.0` or `[::]` as the host to listen on all addresses. HTTPS uses the
same `-ssl-cert`/`-ssl-key` as the DoH server; if these are omitted a
self-signed certificate is generated at startup (shared by DoH and the API).
The self-signed certificate includes the HTTPS API and DoH bind hosts as
subject alternative names (except `0.0.0.0`/`[::]`), so clients can verify it
once they trust it (e.g. `json-rpc --cacert`) rather than skipping
verification.

`-api-auth user:password` enables HTTP basic auth for all API endpoints
(including the dashboard). It may be given multiple times to allow several
users. Credentials are sent in clear text over `http://`, so use `https://`
for anything other than a loopback address or UNIX socket.

**Note:** anyone with API access can read all of the `api-auth` passwords:
they are included in full in the output of `api.Config` and
`api.GetMergedConfig` (and so on the dashboard Config tab), so that the merged
config can be used as a config file. Every API user is effectively an admin.

| Flag | Description |
|------|-------------|
| `-api` | Enable the API |
| `-api-bind` | API bind URL or UNIX socket path (default: `http://127.0.0.1:8553`) |
| `-api-auth` | Basic auth credentials `user:password` (multiple allowed) |
| `-ssl-cert` | TLS certificate file (PEM) for HTTPS API and DoH |
| `-ssl-key` | TLS private key file (PEM) for HTTPS API and DoH |

Equivalent JSON config keys: `api`, `api-bind`, `api-auth` (list),
`ssl-cert`, `ssl-key`.

Endpoints:

| Path | Description |
|------|-------------|
| `GET /` | redirect to dashboard |
| `GET /ping` | returns `PONG` |
| `POST /api` | JSON-RPC 2.0 (see below) |
| `GET /log` | SSE stream of query log |
| `GET /static/` | web dashboard |

JSON-RPC methods:

| Method | Description |
|--------|-------------|
| `api.Config` | Return startup configuration |
| `api.CacheAdd` | Add a DNS record to the cache |
| `api.CacheDelete` | Remove a record from the cache |
| `api.CacheDebug` | List all cache entries |
| `api.BlockListCount` | Number of blocked entries |
| `api.BlockListAdd` | Add one or more block rules |
| `api.BlockListDelete` | Remove a block rule |
| `api.BlockListList` | List all block rules |
| `api.GetUpstreams` | List upstreams and show the active one |
| `api.SetUpstream` | Make a configured upstream active |
| `api.GetBlockingStatus` | Check whether blocking is paused |
| `api.PauseBlocking` | Pause all block rules for N seconds |
| `api.ResumeBlocking` | Resume blocking immediately |
| `api.GetChanges` | Net web-UI changes since server start |
| `api.GetMergedConfig` | Startup config merged with web-UI changes |

The web dashboard (served at `/static/index.html`) provides a query log,
blocklist management (including a timed pause), cache inspection, config
viewer with change tracking, and API reference. Full JSON-RPC documentation
is available on the API tab of the dashboard. The dashboard works over both
HTTP and HTTPS; with `-api-auth` the browser prompts for credentials.

## DoH server

Accept queries from downstream clients over DNS-over-HTTPS (RFC 8484).
Supports both GET and POST request methods; HTTP/2 and keep-alive are
enabled automatically. All proxy features (blocklist, cache, ACL, DNS64) apply
exactly as they do for UDP/TCP clients.

```
# Listen on port 8443, auto-generate a self-signed certificate
./dinosaur -doh 127.0.0.1:8443

# Listen on all addresses of eth0, use a real certificate
./dinosaur -doh eth0:443 -ssl-cert /etc/ssl/doh.crt -ssl-key /etc/ssl/doh.key

# Custom path
./dinosaur -doh 127.0.0.1:8443 -doh-path /resolve
```

Multiple `-doh` flags are accepted (one listener each). If `-ssl-cert` and
`-ssl-key` are omitted a self-signed ECDSA-P256 certificate (with the bind
addresses as subject alternative names) is generated at startup. The same
certificate is used by the HTTPS API.

| Flag | Description |
|------|-------------|
| `-doh` | Listen address/interface (default port 443; multiple allowed) |
| `-ssl-cert` | TLS certificate file (PEM) |
| `-ssl-key` | TLS private key file (PEM) |
| `-doh-path` | Request path (default: `/dns-query`) |

Equivalent JSON config keys: `doh`, `ssl-cert`, `ssl-key`, `doh-path`.

Query a running DoH server:

```
# GET
curl -sk "https://localhost:8443/dns-query?dns=$(echo -n <base64url-msg>)"

# POST
curl -sk -X POST https://localhost:8443/dns-query \
  -H 'Content-Type: application/dns-message' --data-binary @query.bin
```

## json-rpc utility

`cmd/json-rpc` is a standalone CLI for calling any JSON-RPC 2.0 endpoint.

```
go build ./cmd/json-rpc
```

```
# No params (defaults to {})
./json-rpc --url http://localhost:8553/api --method api.Config --pretty

# Params via flag
./json-rpc --url http://localhost:8553/api --method api.BlockListAdd \
  --params '{"entries":["ads.example.com"]}'

# Params via stdin
echo '{"seconds":300}' | \
  ./json-rpc --url http://localhost:8553/api --method api.PauseBlocking --pretty

# HTTPS with basic auth (self-signed certificate)
./json-rpc --url https://192.168.1.1:8553/api --insecure \
  --auth admin:secret --method api.Config --pretty

# Credentials from the environment (or in the URL: https://admin:secret@host/api)
JSONRPC_AUTH=admin:secret ./json-rpc --url https://dns.example.com:8553/api \
  --cacert /etc/ssl/ca.crt --method api.GetUpstreams
```

| Flag | Description |
|------|-------------|
| `--url` | JSON-RPC endpoint URL (required) |
| `--method` | Method name, e.g. `api.Config` (required) |
| `--params` | JSON params string (object or array); reads stdin if omitted |
| `--id` | Request ID (random if 0) |
| `--pretty` | Pretty-print JSON output |
| `--auth` | HTTP basic auth `user:password` (default: `$JSONRPC_AUTH` or credentials in URL) |
| `--insecure` | Skip TLS certificate verification (e.g. self-signed certificate) |
| `--cacert` | PEM CA certificate file used to verify the server certificate |
| `--debug` | Print request body and HTTP status to stderr |

Exit codes: 0 = success, 1 = usage/network error, 2 = HTTP non-200.

## Logging

| Flag | Effect |
|------|--------|
| (default) | log to stderr |
| `-debug` | include debug messages |
| `-syslog` | log to syslog |
| `-discard` | suppress all log output |

## setuid

Drop privileges after binding the port:

```
sudo ./dinosaur -listen 0.0.0.0:53 -setuid nobody:nobody
```

## All flags

```
  -acl value
        Access control list (CIDR)
  -api
        Enable API (default: false)
  -api-auth value
        API HTTP basic auth credentials (format: 'user:password', default: no auth)
  -api-bind string
        API bind URL [http://host:port, https://host:port or UNIX socket path] (default: http://127.0.0.1:8553)
  -block value
        Block entry (format: 'domain[:qtype]')
  -block-delete value
        Delete block entry (format: 'domain[:qtype]')
  -blocklist value
        Blocklist file or URL
  -blocklist-aaaa value
        Blocklist file or URL (blocks AAAA only)
  -blocklist-from-hosts value
        Blocklist from /etc/hosts format file or URL
  -bootstrap value
        Resolver for DoT/DoH upstream hostnames, used instead of the system resolver [ip[:port]] (default: system resolver)
  -bootstrap-host value
        Fallback DoT/DoH upstream hostname mapping if bootstrap lookup fails (format: 'hostname=ip[,ip...]')
  -config string
        JSON config file
  -config-test string
        Check JSON config file and print normalised config (with defaults) to stdout
  -debug
        Debug logging (default: false)
  -discard
        Discard all log output (default: false)
  -dns64
        Enable DNS64 (default: false)
  -dns64-prefix string
        DNS64 prefix (default: 64:ff9b::/96)
  -doh value
        DoH listen address/interface (enables DoH server, default port 443)
  -doh-path string
        DoH request path (default: /dns-query)
  -help
        Show usage
  -listen value
        Listen address/interface (default: lo0:8053)
  -localrr value
        Local DNS resource record
  -localrr-ptr value
        Local DNS resource record with auto PTR
  -localzone value
        Local DNS zone file
  -refresh
        Auto-refresh blocklists (default: false)
  -refresh-interval string
        Blocklist refresh interval (default: 24h)
  -setuid string
        Drop to user[:group] after binding (default: none)
  -ssl-cert string
        TLS certificate file for DoH/HTTPS API (auto-generates self-signed if omitted)
  -ssl-key string
        TLS private key file for DoH/HTTPS API
  -syslog
        Log to syslog (default: false)
  -upstream value
        Upstream resolver (default: tls://1.1.1.1:853 tls://1.0.0.1:853)
  -upstream-error-threshold int
        Consecutive upstream errors before switching to next upstream (default: 3)
  -version
        Show version
```
