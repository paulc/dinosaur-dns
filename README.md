
<img src="./data/dinosaur.svg" width="360" />

# Dinosaur

## About

Dinosaur is a simple DNS caching proxy server intended for local networks. It provides a local caching resolver with support for configurable upstreams (supporting DoH), local block-lists, and local authoratitve entries.

It is intended to provided the basic functionality for local network proxy use with minimum configuration (in the most cases with just a few command-line flags).

It's most notable feature is qtype aware blocklists (you can block a specific lookup-type) - this is primarily useful if you are trying to block IPv6 address resolution for specific domains (the specific use-case is that Netflix wont work if you have a Hurricane Electric IPv6 tunnel as it will treat this as a proxy rather than using the direct IpV4 connection - for a more detailed description of the issue see https://gist.github.com/xorguy/d52bd9ab6558ffafee606d4f87e565ce).

The server was origibally written as a simple upstream for Unbound which wouuld just block all AAAA requests for specified domains (which it can still do `dinosaur --block :AAAA`) however it was pretty simple to add the basic additional functionality needed to act as a caching DNS proxy for a local network and avoid running two servers (for anything more complex you should use Unbound/Dnsmasq).

## Features

## Usage

```
Usage of dinosaur:
  -block value
    	Block entry (format: 'domain[:qtype]')
  -blocklist value
    	Blocklist file
  -debug
    	Debug
  -help
    	Show usage
  -listen value
    	Listen address (default: 127.0.0.1:8053)
  -localrr value
    	Local DNS resource record
  -localrrfile value
    	Local DNS resource record file
  -upstream value
    	Upstream resolver [host:port or https://...] (default: 1.1.1.1:53)
```
