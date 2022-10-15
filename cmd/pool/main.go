package main

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/util"
)

type DnsConnPool struct {
	Client dns.Client
	Conn   *dns.Conn
	Error  error
}

const retryLimit int = 3

func dotQuery(pool *sync.Pool, q *dns.Msg) (r *dns.Msg, rtt time.Duration, err error) {
	for retries := 0; retries < retryLimit; {
		c := pool.Get().(*DnsConnPool)
		if c.Error != nil {
			// Try again
			continue
		}
		r, rtt, err = c.Client.ExchangeWithConn(q, c.Conn)
		if err == nil {
			pool.Put(c)
			return
		} else if errors.Is(err, net.ErrClosed) {
			// retry
			fmt.Println(err)
			retries++
			continue
		} else {
			// return error
			return
		}
	}
	// retryLimit reached - return err
	return
}

func main() {

	var dnsPool = &sync.Pool{
		New: func() any {
			p := &DnsConnPool{
				Client: dns.Client{
					Net: "tcp-tls",
				},
			}
			p.Conn, p.Error = p.Client.Dial("1.1.1.1:853")
			return p
		},
	}

	q := util.CreateQuery("www.google.com", "A")

	for i := 0; i < 5; i++ {
		if i == 3 || i == 0 {
			c := dnsPool.Get().(*DnsConnPool)
			c.Conn.Close()
			dnsPool.Put(c)
		}
		_, rtt, err := dotQuery(dnsPool, q)
		if err != nil {
			fmt.Println(i, err)
		} else {
			fmt.Println(i, rtt)
		}
	}
}
