# Architecture

## Overview

Dinosaur is a DNS caching proxy. It accepts DNS queries over UDP or TCP,
resolves them via configurable upstream resolvers, caches responses, and
applies blocklist and ACL rules before returning answers to clients.

```
client (UDP/TCP)
      |
      v
  server/            -- binds listeners, starts goroutines
      |
      v
  proxy/             -- handler: ACL check, blocklist check, cache lookup,
      |                 upstream resolution, DNS64 synthesis
      |
      +-- cache/     -- in-memory TTL cache (permanent entries for local RRs)
      |
      +-- blocklist/ -- domain/qtype blocklist (trie)
      |
      +-- resolver/  -- upstream resolver types (UDP, DoT, DoH)
      |
      v
  api/               -- optional HTTP API + embedded web dashboard
```

## Packages

**cmd/dinosaur** -- entry point. Parses flags and JSON config via
`GetUserConfig`, builds a `ProxyConfig`, calls `server.StartServer`.

**config** -- `UserConfig` (JSON-serialisable) and `ProxyConfig` (runtime
state). `GetProxyConfig` translates user config into live objects: resolver
instances, parsed CIDRs, populated cache, etc.

**server** -- binds UDP and TCP listeners using `github.com/miekg/dns`,
starts the cache-flush goroutine, blocklist-refresh goroutine, and optional
API goroutine, then blocks on a context for graceful shutdown.

**proxy** -- `MakeHandler` returns the `dns.HandlerFunc` registered with the
miekg mux. For each query: check ACL, check blocklist, consult cache, call
`resolve` (which fans out to upstream resolvers with automatic demotion on
failure), optionally synthesise DNS64 AAAA records, write response.
`CheckUpstream` validates a single upstream at startup.

**resolver** -- three resolver types, all implementing the `Resolver`
interface (`Resolve(log, msg) (msg, error)`):

- `UdpResolver` -- plain DNS over UDP. `dns.Client` stored on the struct
  with a 5 s timeout.
- `DotResolver` -- DNS over TLS. Channel-based idle-connection pool (size 5)
  with health-check on reuse, TLS session resumption, and retry on transient
  EOF.
- `DohResolver` -- DNS over HTTPS. Single `*http.Client` with a custom
  transport: HTTP/2, TLS session cache, keep-alive, 5 s timeout.

**cache** -- `DNSCache` wraps `map[DNSCacheKey]DNSCacheItem` behind an
`RWMutex`. `Add` stores upstream responses with TTL expiry. `AddRR` stores
permanent entries (local RRs). `Get` decrements TTLs on read, skipping OPT
records. `Flush` removes expired entries.

**blocklist** -- trie-based structure keyed by reversed domain labels and
qtype. Supports ANY-type entries (match all qtypes) and specific-type entries
(e.g. block AAAA only). Can be populated from a hosts file or a plain domain
list.

**api** -- optional HTTP server (default `127.0.0.1:8553`) with:
- `GET /ping` -- health check
- `POST /api` -- JSON-RPC 2.0 endpoint (gorilla/rpc): Config, CacheAdd,
  CacheDelete, CacheDebug, BlockListCount, BlockListAdd, BlockListDelete
- `GET /log` -- SSE stream of recent query log entries
- `GET /static/*` -- embedded web dashboard (plain JS, no external dependencies)

Can bind to a TCP address or a UNIX domain socket. When using a socket,
a signal handler removes the socket file before re-raising the signal so
context-based shutdown proceeds normally.

**util** -- shared helpers: `ParseAddr` (resolves interface names to IP
addresses), `JsonRpcRequest` (generic JSON-RPC client), `MultiFlag` (flag
that can be specified multiple times), test helpers.

**logger** -- thin wrapper around `log.Logger` with Debug/Info/Error/Fatal
levels and Stderr, Syslog, and Discard backends.

**statshandler** -- fixed-size ring buffer of `ConnectionLog` entries with
an SSE hook for the `/log` endpoint.

## Data flow

1. `dns.Server` (miekg) calls `MakeHandler` for each incoming query.
2. ACL check -- drop if client IP not in any permitted CIDR (default: allow
   all).
3. Blocklist check -- return NXDOMAIN if domain/qtype matched.
4. Cache lookup -- return cached response with decremented TTLs if hit.
5. Upstream resolution -- iterate resolver list in order; on success reset
   error counter and cache response; on first-resolver failure increment
   counter and demote after 3 consecutive errors.
6. DNS64 (if enabled) -- if AAAA query returned no answers and client is
   IPv6-only, re-resolve as A and synthesise AAAA records using the
   configured prefix (default `64:ff9b::/96`).
7. Write response.

## Configuration precedence

JSON config file < command-line flags. Both are optional. Flags that accept
multiple values (`-listen`, `-upstream`, `-block`, etc.) accumulate; the
JSON file values are read first, flags append to them.
