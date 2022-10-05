package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// For testing
var timeNow = time.Now

type DNSCacheKey struct {
	Name  string
	Qtype uint16
}

func (k DNSCacheKey) String() string {
	return fmt.Sprintf("<%s %s>", k.Name, dns.TypeToString[k.Qtype])
}

type DNSCacheItem struct {
	Message   *dns.Msg
	Inserted  time.Time
	Expires   time.Time
	Permanent bool
}

func (i DNSCacheItem) String() string {
	if i.Permanent {
		return fmt.Sprintf("<%s %s> permanent",
			i.Message.Question[0].Name,
			dns.TypeToString[i.Message.Question[0].Qtype])
	} else {
		return fmt.Sprintf("<%s %s> %.1fs",
			i.Message.Question[0].Name,
			dns.TypeToString[i.Message.Question[0].Qtype],
			i.Expires.Sub(timeNow()).Seconds())
	}
}

type DNSCache struct {
	sync.RWMutex
	Cache map[DNSCacheKey]DNSCacheItem
}

func New() *DNSCache {
	return &DNSCache{Cache: make(map[DNSCacheKey]DNSCacheItem)}
}

func (c *DNSCache) AddRR(entry string, permanent bool) error {

	rr, err := dns.NewRR(entry)
	if err != nil {
		return fmt.Errorf("Error creating RR: %s", err)
	}

	if rr == nil {
		// No RR
		return nil
	}

	// Construct template reply
	name := dns.CanonicalName(rr.Header().Name)
	msg := new(dns.Msg)
	msg.SetQuestion(name, rr.Header().Rrtype)
	msg.Response = true
	msg.Authoritative = permanent
	msg.RecursionAvailable = false
	msg.Rcode = dns.RcodeSuccess
	msg.Answer = append(msg.Answer, rr)

	now := timeNow()
	expires := now.Add(time.Second * time.Duration(rr.Header().Ttl))

	key := DNSCacheKey{Name: name, Qtype: rr.Header().Rrtype}
	val := DNSCacheItem{Message: msg, Inserted: timeNow(), Expires: expires, Permanent: permanent}

	c.Lock()
	defer c.Unlock()

	c.Cache[key] = val

	return nil
}

func (c *DNSCache) Add(msg *dns.Msg) {

	if (msg.Rcode != dns.RcodeSuccess) || (msg.Truncated == true) || (len(msg.Answer)+len(msg.Ns)+len(msg.Extra) == 0) {
		// Error or No RRs
		return
	}

	// Get minium TTL from RRs
	minTTL := uint32(86400) // Max cache age
	for _, section := range [][]dns.RR{msg.Answer, msg.Ns, msg.Extra} {
		for _, rr := range section {
			rr_hdr := rr.Header()
			// Ignore OPT records
			if rr_hdr.Rrtype != dns.TypeOPT && rr_hdr.Ttl < minTTL {
				minTTL = rr_hdr.Ttl
			}
		}
	}

	if minTTL == 0 {
		return
	}

	// Calculate cache expiry time
	now := timeNow()
	expires := now.Add(time.Second * time.Duration(minTTL))

	key := DNSCacheKey{Name: dns.CanonicalName(msg.Question[0].Name), Qtype: msg.Question[0].Qtype}
	val := DNSCacheItem{Message: msg.Copy(), Inserted: now, Expires: expires, Permanent: false}

	c.Lock()
	defer c.Unlock()

	c.Cache[key] = val
}

func (c *DNSCache) Get(query *dns.Msg) (*dns.Msg, bool) {

	c.Lock()
	defer c.Unlock()

	key := DNSCacheKey{Name: dns.CanonicalName(query.Question[0].Name), Qtype: query.Question[0].Qtype}

	entry, found := c.Cache[key]
	if !found {
		return nil, false
	}

	if !entry.Permanent && timeNow().After(entry.Expires) {
		// Expired - flush key
		delete(c.Cache, key)
		return nil, false
	}

	reply := entry.Message.Copy()

	// Fix ID
	reply.Id = query.Id

	if !entry.Permanent {
		// Decrement TTL for cached records
		delta := uint32(timeNow().Sub(entry.Inserted).Seconds())
		for _, section := range [][]dns.RR{reply.Answer, reply.Ns, reply.Extra} {
			for _, v := range section {
				v.Header().Ttl -= delta
			}
		}
	}

	return reply, true
}

// Convenience wrapper for c.Add - for testing
func (c *DNSCache) GetName(qname string, qtype string) (*dns.Msg, bool) {
	msg := new(dns.Msg)
	msg.SetQuestion(qname, dns.StringToType[qtype])
	return c.Get(msg)
}

func (c *DNSCache) Delete(query *dns.Msg) {

	c.Lock()
	defer c.Unlock()

	key := DNSCacheKey{Name: dns.CanonicalName(query.Question[0].Name), Qtype: query.Question[0].Qtype}
	delete(c.Cache, key)
}

func (c *DNSCache) DeleteName(name string, qtype string) {

	c.Lock()
	defer c.Unlock()

	// We ignore invalid qtype as delete will fail
	key := DNSCacheKey{Name: dns.CanonicalName(name), Qtype: dns.StringToType[qtype]}
	delete(c.Cache, key)
}

func (c *DNSCache) Flush() (total, expired int) {

	c.Lock()
	defer c.Unlock()

	now := timeNow()
	for k, v := range c.Cache {
		total++
		if !v.Permanent && now.After(v.Expires) {
			delete(c.Cache, k)
			expired++
		}
	}
	return
}

func (c *DNSCache) Debug() (result []string) {

	c.Lock()
	defer c.Unlock()

	for _, v := range c.Cache {
		result = append(result, fmt.Sprintf("%s", v))
	}

	return
}
