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
| `1.1.1.1:53` | UDP (port required) |
| `tls://1.1.1.1:853` | DNS-over-TLS |
| `https://cloudflare-dns.com/dns-query` | DNS-over-HTTPS |

Multiple `-upstream` flags are tried in order. If the first upstream fails
three consecutive times it is demoted to the end of the list.

## Listen address formats

| Format | Meaning |
|--------|---------|
| `127.0.0.1:53` | specific IP and port |
| `lo0:8053` | all addresses on interface `lo0`, port 8053 |
| `lo0` | all addresses on interface `lo0`, port 53 |

Multiple `-listen` flags are accepted.

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
  "blocklist": ["/etc/dns/blocklist.txt"],
  "localrr":  ["host.local. 3600 IN A 192.168.1.10"],
  "acl":      ["192.168.1.0/24"],
  "api":      true,
  "refresh":  true,
  "refresh-interval": "6h",
  "debug":    false
}
```

```
./dinosaur -config /etc/dns/dinosaur.json
```

Command-line flags take precedence over the config file and are appended to
any list-valued fields.

## API

Enable the HTTP API (default bind `127.0.0.1:8553`):

```
./dinosaur -api
./dinosaur -api -api-bind 127.0.0.1:9000
./dinosaur -api -api-bind /run/dinosaur.sock
```

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
| `api.GetBlockingStatus` | Check whether blocking is paused |
| `api.PauseBlocking` | Pause all block rules for N seconds |
| `api.ResumeBlocking` | Resume blocking immediately |
| `api.GetChanges` | Net web-UI changes since server start |
| `api.GetMergedConfig` | Startup config merged with web-UI changes |

The web dashboard (served at `/static/index.html`) provides a query log,
blocklist management (including a timed pause), cache inspection, config
viewer with change tracking, and API reference. Full JSON-RPC documentation
is available on the API tab of the dashboard.

## DoH server

Accept queries from downstream clients over DNS-over-HTTPS (RFC 8484).
Supports both GET and POST request methods; HTTP/2 and keep-alive are
enabled automatically. All proxy features (blocklist, cache, ACL, DNS64) apply
exactly as they do for UDP/TCP clients.

```
# Listen on port 8443, auto-generate a self-signed certificate
./dinosaur -doh 127.0.0.1:8443

# Listen on all addresses of eth0, use a real certificate
./dinosaur -doh eth0:443 -doh-cert /etc/ssl/doh.crt -doh-key /etc/ssl/doh.key

# Custom path
./dinosaur -doh 127.0.0.1:8443 -doh-path /resolve
```

Multiple `-doh` flags are accepted (one listener each). If `-doh-cert` and
`-doh-key` are omitted a self-signed ECDSA-P256 certificate is generated at
startup.

| Flag | Description |
|------|-------------|
| `-doh` | Listen address/interface (default port 443; multiple allowed) |
| `-doh-cert` | TLS certificate file (PEM) |
| `-doh-key` | TLS private key file (PEM) |
| `-doh-path` | Request path (default: `/dns-query`) |

Equivalent JSON config keys: `doh`, `doh-cert`, `doh-key`, `doh-path`.

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
```

| Flag | Description |
|------|-------------|
| `--url` | JSON-RPC endpoint URL (required) |
| `--method` | Method name, e.g. `api.Config` (required) |
| `--params` | JSON params string (object or array); reads stdin if omitted |
| `--id` | Request ID (random if 0) |
| `--pretty` | Pretty-print JSON output |
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
  -api-bind string
        API bind address (default: 127.0.0.1:8553)
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
  -config string
        JSON config file
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
  -doh-cert string
        DoH TLS certificate file (auto-generates self-signed if omitted)
  -doh-key string
        DoH TLS private key file
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
  -syslog
        Log to syslog (default: false)
  -upstream value
        Upstream resolver (default: tls://1.1.1.1:853 tls://1.0.0.1:853)
```
