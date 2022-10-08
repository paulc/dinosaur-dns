
<img src="./data/dinosaur.svg" width="360" />

# Dinosaur

## About

Dinosaur is a simple DNS caching proxy server intended for local networks. It provides a local caching resolver with support for configurable upstreams (supporting DoH), local block-lists, and local authoratitve entries.

It is intended to provided the basic functionality for local network proxy use with minimum configuration (in the most cases with just a few command-line flags).

It's most notable feature is qtype aware blocklists (you can block a specific lookup-type) - this is primarily useful if you are trying to block IPv6 address resolution for specific domains (the specific use-case is that Netflix wont work if you have a Hurricane Electric IPv6 tunnel as it will treat this as a proxy rather than using the direct IPv4 connection - for a more detailed description of the issue see https://gist.github.com/xorguy/d52bd9ab6558ffafee606d4f87e565ce).

The server was origibally written as a simple upstream for Unbound which wouuld just block all AAAA requests for specified domains (which it can still do `dinosaur --block :AAAA`) however it was pretty simple to add the basic additional functionality needed to act as a caching DNS proxy for a local network and avoid running two servers (for anything more complex you should use Unbound/Dnsmasq).

## Features

* UDP/DoH upstreams
* In-memory caching
* Local authorative entries (implemented as permament cache entries)
* Qtype aware blocklist (can block specific query-types - in particular AAAA for specific domains) - default is ANY
* Parse blocklist from hosts file (eg. https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts for example)
* Load blocklist/local zones from URL 
* ACL

## Usage

```
Usage of ./cmd/dinosaur/dinosaur:

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
    	Blocklist file
  -blocklist-aaaa value
    	Blocklist file (AAAA)
  -blocklist-from-hosts value
    	Blocklist from /etc/hosts format file
  -config string
    	JSON config file
  -debug
    	Discard logs (default: false)
  -discard
    	Debug logging (default: false)
  -dns64
    	Enable DNS64 (for queries from IPv6 addresses)
  -dns64-prefix string
    	DNS64 prefix (default: 64:ff9b::/96)
  -help
    	Show usage
  -listen value
    	Listen address/interface (default: lo0:8053)
  -local value
    	Local DNS resource record
  -localzone value
    	Local DNS resource record file
  -refresh
    	Auto refresh blocklist (default: false)
  -refresh-interval string
    	Blocklist refresh interval (default: 24hrs)
  -syslog
    	Use syslog (default: false)
  -upstream value
    	Upstream resolver [host:port or https://...] (default: 1.1.1.1:53,1.0.0.1:53)

```

(Netflix domains to filter: https://openconnect.netflix.com/mobiledeliverydomains.txt)
