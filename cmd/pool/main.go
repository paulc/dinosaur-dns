package main

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/util"
)

type ConnPoolItem struct {
	Client dns.Client
	Conn   *dns.Conn
	Error  error
}

func main() {

	var connPool = &sync.Pool{
		New: func() any {
			poolItem := &ConnPoolItem{
				Client: dns.Client{
					Net: "tcp-tls",
				},
			}
			poolItem.Conn, poolItem.Error = poolItem.Client.Dial("1.1.1.1:853")
			return poolItem
		},
	}

	q := util.CreateQuery("www.google.com", "A")

	for i := 0; i < 5; i++ {
		for count, done := 0, false; !done && count < 3; {
			c := connPool.Get().(*ConnPoolItem)
			if c.Error != nil {
				continue
			}
			if i == 2 { // && count == 0 {
				c.Conn.Close()
			}
			_, rtt, err := c.Client.ExchangeWithConn(q, c.Conn)
			switch {
			case err == nil:
				fmt.Println(i, "OK", rtt)
				connPool.Put(c)
				done = true
			case errors.Is(err, net.ErrClosed):
				fmt.Println(i, "Connection Error")
			default:
				fmt.Println(err)
				done = true
			}
			count++
		}
	}
}
