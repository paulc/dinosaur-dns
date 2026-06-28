package resolver

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/logger"
)

// Resolver is the interface implemented by all upstream resolver types.
type Resolver interface {
	Resolve(log *logger.Logger, r *dns.Msg) (*dns.Msg, error)
	String() string
}

// ── UDP Resolver ──────────────────────────────────────────────────────────────

type UdpResolver struct {
	Upstream string
}

func (r *UdpResolver) Resolve(log *logger.Logger, q *dns.Msg) (*dns.Msg, error) {
	c := &dns.Client{}
	out, _, err := c.Exchange(q, r.Upstream)
	if err != nil {
		return nil, fmt.Errorf("DNS Query Error: %s", err)
	}
	return out, nil
}

func (r *UdpResolver) String() string { return r.Upstream }

func NewUdpResolver(upstream string) *UdpResolver {
	return &UdpResolver{Upstream: upstream}
}

// ── DoT Resolver ──────────────────────────────────────────────────────────────

const (
	// dotPoolSize is the maximum number of idle TLS connections kept in the pool.
	dotPoolSize = 5
	// dotTimeout is applied to both the TLS handshake (dial) and each DNS exchange.
	dotTimeout = 5 * time.Second
)

type dotConn struct {
	conn *dns.Conn
}

// DotResolver maintains a bounded pool of reusable TLS connections to a single
// DoT upstream. Idle connections are health-checked before reuse; dead ones are
// closed and replaced. A shared tls.Config with a session cache enables TLS
// session resumption across all connections.
type DotResolver struct {
	upstream string
	address  string
	client   dns.Client    // shared across all connections; holds TLS config and timeouts
	pool     chan *dotConn // buffered channel acts as the idle-connection pool
}

// newConn dials a fresh TLS connection to the upstream.
func (r *DotResolver) newConn() (*dotConn, error) {
	conn, err := r.client.Dial(r.address)
	if err != nil {
		return nil, fmt.Errorf("DoT dial: %w", err)
	}
	return &dotConn{conn: conn}, nil
}

// isAlive reports whether an idle pooled connection is still open.
//
// It sets a 1 ms read deadline and attempts a single-byte read:
//   - timeout error  → connection is alive and idle (nothing to read)
//   - io.EOF / other → server has closed the connection
//
// The 1 ms deadline is short enough that no real DNS response will arrive
// during the check, so the read is non-destructive on a healthy idle connection.
func isAlive(c *dotConn) bool {
	var b [1]byte
	c.conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	defer c.conn.SetReadDeadline(time.Time{})
	_, err := c.conn.Read(b[:])
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true // idle but open
		}
		return false // EOF, reset, or other close signal
	}
	return true // unexpected data, but connection is clearly alive
}

// getConn returns a healthy connection from the pool, or dials a new one.
// Dead connections detected by isAlive are closed and skipped.
func (r *DotResolver) getConn() (*dotConn, error) {
	for {
		select {
		case c := <-r.pool:
			if isAlive(c) {
				return c, nil
			}
			c.conn.Close() // dead — discard and try next
		default:
			return r.newConn() // pool empty — dial fresh
		}
	}
}

// putConn returns a connection to the idle pool.
// If the pool is already full the connection is closed immediately.
func (r *DotResolver) putConn(c *dotConn) {
	select {
	case r.pool <- c:
	default:
		c.conn.Close()
	}
}

// isTransientConnErr reports whether err is a connection-level error that
// warrants a retry with a fresh connection. This covers the common case of
// a server-side idle timeout: the remote sends TCP FIN, which Go surfaces as
// io.EOF on the next read — not net.ErrClosed.
func isTransientConnErr(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed)
}

func (r *DotResolver) Resolve(log *logger.Logger, q *dns.Msg) (out *dns.Msg, err error) {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var c *dotConn
		c, err = r.getConn()
		if err != nil {
			return // dial failed — no point retrying immediately
		}
		out, _, err = r.client.ExchangeWithConn(q, c.conn)
		if err == nil {
			r.putConn(c)
			return
		}
		c.conn.Close() // always close on any exchange error
		if !isTransientConnErr(err) {
			return // non-transient (e.g. malformed response) — propagate immediately
		}
		log.Debugf("DoT transient error (attempt %d/%d): %s", attempt+1, maxAttempts, err)
	}
	return
}

func (r *DotResolver) String() string { return r.upstream }

func NewDotResolver(upstream string) *DotResolver {
	address := strings.TrimPrefix(upstream, "tls://")
	return &DotResolver{
		upstream: upstream,
		address:  address,
		client: dns.Client{
			Net: "tcp-tls",
			// Shared TLS config with a session cache so that reconnections
			// after idle-timeout drops can resume the TLS session (~0 RTT
			// overhead) rather than doing a full handshake (~1 RTT extra).
			TLSConfig: &tls.Config{
				ClientSessionCache: tls.NewLRUClientSessionCache(64),
			},
			// Timeout covers both ExchangeWithConn and (via Dialer) the
			// initial TLS handshake, bounding worst-case latency.
			Timeout: dotTimeout,
			Dialer: &net.Dialer{
				Timeout:   dotTimeout,
				KeepAlive: 30 * time.Second,
			},
		},
		pool: make(chan *dotConn, dotPoolSize),
	}
}

// ── DoH Resolver ─────────────────────────────────────────────────────────────

type DohResolver struct {
	Upstream string
}

func (r *DohResolver) Resolve(log *logger.Logger, q *dns.Msg) (*dns.Msg, error) {

	c := &http.Client{}

	pack, err := q.Pack()
	if err != nil {
		return nil, fmt.Errorf("Error packing record: %s", err)
	}

	request, err := http.NewRequest("POST", r.Upstream, bytes.NewReader(pack))
	if err != nil {
		return nil, fmt.Errorf("Error creating HTTP request: %s", err)
	}

	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("content-type", "application/dns-message")

	resp, err := c.Do(request)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP request failed - status: %s", resp.Status)
	}

	buffer := bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading HTTP body: %s", err)
	}

	out := new(dns.Msg)
	if err = out.Unpack(buffer.Bytes()); err != nil {
		return nil, fmt.Errorf("Error parsing DNS response: %s", err)
	}

	return out, nil

}

func (r *DohResolver) String() string { return r.Upstream }

func NewDohResolver(upstream string) *DohResolver {
	return &DohResolver{Upstream: upstream}
}
